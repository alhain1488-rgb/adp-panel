package store

import (
	"context"
	"database/sql"
)

// AuditLog records an administrative action.
type AuditLog struct {
	ID            int64
	AdminID       int64
	AdminUsername string
	Action        string
	TargetType    string
	TargetID      int64
	DetailJSON    string
	IP            string
	UserAgent     string
	CreatedAt     string
}

// InsertAuditLog appends an audit entry.
func (s *Store) InsertAuditLog(ctx context.Context, e AuditLog) error {
	detail := e.DetailJSON
	if detail == "" {
		detail = "{}"
	}
	var adminID any
	if e.AdminID != 0 {
		adminID = e.AdminID
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_logs (admin_id, action, target_type, target_id, detail_json, ip, user_agent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		adminID, e.Action, e.TargetType, e.TargetID, detail, e.IP, e.UserAgent, nowRFC3339())
	return err
}

// CountAuditLogs returns the total number of audit entries.
func (s *Store) CountAuditLogs(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs").Scan(&n)
	return n, err
}

// ListAuditLogs returns a page of audit entries, newest first, joined with the
// admin username.
func (s *Store) ListAuditLogs(ctx context.Context, limit, offset int) ([]AuditLog, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, COALESCE(a.admin_id, 0), COALESCE(ad.username, ''), a.action,
		       a.target_type, a.target_id, a.detail_json, a.ip, a.user_agent, a.created_at
		FROM audit_logs a
		LEFT JOIN admins ad ON ad.id = a.admin_id
		ORDER BY a.id DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []AuditLog
	for rows.Next() {
		var e AuditLog
		var adminID sql.NullInt64
		if err := rows.Scan(&e.ID, &adminID, &e.AdminUsername, &e.Action, &e.TargetType,
			&e.TargetID, &e.DetailJSON, &e.IP, &e.UserAgent, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.AdminID = adminID.Int64
		out = append(out, e)
	}
	return out, rows.Err()
}
