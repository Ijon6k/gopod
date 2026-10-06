package projects

import (
	"context"
	"database/sql"
)

// SQLiteRepository implements projects.Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository for projects.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) List(ctx context.Context) ([]Project, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, description, created_at FROM projects ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	if list == nil {
		list = []Project{}
	}
	return list, rows.Err()
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (*Project, error) {
	var p Project
	err := r.db.QueryRowContext(ctx, "SELECT id, name, description, created_at FROM projects WHERE id = ?", id).
		Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *SQLiteRepository) Create(ctx context.Context, p Project) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO projects (id, name, description) VALUES (?, ?, ?)",
		p.ID, p.Name, p.Description,
	)
	return err
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	return err
}
