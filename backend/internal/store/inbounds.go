package store

import (
	"context"
	"database/sql"
	"errors"
)

// Inbound is a proxy endpoint on a server.
type Inbound struct {
	ID                 int64
	ServerID           int64
	Tag                string
	Protocol           string
	Listen             string
	Port               int
	SettingsJSON       string
	StreamSettingsJSON string
	SniffingJSON       string
	Remark             string
	Enabled            bool
	ClientCount        int // computed
	CreatedAt          string
	UpdatedAt          string
}

// InboundParams holds writable inbound fields.
type InboundParams struct {
	Tag                string
	Protocol           string
	Listen             string
	Port               int
	SettingsJSON       string
	StreamSettingsJSON string
	SniffingJSON       string
	Remark             string
	Enabled            bool
}

const inboundSelect = `
	SELECT i.id, i.server_id, i.tag, i.protocol, i.listen, i.port,
	       i.settings_json, i.stream_settings_json, i.sniffing_json, i.remark, i.enabled,
	       i.created_at, i.updated_at,
	       (SELECT COUNT(*) FROM client_inbounds WHERE inbound_id = i.id) AS client_count
	FROM inbounds i`

func scanInbound(sc interface{ Scan(...any) error }) (*Inbound, error) {
	var in Inbound
	var enabled int64
	err := sc.Scan(&in.ID, &in.ServerID, &in.Tag, &in.Protocol, &in.Listen, &in.Port,
		&in.SettingsJSON, &in.StreamSettingsJSON, &in.SniffingJSON, &in.Remark, &enabled,
		&in.CreatedAt, &in.UpdatedAt, &in.ClientCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	in.Enabled = enabled != 0
	return &in, nil
}

func jsonOrEmpty(s string) string {
	if s == "" {
		return "{}"
	}
	return s
}

// ListInboundsByServer returns a server's inbounds.
func (s *Store) ListInboundsByServer(ctx context.Context, serverID int64) ([]Inbound, error) {
	rows, err := s.db.QueryContext(ctx, inboundSelect+" WHERE i.server_id = ? ORDER BY i.id ASC", serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Inbound
	for rows.Next() {
		in, err := scanInbound(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *in)
	}
	return out, rows.Err()
}

// ListEnabledInboundsByServer returns only enabled inbounds (used by sync).
func (s *Store) ListEnabledInboundsByServer(ctx context.Context, serverID int64) ([]Inbound, error) {
	all, err := s.ListInboundsByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]Inbound, 0, len(all))
	for _, in := range all {
		if in.Enabled {
			out = append(out, in)
		}
	}
	return out, nil
}

// ListAllEnabledInboundIDs returns the ids of every enabled inbound across all
// servers. Used to grant a self-signed-up client everything currently on offer.
func (s *Store) ListAllEnabledInboundIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM inbounds WHERE enabled = 1 ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// GetInbound returns an inbound by id.
func (s *Store) GetInbound(ctx context.Context, id int64) (*Inbound, error) {
	return scanInbound(s.db.QueryRowContext(ctx, inboundSelect+" WHERE i.id = ?", id))
}

// CreateInbound inserts an inbound on a server.
func (s *Store) CreateInbound(ctx context.Context, serverID int64, p InboundParams) (*Inbound, error) {
	now := nowRFC3339()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO inbounds (server_id, tag, protocol, listen, port, settings_json,
			stream_settings_json, sniffing_json, remark, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		serverID, p.Tag, p.Protocol, orDefaultStr(p.Listen, "0.0.0.0"), p.Port,
		jsonOrEmpty(p.SettingsJSON), jsonOrEmpty(p.StreamSettingsJSON), jsonOrEmpty(p.SniffingJSON),
		p.Remark, boolToInt(p.Enabled), now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetInbound(ctx, id)
}

// UpdateInbound updates an inbound.
func (s *Store) UpdateInbound(ctx context.Context, id int64, p InboundParams) (*Inbound, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE inbounds SET tag=?, protocol=?, listen=?, port=?, settings_json=?,
			stream_settings_json=?, sniffing_json=?, remark=?, enabled=?, updated_at=?
		WHERE id=?`,
		p.Tag, p.Protocol, orDefaultStr(p.Listen, "0.0.0.0"), p.Port,
		jsonOrEmpty(p.SettingsJSON), jsonOrEmpty(p.StreamSettingsJSON), jsonOrEmpty(p.SniffingJSON),
		p.Remark, boolToInt(p.Enabled), nowRFC3339(), id)
	if err != nil {
		return nil, err
	}
	return s.GetInbound(ctx, id)
}

// DeleteInbound removes an inbound.
func (s *Store) DeleteInbound(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM inbounds WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func orDefaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
