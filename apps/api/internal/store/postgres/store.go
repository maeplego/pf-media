package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/portfolio/pf-media/api/internal/domain"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		b, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, string(b)); err != nil {
			return fmt.Errorf("migrate %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) CreatePendingFile(ctx context.Context, f domain.File) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO files (id, owner_sub, object_key, content_type, size_bytes, purpose, status, variants)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		f.ID, f.OwnerSub, f.ObjectKey, f.ContentType, f.SizeBytes, f.Purpose, string(f.Status), f.Variants.JSON(),
	)
	return err
}

func (s *Store) GetFile(ctx context.Context, id string) (domain.File, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, owner_sub, object_key, content_type, size_bytes, purpose, status, variants, created_at
		FROM files WHERE id = $1`, id)
	return scanFile(row)
}

func (s *Store) ListFilesByOwner(ctx context.Context, ownerSub string, limit int) ([]domain.File, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, owner_sub, object_key, content_type, size_bytes, purpose, status, variants, created_at
		FROM files WHERE owner_sub = $1 ORDER BY created_at DESC LIMIT $2`, ownerSub, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.File, 0)
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) MarkFileReady(ctx context.Context, id string, size int64) error {
	tag, err := s.pool.Exec(ctx, `UPDATE files SET status = 'ready', size_bytes = $2 WHERE id = $1`, id, size)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) SetFileStatus(ctx context.Context, id string, status domain.FileStatus) error {
	tag, err := s.pool.Exec(ctx, `UPDATE files SET status = $2 WHERE id = $1`, id, string(status))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) UpdateVariants(ctx context.Context, id string, variants domain.Variants) error {
	tag, err := s.pool.Exec(ctx, `UPDATE files SET variants = $2, status = 'ready' WHERE id = $1`, id, variants.JSON())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) CreateJob(ctx context.Context, j domain.Job) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO jobs (id, file_id, status, error, updated_at)
		VALUES ($1,$2,$3,$4,$5)`,
		j.ID, j.FileID, string(j.Status), j.Error, j.UpdatedAt.UTC())
	return err
}

func (s *Store) GetJob(ctx context.Context, id string) (domain.Job, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, file_id, status, error, updated_at FROM jobs WHERE id = $1`, id)
	var j domain.Job
	var st string
	if err := row.Scan(&j.ID, &j.FileID, &st, &j.Error, &j.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Job{}, domain.ErrNotFound
		}
		return domain.Job{}, err
	}
	j.Status = domain.JobStatus(st)
	return j, nil
}

func (s *Store) UpdateJob(ctx context.Context, id string, status domain.JobStatus, errMsg string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = $2, error = $3, updated_at = $4 WHERE id = $1`,
		id, string(status), errMsg, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) AddQuota(ctx context.Context, ownerSub string, delta int64, limit int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var used int64
	err = tx.QueryRow(ctx, `
		INSERT INTO user_quota (owner_sub, used_bytes) VALUES ($1, 0)
		ON CONFLICT (owner_sub) DO UPDATE SET owner_sub = EXCLUDED.owner_sub
		RETURNING used_bytes`, ownerSub).Scan(&used)
	if err != nil {
		return err
	}
	if used+delta > limit {
		return domain.ErrQuota
	}
	_, err = tx.Exec(ctx, `UPDATE user_quota SET used_bytes = used_bytes + $2 WHERE owner_sub = $1`, ownerSub, delta)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetQuotaUsed(ctx context.Context, ownerSub string) (int64, error) {
	var used int64
	err := s.pool.QueryRow(ctx, `SELECT used_bytes FROM user_quota WHERE owner_sub = $1`, ownerSub).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return used, err
}

func (s *Store) CreateShareLink(ctx context.Context, l domain.ShareLink) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO share_links (id, token, file_id, owner_sub, password_hash, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		l.ID, l.Token, l.FileID, l.OwnerSub, l.PasswordHash, l.ExpiresAt.UTC(), l.CreatedAt.UTC())
	return err
}

func (s *Store) GetShareLinkByToken(ctx context.Context, token string) (domain.ShareLink, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, token, file_id, owner_sub, password_hash, expires_at, created_at
		FROM share_links WHERE token = $1`, token)
	var l domain.ShareLink
	if err := row.Scan(&l.ID, &l.Token, &l.FileID, &l.OwnerSub, &l.PasswordHash, &l.ExpiresAt, &l.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ShareLink{}, domain.ErrNotFound
		}
		return domain.ShareLink{}, err
	}
	return l, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanFile(row scannable) (domain.File, error) {
	var f domain.File
	var st string
	var raw []byte
	if err := row.Scan(&f.ID, &f.OwnerSub, &f.ObjectKey, &f.ContentType, &f.SizeBytes, &f.Purpose, &st, &raw, &f.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.File{}, domain.ErrNotFound
		}
		return domain.File{}, err
	}
	f.Status = domain.FileStatus(st)
	f.Variants = domain.ParseVariants(raw)
	return f, nil
}
