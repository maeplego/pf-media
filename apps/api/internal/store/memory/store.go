package memory

import (
	"context"
	"sync"
	"time"

	"github.com/portfolio/pf-media/api/internal/domain"
)

type Store struct {
	mu      sync.Mutex
	files   map[string]domain.File
	jobs    map[string]domain.Job
	quota   map[string]int64
	shares  map[string]domain.ShareLink
	folders map[string]domain.Folder
}

func New() *Store {
	return &Store{
		files:   map[string]domain.File{},
		jobs:    map[string]domain.Job{},
		quota:   map[string]int64{},
		shares:  map[string]domain.ShareLink{},
		folders: map[string]domain.Folder{},
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

func (s *Store) ListFilesByOwner(_ context.Context, ownerSub, orgID, folderID string, limit int) ([]domain.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.File, 0)
	for _, f := range s.files {
		if f.OwnerSub == ownerSub && f.OrgID == orgID && f.FolderID == folderID {
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

func (s *Store) GetLatestJobByFile(_ context.Context, fileID string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var best domain.Job
	found := false
	for _, j := range s.jobs {
		if j.FileID != fileID {
			continue
		}
		if !found || j.UpdatedAt.After(best.UpdatedAt) {
			best = j
			found = true
		}
	}
	if !found {
		return domain.Job{}, domain.ErrNotFound
	}
	return best, nil
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
	if delta > 0 && used > limit {
		return domain.ErrQuota
	}
	if used < 0 {
		used = 0
	}
	s.quota[ownerSub] = used
	return nil
}

func (s *Store) GetQuotaUsed(_ context.Context, ownerSub string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quota[ownerSub], nil
}

func (s *Store) DeleteFile(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.files[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.files, id)
	for jid, j := range s.jobs {
		if j.FileID == id {
			delete(s.jobs, jid)
		}
	}
	for token, link := range s.shares {
		if link.FileID == id {
			delete(s.shares, token)
		}
	}
	return nil
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

func (s *Store) ListShareLinksByOwner(_ context.Context, ownerSub, orgID string) ([]domain.ShareLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.ShareLink, 0)
	for _, l := range s.shares {
		if l.OwnerSub == ownerSub && l.OrgID == orgID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (s *Store) DeleteShareLink(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.shares[token]; !ok {
		return domain.ErrNotFound
	}
	delete(s.shares, token)
	return nil
}

func (s *Store) CreateFolder(_ context.Context, f domain.Folder) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.folders[f.ID]; ok {
		return domain.ErrConflict
	}
	for _, existing := range s.folders {
		if existing.OwnerSub == f.OwnerSub && existing.OrgID == f.OrgID && existing.ParentID == f.ParentID && existing.Name == f.Name {
			return domain.ErrConflict
		}
	}
	s.folders[f.ID] = f
	return nil
}

func (s *Store) GetFolder(_ context.Context, id string) (domain.Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.folders[id]
	if !ok {
		return domain.Folder{}, domain.ErrNotFound
	}
	return f, nil
}

func (s *Store) ListFolders(_ context.Context, ownerSub, orgID, parentID string) ([]domain.Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Folder, 0)
	for _, f := range s.folders {
		if f.OwnerSub == ownerSub && f.OrgID == orgID && f.ParentID == parentID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (s *Store) FolderIsEmpty(_ context.Context, folderID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.files {
		if f.FolderID == folderID {
			return false, nil
		}
	}
	for _, f := range s.folders {
		if f.ParentID == folderID {
			return false, nil
		}
	}
	return true, nil
}

func (s *Store) DeleteFolder(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.folders[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.folders, id)
	return nil
}
