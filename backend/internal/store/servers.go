package store

import (
	"context"
	"database/sql"
	"errors"
)

// Server is a managed node.
type Server struct {
	ID                  int64
	Name                string
	Host                string
	SSHPort             int
	SSHUser             string
	SSHAuthMethod       string
	SSHSecretEnc        string
	SSHPassphraseEnc    string
	XrayConfigPath      string
	XrayServiceName     string
	HysteriaConfigPath  string
	HysteriaServiceName string
	IP                  string
	GeoCountry          string
	GeoCity             string
	GeoASN              string
	Status              string
	ProvisionStatus     string
	ProvisionError      string
	EnginesJSON         string
	LastCheckAt         string
	LastSyncAt          string
	LastSyncError       string
	CreatedAt           string
	UpdatedAt           string
	// Computed, not a column.
	InboundCount int
}

// ServerParams holds the writable fields for create/update.
type ServerParams struct {
	Name                string
	Host                string
	SSHPort             int
	SSHUser             string
	SSHAuthMethod       string
	SSHSecretEnc        string
	SSHPassphraseEnc    string
	XrayConfigPath      string
	XrayServiceName     string
	HysteriaConfigPath  string
	HysteriaServiceName string
}

const serverSelect = `
	SELECT s.id, s.name, s.host, s.ssh_port, s.ssh_user, s.ssh_auth_method,
	       s.ssh_secret_enc, s.ssh_passphrase_enc, s.xray_config_path, s.xray_service_name,
	       s.hysteria_config_path, s.hysteria_service_name, s.ip, s.geo_country, s.geo_city,
	       s.geo_asn, s.status, s.provision_status, s.provision_error, s.engines_json,
	       COALESCE(s.last_check_at, ''), COALESCE(s.last_sync_at, ''), s.last_sync_error,
	       s.created_at, s.updated_at,
	       (SELECT COUNT(*) FROM inbounds WHERE server_id = s.id) AS inbound_count
	FROM servers s`

func scanServer(sc interface{ Scan(...any) error }) (*Server, error) {
	var s Server
	err := sc.Scan(&s.ID, &s.Name, &s.Host, &s.SSHPort, &s.SSHUser, &s.SSHAuthMethod,
		&s.SSHSecretEnc, &s.SSHPassphraseEnc, &s.XrayConfigPath, &s.XrayServiceName,
		&s.HysteriaConfigPath, &s.HysteriaServiceName, &s.IP, &s.GeoCountry, &s.GeoCity,
		&s.GeoASN, &s.Status, &s.ProvisionStatus, &s.ProvisionError, &s.EnginesJSON,
		&s.LastCheckAt, &s.LastSyncAt, &s.LastSyncError, &s.CreatedAt, &s.UpdatedAt, &s.InboundCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

// CreateServer inserts a server and returns it.
func (s *Store) CreateServer(ctx context.Context, p ServerParams) (*Server, error) {
	now := nowRFC3339()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO servers (name, host, ssh_port, ssh_user, ssh_auth_method, ssh_secret_enc,
			ssh_passphrase_enc, xray_config_path, xray_service_name, hysteria_config_path,
			hysteria_service_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Host, p.SSHPort, p.SSHUser, p.SSHAuthMethod, p.SSHSecretEnc, p.SSHPassphraseEnc,
		p.XrayConfigPath, p.XrayServiceName, p.HysteriaConfigPath, p.HysteriaServiceName, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetServer(ctx, id)
}

// GetServer returns a server by id.
func (s *Store) GetServer(ctx context.Context, id int64) (*Server, error) {
	return scanServer(s.db.QueryRowContext(ctx, serverSelect+" WHERE s.id = ?", id))
}

// ListServers returns all servers, newest first.
func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx, serverSelect+" ORDER BY s.id ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Server
	for rows.Next() {
		srv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *srv)
	}
	return out, rows.Err()
}

// UpdateServer updates the writable fields. Secret/passphrase are updated only
// when non-empty (the caller passes the existing encrypted value to keep it).
func (s *Store) UpdateServer(ctx context.Context, id int64, p ServerParams) (*Server, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET name=?, host=?, ssh_port=?, ssh_user=?, ssh_auth_method=?,
			ssh_secret_enc=?, ssh_passphrase_enc=?, xray_config_path=?, xray_service_name=?,
			hysteria_config_path=?, hysteria_service_name=?, updated_at=?
		WHERE id=?`,
		p.Name, p.Host, p.SSHPort, p.SSHUser, p.SSHAuthMethod, p.SSHSecretEnc, p.SSHPassphraseEnc,
		p.XrayConfigPath, p.XrayServiceName, p.HysteriaConfigPath, p.HysteriaServiceName,
		nowRFC3339(), id)
	if err != nil {
		return nil, err
	}
	return s.GetServer(ctx, id)
}

// DeleteServer removes a server (cascades to inbounds/grants).
func (s *Store) DeleteServer(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM servers WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CheckUpdate holds the fields refreshed by a reachability check.
type CheckUpdate struct {
	Status      string
	IP          string
	GeoCountry  string
	GeoCity     string
	GeoASN      string
	EnginesJSON string
}

// ApplyCheck stores reachability/geo/engine results and stamps last_check_at.
func (s *Store) ApplyCheck(ctx context.Context, id int64, c CheckUpdate) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET status=?, ip=?, geo_country=?, geo_city=?, geo_asn=?,
			engines_json=?, last_check_at=?, updated_at=? WHERE id=?`,
		c.Status, c.IP, c.GeoCountry, c.GeoCity, c.GeoASN, c.EnginesJSON,
		nowRFC3339(), nowRFC3339(), id)
	return err
}

// SetSync stamps last_sync_at and records the sync outcome (empty errMsg = ok).
func (s *Store) SetSync(ctx context.Context, id int64, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE servers SET last_sync_at=?, last_sync_error=?, updated_at=? WHERE id=?",
		nowRFC3339(), errMsg, nowRFC3339(), id)
	return err
}

// SetProvision updates provisioning status (and optionally engines).
func (s *Store) SetProvision(ctx context.Context, id int64, status, errMsg, enginesJSON string) error {
	if enginesJSON != "" {
		_, err := s.db.ExecContext(ctx,
			"UPDATE servers SET provision_status=?, provision_error=?, engines_json=?, updated_at=? WHERE id=?",
			status, errMsg, enginesJSON, nowRFC3339(), id)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		"UPDATE servers SET provision_status=?, provision_error=?, updated_at=? WHERE id=?",
		status, errMsg, nowRFC3339(), id)
	return err
}
