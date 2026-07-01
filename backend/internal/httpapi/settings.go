package httpapi

import (
	"net/http"
	"strconv"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/store"
)

// settingsHandler serves panel-wide settings backed by the settings KV table,
// falling back to config-derived defaults.
type settingsHandler struct {
	store        *store.Store
	domain       string
	subBaseURL   string
	defaultTheme string
}

// settingsDTO mirrors the Settings schema in docs/openapi.yaml.
type settingsDTO struct {
	Domain              string `json:"domain"`
	SubscriptionBaseURL string `json:"subscription_base_url"`
	SyncIntervalSeconds int    `json:"sync_interval_seconds"`
	HysteriaEngine      string `json:"hysteria_engine"`
	Theme               string `json:"theme"`
}

// settings keys in the KV table.
const (
	keyDomain      = "domain"
	keySubBaseURL  = "subscription_base_url"
	keySyncSeconds = "sync_interval_seconds"
	keyHyEngine    = "hysteria_engine"
	keyTheme       = "theme"
)

func (h *settingsHandler) current(r *http.Request) settingsDTO {
	ctx := r.Context()
	get := func(key, def string) string {
		if v, err := h.store.GetSetting(ctx, key); err == nil && v != "" {
			return v
		}
		return def
	}
	syncSecs, _ := strconv.Atoi(get(keySyncSeconds, "0"))
	return settingsDTO{
		Domain:              get(keyDomain, h.domain),
		SubscriptionBaseURL: get(keySubBaseURL, h.subBaseURL),
		SyncIntervalSeconds: syncSecs,
		HysteriaEngine:      get(keyHyEngine, "sing-box"),
		Theme:               get(keyTheme, firstNonEmptyStr(h.defaultTheme, "system")),
	}
}

func (h *settingsHandler) get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.current(r))
}

func (h *settingsHandler) update(w http.ResponseWriter, r *http.Request) {
	var in settingsDTO
	if err := decode(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx := r.Context()
	// hysteria_engine is deployment-chosen and read-only from the UI; never written here.
	pairs := map[string]string{
		keyDomain:      in.Domain,
		keySubBaseURL:  in.SubscriptionBaseURL,
		keySyncSeconds: strconv.Itoa(in.SyncIntervalSeconds),
		keyTheme:       in.Theme,
	}
	for k, v := range pairs {
		if err := h.store.SetSetting(ctx, k, v); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save settings")
			return
		}
	}
	adminID, _ := auth.AdminIDFrom(ctx)
	recordAudit(ctx, h.store, r, adminID, "settings.update", "settings", 0, "{}")
	writeJSON(w, http.StatusOK, h.current(r))
}

func firstNonEmptyStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
