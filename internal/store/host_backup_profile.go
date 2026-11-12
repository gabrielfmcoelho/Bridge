package store

import (
	"context"
	"database/sql"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

// HostBackupProfileRepo owns host_backup_profiles: a host's own backup paths.
// Not scoped itself: callers reach it through a host they could load.
type HostBackupProfileRepo struct{ db *sql.DB }

func NewHostBackupProfileRepo(db *sql.DB) *HostBackupProfileRepo {
	return &HostBackupProfileRepo{db: db}
}

// Get returns the host's profile; a host without one gets an empty profile
// (everything inherited).
func (r *HostBackupProfileRepo) Get(ctx context.Context, hostID int64) (*models.HostBackupProfile, error) {
	p := &models.HostBackupProfile{HostID: hostID}
	var include, exclude string
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(p.include, ''), COALESCE(p.exclude, ''), p.updated_by, COALESCE(u.display_name, ''), p.updated_at
		FROM host_backup_profiles p LEFT JOIN users u ON u.id = p.updated_by WHERE p.host_id = ?`, hostID).
		Scan(&include, &exclude, &p.UpdatedBy, &p.UpdatedName, &p.UpdatedAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	p.Include, p.Exclude = splitPaths(include), splitPaths(exclude)
	return p, nil
}

// Upsert writes the profile's lists; an empty list stores NULL (inherit).
func (r *HostBackupProfileRepo) Upsert(ctx context.Context, p *models.HostBackupProfile) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO host_backup_profiles (host_id, include, exclude, updated_by, updated_at)
		VALUES (?, ?, ?, ?, now())
		ON CONFLICT (host_id) DO UPDATE SET include = EXCLUDED.include, exclude = EXCLUDED.exclude,
			updated_by = EXCLUDED.updated_by, updated_at = now()`,
		p.HostID, joinPaths(p.Include), joinPaths(p.Exclude), p.UpdatedBy)
	return err
}
