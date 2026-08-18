package memory

import (
	"context"
	"sync"
	"time"

	"github.com/portfolio/pf-media/api/internal/domain"
)

type Store struct {
	mu     sync.Mutex
	files  map[string]domain.File
	jobs   map[string]domain.Job
	quota  map[string]int64
	shares map[string]domain.ShareLink
}

func New() *Store {
	return &Store{
		files:  map[string]domain.File{},
		jobs:   map[string]domain.Job{},
		quota:  map[string]int64{},
		shares: map[string]domain.ShareLink{},
	}
}

func (s *Store) CreatePendingFile(_ context.Context, f domain.File) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.files[f.ID]; ok {
		return domain.ErrConflict
	}
	s.files[f.ID] = f
	return nil
}

func (s *Store) GetFile(_ context.Context, id string) (domain.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[id]
	if !ok {
		return domain.File{}, domain.ErrNotFound
	}
	return f, nil
}

func (s *Store) ListFilesByOwner(_ context.Context, ownerSub string, limit int) ([]domain.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.File, 0)
	for _, f := range s.files {
		if f.OwnerSub == ownerSub {
			out = append(out, f)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) MarkFileReady(_ context.Context, id string, size int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[id]
	if !ok {
		return domain.ErrNotFound
	}
	f.Status = domain.FileReady
	f.SizeBytes = size
	s.files[id] = f
	return nil
}

func (s *Store) SetFileStatus(_ context.Context, id string, status domain.FileStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[id]
	if !ok {
		return domain.ErrNotFound
	}
	f.Status = status
	s.files[id] = f
	return nil
}

func (s *Store) UpdateVariants(_ context.Context, id string, variants domain.Variants) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[id]
	if !ok {
		return domain.ErrNotFound
	}
	f.Variants = variants
	f.Status = domain.FileReady
	s.files[id] = f
	return nil
}

func (s *Store) CreateJob(_ context.Context, j domain.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = j
	return nil
}

func (s *Store) GetJob(_ context.Context, id string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, domain.ErrNotFound
	}
	return j, nil
}

func (s *Store) UpdateJob(_ context.Context, id string, status domain.JobStatus, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return domain.ErrNotFound
	}
	j.Status = status
	j.Error = errMsg
	j.UpdatedAt = time.Now().UTC()
	s.jobs[id] = j
	return nil
}

func (s *Store) AddQuota(_ context.Context, ownerSub string, delta int64, limit int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	used := s.quota[ownerSub] + delta
	if used > limit {
		return domain.ErrQuota
	}
	s.quota[ownerSub] = used
	return nil
}

func (s *Store) GetQuotaUsed(_ context.Context, ownerSub string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quota[ownerSub], nil
}

func (s *Store) CreateShareLink(_ context.Context, l domain.ShareLink) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.shares[l.Token]; ok {
		return domain.ErrConflict
	}
	s.shares[l.Token] = l
	return nil
}

func (s *Store) GetShareLinkByToken(_ context.Context, token string) (domain.ShareLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.shares[token]
	if !ok {
		return domain.ShareLink{}, domain.ErrNotFound
	}
	return l, nil
}
