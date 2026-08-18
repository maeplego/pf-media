package service

import (
	"context"
	"fmt"
	"time"

	"github.com/portfolio/pf-media/api/internal/domain"
	"github.com/portfolio/pf-media/api/internal/id"
	"github.com/portfolio/pf-media/api/internal/mimeutil"
	"github.com/portfolio/pf-media/api/internal/objectstore"
	"github.com/portfolio/pf-media/api/internal/queue"
)

type ObjectStore interface {
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Stat(ctx context.Context, key string) (int64, string, error)
	Delete(ctx context.Context, key string) error
	Bucket() string
}

type JobQueue interface {
	Enqueue(ctx context.Context, msg queue.JobMessage) error
}

type minioAdapter struct {
	*objectstore.Client
}

func (a minioAdapter) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	u, err := a.Client.PresignPut(ctx, key, contentType, ttl)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (a minioAdapter) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := a.Client.PresignGet(ctx, key, ttl)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func NewObjectStore(c *objectstore.Client) ObjectStore {
	return minioAdapter{c}
}

type PresignInput struct {
	ContentType string
	Size        int64
	Purpose     string
}

type PresignResult struct {
	FileID    string `json:"fileId"`
	UploadURL string `json:"uploadUrl"`
	ObjectKey string `json:"objectKey"`
}

type FileView struct {
	ID          string                    `json:"id"`
	ContentType string                    `json:"contentType"`
	SizeBytes   int64                     `json:"sizeBytes"`
	Purpose     string                    `json:"purpose"`
	Status      domain.FileStatus         `json:"status"`
	Variants    map[string]VariantView    `json:"variants"`
	CreatedAt   time.Time                 `json:"createdAt"`
}

type VariantView struct {
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
}

type Media struct {
	store      domain.Store
	objects    ObjectStore
	queue      JobQueue
	quotaLimit int64
	maxUpload  int64
	presignTTL time.Duration
}

func NewMedia(store domain.Store, objects ObjectStore, q JobQueue, quotaLimit, maxUpload int64, presignTTL time.Duration) *Media {
	return &Media{
		store:      store,
		objects:    objects,
		queue:      q,
		quotaLimit: quotaLimit,
		maxUpload:  maxUpload,
		presignTTL: presignTTL,
	}
}

func (m *Media) Presign(ctx context.Context, ownerSub string, in PresignInput) (PresignResult, error) {
	if in.Size <= 0 {
		return PresignResult{}, domain.ErrInvalid
	}
	if in.Size > m.maxUpload {
		return PresignResult{}, domain.ErrTooLarge
	}
	if !mimeutil.AllowedImage(in.ContentType) {
		return PresignResult{}, domain.ErrInvalid
	}
	if in.Purpose == "" {
		in.Purpose = "drive"
	}
	used, err := m.store.GetQuotaUsed(ctx, ownerSub)
	if err != nil {
		return PresignResult{}, err
	}
	if used+in.Size > m.quotaLimit {
		return PresignResult{}, domain.ErrQuota
	}

	fileID := id.New()
	key := objectstore.ObjectKey(ownerSub, fileID)
	f := domain.File{
		ID:          fileID,
		OwnerSub:    ownerSub,
		ObjectKey:   key,
		ContentType: in.ContentType,
		SizeBytes:   in.Size,
		Purpose:     in.Purpose,
		Status:      domain.FilePending,
		CreatedAt:   time.Now().UTC(),
	}
	if err := m.store.CreatePendingFile(ctx, f); err != nil {
		return PresignResult{}, err
	}
	url, err := m.objects.PresignPut(ctx, key, in.ContentType, m.presignTTL)
	if err != nil {
		return PresignResult{}, err
	}
	return PresignResult{FileID: fileID, UploadURL: url, ObjectKey: key}, nil
}

func (m *Media) Complete(ctx context.Context, ownerSub, fileID, etag string) (FileView, error) {
	f, err := m.store.GetFile(ctx, fileID)
	if err != nil {
		return FileView{}, err
	}
	if f.OwnerSub != ownerSub {
		return FileView{}, domain.ErrForbidden
	}
	if f.Status != domain.FilePending {
		return FileView{}, domain.ErrConflict
	}
	size, gotETag, err := m.objects.Stat(ctx, f.ObjectKey)
	if err != nil {
		return FileView{}, domain.ErrInvalid
	}
	if size > m.maxUpload {
		_ = m.objects.Delete(ctx, f.ObjectKey)
		return FileView{}, domain.ErrTooLarge
	}
	_ = etag
	_ = gotETag

	if err := m.store.AddQuota(ctx, ownerSub, size, m.quotaLimit); err != nil {
		_ = m.objects.Delete(ctx, f.ObjectKey)
		return FileView{}, err
	}
	if err := m.store.MarkFileReady(ctx, fileID, size); err != nil {
		return FileView{}, err
	}
	f.SizeBytes = size
	f.Status = domain.FileReady

	if mimeutil.IsImage(f.ContentType) {
		jobID := id.New()
		j := domain.Job{
			ID:        jobID,
			FileID:    fileID,
			Status:    domain.JobQueued,
			UpdatedAt: time.Now().UTC(),
		}
		if err := m.store.CreateJob(ctx, j); err != nil {
			return FileView{}, err
		}
		if err := m.store.SetFileStatus(ctx, fileID, domain.FilePending); err != nil {
			return FileView{}, err
		}
		f.Status = domain.FilePending
		if m.queue != nil {
			if err := m.queue.Enqueue(ctx, queue.JobMessage{
				JobID:     jobID,
				FileID:    fileID,
				ObjectKey: f.ObjectKey,
				Bucket:    m.objects.Bucket(),
			}); err != nil {
				_ = m.store.UpdateJob(ctx, jobID, domain.JobFailed, "enqueue failed")
				_ = m.store.SetFileStatus(ctx, fileID, domain.FileFailed)
				return FileView{}, fmt.Errorf("enqueue job: %w", err)
			}
		}
	}
	return m.fileView(ctx, f)
}

func (m *Media) GetFile(ctx context.Context, requesterSub, fileID string) (FileView, error) {
	f, err := m.store.GetFile(ctx, fileID)
	if err != nil {
		return FileView{}, err
	}
	if f.OwnerSub != requesterSub {
		return FileView{}, domain.ErrForbidden
	}
	return m.fileView(ctx, f)
}

func (m *Media) ListFiles(ctx context.Context, ownerSub string, limit int) ([]FileView, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	files, err := m.store.ListFilesByOwner(ctx, ownerSub, limit)
	if err != nil {
		return nil, err
	}
	out := make([]FileView, 0, len(files))
	for _, f := range files {
		v, err := m.fileView(ctx, f)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (m *Media) FinishJob(ctx context.Context, jobID string, variants domain.Variants, jobErr string) error {
	j, err := m.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	if jobErr != "" {
		_ = m.store.UpdateJob(ctx, jobID, domain.JobFailed, jobErr)
		_ = m.store.SetFileStatus(ctx, j.FileID, domain.FileFailed)
		return nil
	}
	if err := m.store.UpdateVariants(ctx, j.FileID, variants); err != nil {
		return err
	}
	return m.store.UpdateJob(ctx, jobID, domain.JobSucceeded, "")
}

func (m *Media) fileView(ctx context.Context, f domain.File) (FileView, error) {
	view := FileView{
		ID:          f.ID,
		ContentType: f.ContentType,
		SizeBytes:   f.SizeBytes,
		Purpose:     f.Purpose,
		Status:      f.Status,
		Variants:    map[string]VariantView{},
		CreatedAt:   f.CreatedAt,
	}
	if len(f.Variants) == 0 && f.Status == domain.FileReady {
		url, err := m.objects.PresignGet(ctx, f.ObjectKey, m.presignTTL)
		if err != nil {
			return FileView{}, err
		}
		view.Variants["orig"] = VariantView{URL: url, ContentType: f.ContentType}
		return view, nil
	}
	for name, v := range f.Variants {
		url, err := m.objects.PresignGet(ctx, v.Key, m.presignTTL)
		if err != nil {
			return FileView{}, fmt.Errorf("presign %s: %w", name, err)
		}
		view.Variants[name] = VariantView{URL: url, ContentType: v.ContentType}
	}
	return view, nil
}
