package ingress

import (
	"context"
	"database/sql"
)

// SQLiteRepository implements ingress.Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository for ingress domains.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

const domainSelectCols = "id, project_id, service_id, hostname, container_port, tls, https_redirect, path_prefix, strip_path_prefix, websocket, cors, hsts, basic_auth, basic_auth_user, basic_auth_pass, dns_status, created_at"

func (r *SQLiteRepository) List(ctx context.Context, projectID string) ([]Domain, error) {
	var query string
	var args []interface{}
	if projectID != "" {
		query = "SELECT " + domainSelectCols + " FROM domains WHERE project_id = ? ORDER BY created_at DESC"
		args = append(args, projectID)
	} else {
		query = "SELECT " + domainSelectCols + " FROM domains ORDER BY created_at DESC"
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.ServiceID, &d.Hostname, &d.ContainerPort, &d.TLS, &d.HTTPSRedirect, &d.PathPrefix, &d.StripPathPrefix, &d.WebSocket, &d.CORS, &d.HSTS, &d.BasicAuth, &d.BasicAuthUser, &d.BasicAuthPass, &d.DNSStatus, &d.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	if list == nil {
		list = []Domain{}
	}
	return list, rows.Err()
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (*Domain, error) {
	var d Domain
	err := r.db.QueryRowContext(ctx,
		"SELECT "+domainSelectCols+" FROM domains WHERE id = ?", id).
		Scan(&d.ID, &d.ProjectID, &d.ServiceID, &d.Hostname, &d.ContainerPort, &d.TLS, &d.HTTPSRedirect, &d.PathPrefix, &d.StripPathPrefix, &d.WebSocket, &d.CORS, &d.HSTS, &d.BasicAuth, &d.BasicAuthUser, &d.BasicAuthPass, &d.DNSStatus, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *SQLiteRepository) Create(ctx context.Context, d Domain) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO domains (id, project_id, service_id, hostname, container_port, tls, https_redirect, path_prefix, strip_path_prefix, websocket, cors, hsts, basic_auth, basic_auth_user, basic_auth_pass, dns_status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.ProjectID, d.ServiceID, d.Hostname, d.ContainerPort, d.TLS, d.HTTPSRedirect, d.PathPrefix, d.StripPathPrefix, d.WebSocket, d.CORS, d.HSTS, d.BasicAuth, d.BasicAuthUser, d.BasicAuthPass, d.DNSStatus,
	)
	return err
}

func (r *SQLiteRepository) Update(ctx context.Context, d Domain) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE domains SET
			project_id = ?, service_id = ?, hostname = ?, container_port = ?,
			tls = ?, https_redirect = ?, path_prefix = ?, strip_path_prefix = ?,
			websocket = ?, cors = ?, hsts = ?, basic_auth = ?, basic_auth_user = ?,
			basic_auth_pass = ?, dns_status = ?
		 WHERE id = ?`,
		d.ProjectID, d.ServiceID, d.Hostname, d.ContainerPort,
		d.TLS, d.HTTPSRedirect, d.PathPrefix, d.StripPathPrefix,
		d.WebSocket, d.CORS, d.HSTS, d.BasicAuth, d.BasicAuthUser,
		d.BasicAuthPass, d.DNSStatus, d.ID,
	)
	return err
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM domains WHERE id = ?", id)
	return err
}
