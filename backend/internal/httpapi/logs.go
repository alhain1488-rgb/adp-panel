package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/adp/panel/internal/store"
)

type auditLogDTO struct {
	ID            int64           `json:"id"`
	AdminID       int64           `json:"admin_id,omitempty"`
	AdminUsername string          `json:"admin_username,omitempty"`
	Action        string          `json:"action"`
	TargetType    string          `json:"target_type,omitempty"`
	TargetID      int64           `json:"target_id,omitempty"`
	Detail        json.RawMessage `json:"detail,omitempty"`
	IP            string          `json:"ip,omitempty"`
	UserAgent     string          `json:"user_agent,omitempty"`
	CreatedAt     string          `json:"created_at"`
}

type auditLogPage struct {
	Items    []auditLogDTO `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

func (h *authHandler) listLogs(w http.ResponseWriter, r *http.Request) {
	page := atoiDefault(r.URL.Query().Get("page"), 1, 1, 1<<30)
	pageSize := atoiDefault(r.URL.Query().Get("page_size"), 50, 1, 200)

	total, err := h.store.CountAuditLogs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load logs")
		return
	}
	logs, err := h.store.ListAuditLogs(r.Context(), pageSize, (page-1)*pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load logs")
		return
	}

	items := make([]auditLogDTO, 0, len(logs))
	for _, l := range logs {
		items = append(items, toAuditDTO(l))
	}
	writeJSON(w, http.StatusOK, auditLogPage{Items: items, Total: total, Page: page, PageSize: pageSize})
}

func toAuditDTO(l store.AuditLog) auditLogDTO {
	detail := json.RawMessage(l.DetailJSON)
	if !json.Valid(detail) {
		detail = json.RawMessage(`{}`)
	}
	return auditLogDTO{
		ID:            l.ID,
		AdminID:       l.AdminID,
		AdminUsername: l.AdminUsername,
		Action:        l.Action,
		TargetType:    l.TargetType,
		TargetID:      l.TargetID,
		Detail:        detail,
		IP:            l.IP,
		UserAgent:     l.UserAgent,
		CreatedAt:     l.CreatedAt,
	}
}

func atoiDefault(s string, def, min, max int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
