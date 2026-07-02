package httpapi

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/adp/panel/internal/auth"
	"github.com/adp/panel/internal/backup"
	"github.com/adp/panel/internal/store"
)

// backupHandler exposes encrypted export/import of the whole panel database.
type backupHandler struct {
	svc     *backup.Service
	store   *store.Store
	logger  *slog.Logger
	restart func() // triggers a process restart to apply a staged restore
}

// minPassphrase is the shortest passphrase we accept — it is the only thing
// protecting a backup that may travel through Telegram or e-mail.
const minPassphrase = 8

type exportRequest struct {
	Passphrase string `json:"passphrase"`
}

// export streams an encrypted .adpbak archive of the current database.
func (h *backupHandler) export(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len([]rune(req.Passphrase)) < minPassphrase {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("passphrase must be at least %d characters", minPassphrase))
		return
	}

	data, filename, err := h.svc.Export(r.Context(), req.Passphrase)
	if err != nil {
		h.logger.Error("backup export failed", "err", err)
		writeError(w, http.StatusInternalServerError, "backup failed")
		return
	}

	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, "backup.export", "backup", 0, "")

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// importBackup restores an uploaded archive. On success it stages the restored
// database and schedules a restart; the panel comes back up with the imported
// data. The request is multipart: field "file" (the archive) + "passphrase".
func (h *backupHandler) importBackup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "expected a multipart upload")
		return
	}
	passphrase := r.FormValue("passphrase")
	if passphrase == "" {
		writeError(w, http.StatusBadRequest, "passphrase is required")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no backup file provided")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, 128<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read the uploaded file")
		return
	}

	report, err := h.svc.Import(r.Context(), data, passphrase)
	if err != nil {
		switch {
		case errors.Is(err, backup.ErrWrongPassphrase):
			writeError(w, http.StatusBadRequest, "wrong passphrase or corrupted file")
		case errors.Is(err, backup.ErrBadArchive):
			writeError(w, http.StatusBadRequest, "this is not a valid ADP backup file")
		case errors.Is(err, backup.ErrNoKey):
			writeError(w, http.StatusBadRequest, "backup is missing its master key; cannot restore")
		case errors.Is(err, backup.ErrNoPassphrase):
			writeError(w, http.StatusBadRequest, "passphrase is required")
		default:
			h.logger.Error("backup import failed", "err", err)
			writeError(w, http.StatusInternalServerError, "restore failed")
		}
		return
	}

	adminID, _ := auth.AdminIDFrom(r.Context())
	recordAudit(r.Context(), h.store, r, adminID, "backup.import", "backup", 0,
		fmt.Sprintf(`{"servers":%d,"inbounds":%d,"clients":%d}`, report.Servers, report.Inbounds, report.Clients))
	h.logger.Info("backup restored; restarting to apply",
		"servers", report.Servers, "inbounds", report.Inbounds, "clients", report.Clients)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"restarting": true,
		"report":     report,
	})

	// Apply the staged restore by restarting once the response has been sent.
	if h.restart != nil {
		go h.restart()
	}
}
