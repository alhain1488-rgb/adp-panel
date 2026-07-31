package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/billing"
	"github.com/adp/panel/internal/store"
)

// billingHandler serves the operator-facing billing endpoints: global settings
// (tariffs, Star rate, support contact) and per-client wallet actions.
type billingHandler struct {
	svc   *billing.Service
	store *store.Store
}

func (h *billingHandler) getSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.GetSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load billing settings")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *billingHandler) putSettings(w http.ResponseWriter, r *http.Request) {
	var in billing.Settings
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.SetSettings(r.Context(), in); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save billing settings")
		return
	}
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, "billing.settings", "billing", 0, "{}")
	cfg, err := h.svc.GetSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load billing settings")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// clientGet returns a client's wallet + subscription snapshot with recent ledger.
func (h *billingHandler) clientGet(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	cb, err := h.svc.ClientBilling(r.Context(), id, 50)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load client billing")
		return
	}
	writeJSON(w, http.StatusOK, cb)
}

// clientTopup applies an operator wallet adjustment in kopecks (positive credit,
// negative debit).
func (h *billingHandler) clientTopup(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in struct {
		Kopecks int64  `json:"kopecks"`
		Detail  string `json:"detail"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if in.Kopecks == 0 {
		writeError(w, http.StatusBadRequest, "amount must be non-zero")
		return
	}
	if _, err := h.svc.ManualAdjust(r.Context(), id, in.Kopecks, in.Detail); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to adjust balance")
		return
	}
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, "billing.adjust", "client", id,
		`{"kopecks":`+strconv.FormatInt(in.Kopecks, 10)+`}`)
	h.writeClientBilling(w, r, id)
}

// clientGrant extends a client's subscription without charging the wallet, either
// by a tariff's duration ({"tariff":"month"}) or an explicit day count.
func (h *billingHandler) clientGrant(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in struct {
		Tariff string `json:"tariff"`
		Days   int    `json:"days"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	days := in.Days
	if in.Tariff != "" {
		if tf, found := h.svc.TariffByKey(r.Context(), in.Tariff); found {
			days = tf.Days
		}
	}
	if days <= 0 {
		writeError(w, http.StatusBadRequest, "days must be positive (or a valid tariff)")
		return
	}
	if _, err := h.svc.Grant(r.Context(), id, days, "operator grant"); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to grant subscription")
		return
	}
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, "billing.grant", "client", id,
		`{"days":`+strconv.Itoa(days)+`}`)
	h.writeClientBilling(w, r, id)
}

// clientRefund returns a Stars top-up to the payer via Telegram and debits the
// credited amount from their wallet. Refunds are refused when the money has
// already been spent — the balance never goes negative.
func (h *billingHandler) clientRefund(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in struct {
		TxID int64 `json:"tx_id"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if in.TxID <= 0 {
		writeError(w, http.StatusBadRequest, "tx_id is required")
		return
	}
	cb, err := h.svc.RefundStars(r.Context(), id, in.TxID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	case errors.Is(err, store.ErrNotRefundable):
		writeError(w, http.StatusConflict, "only Telegram Stars top-ups can be refunded")
		return
	case errors.Is(err, store.ErrAlreadyRefunded):
		writeError(w, http.StatusConflict, "this payment has already been refunded")
		return
	case errors.Is(err, store.ErrInsufficientFunds):
		writeError(w, http.StatusConflict, "balance no longer covers this top-up — settle it manually")
		return
	case errors.Is(err, billing.ErrNotLinked):
		writeError(w, http.StatusConflict, "client has no linked Telegram account to refund to")
		return
	case errors.Is(err, billing.ErrNoRefunder):
		writeError(w, http.StatusServiceUnavailable, "Telegram bot is not configured")
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, "Telegram refused the refund")
		return
	}
	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, "billing.refund", "client", id,
		`{"tx_id":`+strconv.FormatInt(in.TxID, 10)+`}`)
	writeJSON(w, http.StatusOK, cb)
}

func (h *billingHandler) writeClientBilling(w http.ResponseWriter, r *http.Request, id int64) {
	cb, err := h.svc.ClientBilling(r.Context(), id, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load client billing")
		return
	}
	writeJSON(w, http.StatusOK, cb)
}
