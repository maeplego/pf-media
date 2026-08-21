package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/portfolio/pf-media/api/internal/domain"
	"github.com/portfolio/pf-media/api/internal/id"
	"github.com/portfolio/pf-media/api/internal/mimeutil"
	"github.com/portfolio/pf-media/api/internal/objectstore"
	"github.com/portfolio/pf-media/api/internal/password"
	"github.com/portfolio/pf-media/api/internal/queue"
	"github.com/portfolio/pf-media/api/internal/sharetoken"
)

type ObjectStore interface {
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Stat(ctx context.Context, key string) (int64, string, error)
	ReadHead(ctx context.Context, key string, max int) ([]byte, error)
	Delete(ctx context.Context, key string) error
	DeletePrefix(ctx context.Context, prefix string) error
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
	FolderID    string
}

type PresignResult struct {
	FileID    string `json:"fileId"`
	UploadURL string `json:"uploadUrl"`
	ObjectKey string `json:"objectKey"`
}

type FileView struct {
	ID          string                 `json:"id"`
	FolderID    string                 `json:"folderId,omitempty"`
	ContentType string                 `json:"contentType"`
	SizeBytes   int64                  `json:"sizeBytes"`
	Purpose     string                 `json:"purpose"`
	Status      domain.FileStatus      `json:"status"`
	Variants    map[string]VariantView `json:"variants"`
	CreatedAt   time.Time              `json:"createdAt"`
	JobID       string                 `json:"jobId,omitempty"`
	JobStatus   domain.JobStatus       `json:"jobStatus,omitempty"`
	JobError    string                 `json:"jobError,omitempty"`
}

type FolderView struct {
	ID        string    `json:"id"`
	ParentID  string    `json:"parentId,omitempty"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type JobView struct {
	ID        string           `json:"id"`
	FileID    string           `json:"fileId"`
	Status    domain.JobStatus `json:"status"`
	Error     string           `json:"error"`
	UpdatedAt time.Time        `json:"updatedAt"`
}

type ShareLinkView struct {
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expiresAt"`
	PasswordSet bool      `json:"passwordSet"`
}

type PublicFile struct {
	ContentType string                 `json:"contentType"`
	Status      domain.FileStatus      `json:"status"`
	Variants    map[string]VariantView `json:"variants"`
	ExpiresAt   time.Time              `json:"expiresAt"`
}

type QuotaView struct {
	UsedBytes  int64 `json:"usedBytes"`
	LimitBytes int64 `json:"limitBytes"`
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

func quotaSubject(orgID, ownerSub string) string {
	if strings.TrimSpace(orgID) != "" {
		return "org:" + orgID
	}
	return "user:" + ownerSub
}

var allowedPurposes = map[string]struct{}{
	"drive": {}, "wiki": {}, "chat": {}, "blog": {}, "blog-cover": {}, "product": {},
}

func normalizeETag(s string) string {
	return strings.Trim(s, `"`)
}

func (m *Media) Presign(ctx context.Context, ownerSub, orgID string, in PresignInput) (PresignResult, error) {
	if in.Size <= 0 {
		return PresignResult{}, domain.ErrInvalid
	}
	if in.Size > m.maxUpload {
		return PresignResult{}, domain.ErrTooLarge
	}
	if !mimeutil.AllowedUpload(in.ContentType) {
		return PresignResult{}, domain.ErrInvalid
	}
	if in.Purpose == "" {
		in.Purpose = "drive"
	} else if _, ok := allowedPurposes[in.Purpose]; !ok {
		return PresignResult{}, domain.ErrInvalid
	}
	used, err := m.store.GetQuotaUsed(ctx, quotaSubject(orgID, ownerSub))
	if err != nil {
		return PresignResult{}, err
	}
	if used+in.Size > m.quotaLimit {
		return PresignResult{}, domain.ErrQuota
	}
	if err := m.ensureOwnedFolder(ctx, ownerSub, orgID, in.FolderID); err != nil {
		return PresignResult{}, err
	}

	fileID := id.New()
	key := objectstore.ObjectKey(ownerSub, fileID)
	f := domain.File{
		ID:          fileID,
		OwnerSub:    ownerSub,
		OrgID:       orgID,
		FolderID:    in.FolderID,
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

func (m *Media) Complete(ctx context.Context, ownerSub, orgID, fileID, etag string) (FileView, error) {
	f, err := m.store.GetFile(ctx, fileID)
	if err != nil {
		return FileView{}, err
	}
	if f.OwnerSub != ownerSub || f.OrgID != orgID {
		return FileView{}, domain.ErrNotFound
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
	head, err := m.objects.ReadHead(ctx, f.ObjectKey, 512)
	if err != nil {
		_ = m.objects.Delete(ctx, f.ObjectKey)
		return FileView{}, domain.ErrInvalid
	}
	if !mimeutil.MatchesUpload(f.ContentType, head) {
		_ = m.objects.Delete(ctx, f.ObjectKey)
		_ = m.store.DeleteFile(ctx, fileID)
		return FileView{}, domain.ErrInvalid
	}
	normGot := normalizeETag(gotETag)
	normClient := normalizeETag(etag)
	if normGot != "" && (normClient == "" || normClient != normGot) {
		return FileView{}, domain.ErrInvalid
	}

	if err := m.store.AddQuota(ctx, quotaSubject(orgID, ownerSub), size, m.quotaLimit); err != nil {
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

const (
	defaultShareTTL = time.Hour
	maxShareTTL     = 7 * 24 * time.Hour
)

func (m *Media) CreateShareLink(ctx context.Context, ownerSub, orgID, fileID string, ttl time.Duration, passwordPlain string) (ShareLinkView, error) {
	if ttl <= 0 {
		ttl = defaultShareTTL
	}
	if ttl > maxShareTTL {
		return ShareLinkView{}, domain.ErrInvalid
	}
	if len(passwordPlain) > 128 {
		return ShareLinkView{}, domain.ErrInvalid
	}
	f, err := m.store.GetFile(ctx, fileID)
	if err != nil {
		return ShareLinkView{}, err
	}
	if f.OwnerSub != ownerSub || f.OrgID != orgID {
		return ShareLinkView{}, domain.ErrNotFound
	}
	if f.Status == domain.FileFailed {
		return ShareLinkView{}, domain.ErrInvalid
	}
	token, err := sharetoken.New()
	if err != nil {
		return ShareLinkView{}, err
	}
	hash := ""
	if passwordPlain != "" {
		hash, err = password.Hash(passwordPlain)
		if err != nil {
			return ShareLinkView{}, err
		}
	}
	now := time.Now().UTC()
	link := domain.ShareLink{
		ID:           id.New(),
		Token:        token,
		FileID:       fileID,
		OwnerSub:     ownerSub,
		OrgID:        orgID,
		PasswordHash: hash,
		ExpiresAt:    now.Add(ttl),
		CreatedAt:    now,
	}
	if err := m.store.CreateShareLink(ctx, link); err != nil {
		return ShareLinkView{}, err
	}
	return ShareLinkView{Token: token, ExpiresAt: link.ExpiresAt, PasswordSet: hash != ""}, nil
}

func (m *Media) ListShareLinks(ctx context.Context, ownerSub, orgID string) ([]ShareLinkView, error) {
	links, err := m.store.ListShareLinksByOwner(ctx, ownerSub, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]ShareLinkView, 0, len(links))
	for _, l := range links {
		out = append(out, ShareLinkView{
			Token:       l.Token,
			ExpiresAt:   l.ExpiresAt,
			PasswordSet: l.PasswordHash != "",
		})
	}
	return out, nil
}

func (m *Media) DeleteShareLink(ctx context.Context, ownerSub, orgID, token string) error {
	if token == "" {
		return domain.ErrNotFound
	}
	link, err := m.store.GetShareLinkByToken(ctx, token)
	if err != nil {
		return err
	}
	if link.OwnerSub != ownerSub || link.OrgID != orgID {
		return domain.ErrNotFound
	}
	return m.store.DeleteShareLink(ctx, token)
}

func sharePasswordOK(link domain.ShareLink, provided string) error {
	if link.PasswordHash == "" {
		return nil
	}
	if provided == "" {
		return domain.ErrPasswordRequired
	}
	ok, err := password.Verify(provided, link.PasswordHash)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (m *Media) ResolveShare(ctx context.Context, token, passwordPlain string) (PublicFile, error) {
	if token == "" {
		return PublicFile{}, domain.ErrNotFound
	}
	link, err := m.store.GetShareLinkByToken(ctx, token)
	if err != nil {
		return PublicFile{}, err
	}
	if !time.Now().UTC().Before(link.ExpiresAt) {
		return PublicFile{}, domain.ErrExpired
	}
	if err := sharePasswordOK(link, passwordPlain); err != nil {
		return PublicFile{}, err
	}
	f, err := m.store.GetFile(ctx, link.FileID)
	if err != nil {
		return PublicFile{}, err
	}
	view, err := m.fileView(ctx, f)
	if err != nil {
		return PublicFile{}, err
	}
	return PublicFile{
		ContentType: view.ContentType,
		Status:      view.Status,
		Variants:    view.Variants,
		ExpiresAt:   link.ExpiresAt,
	}, nil
}

func (m *Media) ShareDownloadURL(ctx context.Context, token, variant, passwordPlain string) (string, error) {
	pub, err := m.ResolveShare(ctx, token, passwordPlain)
	if err != nil {
		return "", err
	}
	if variant == "" {
		variant = "orig"
	}
	if v, ok := pub.Variants[variant]; ok {
		return v.URL, nil
	}
	if v, ok := pub.Variants["orig"]; ok {
		return v.URL, nil
	}
	for _, v := range pub.Variants {
		return v.URL, nil
	}
	return "", domain.ErrNotFound
}

func (m *Media) GetFile(ctx context.Context, requesterSub, orgID, fileID string) (FileView, error) {
	f, err := m.store.GetFile(ctx, fileID)
	if err != nil {
		return FileView{}, err
	}
	if f.OwnerSub != requesterSub || f.OrgID != orgID {
		return FileView{}, domain.ErrNotFound
	}
	return m.withJob(ctx, f)
}

func (m *Media) ListFiles(ctx context.Context, ownerSub, orgID, folderID string, limit int) ([]FileView, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if err := m.ensureOwnedFolder(ctx, ownerSub, orgID, folderID); err != nil {
		return nil, err
	}
	files, err := m.store.ListFilesByOwner(ctx, ownerSub, orgID, folderID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]FileView, 0, len(files))
	for _, f := range files {
		v, err := m.withJob(ctx, f)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (m *Media) GetQuota(ctx context.Context, ownerSub, orgID string) (QuotaView, error) {
	used, err := m.store.GetQuotaUsed(ctx, quotaSubject(orgID, ownerSub))
	if err != nil {
		return QuotaView{}, err
	}
	return QuotaView{UsedBytes: used, LimitBytes: m.quotaLimit}, nil
}

func folderNameOK(name string) bool {
	if name != strings.TrimSpace(name) {
		return false
	}
	if name == "" || name == "." || name == ".." || len(name) > 64 {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	return true
}

func (m *Media) ensureOwnedFolder(ctx context.Context, ownerSub, orgID, folderID string) error {
	if folderID == "" {
		return nil
	}
	f, err := m.store.GetFolder(ctx, folderID)
	if err != nil {
		return err
	}
	if f.OwnerSub != ownerSub || f.OrgID != orgID {
		return domain.ErrForbidden
	}
	return nil
}

func (m *Media) CreateFolder(ctx context.Context, ownerSub, orgID, name, parentID string) (FolderView, error) {
	if !folderNameOK(name) {
		return FolderView{}, domain.ErrInvalid
	}
	if err := m.ensureOwnedFolder(ctx, ownerSub, orgID, parentID); err != nil {
		return FolderView{}, err
	}
	now := time.Now().UTC()
	f := domain.Folder{
		ID:        id.New(),
		OwnerSub:  ownerSub,
		OrgID:     orgID,
		ParentID:  parentID,
		Name:      name,
		CreatedAt: now,
	}
	if err := m.store.CreateFolder(ctx, f); err != nil {
		return FolderView{}, err
	}
	return FolderView{ID: f.ID, ParentID: f.ParentID, Name: f.Name, CreatedAt: f.CreatedAt}, nil
}

func (m *Media) ListFolders(ctx context.Context, ownerSub, orgID, parentID string) ([]FolderView, error) {
	if err := m.ensureOwnedFolder(ctx, ownerSub, orgID, parentID); err != nil {
		return nil, err
	}
	folders, err := m.store.ListFolders(ctx, ownerSub, orgID, parentID)
	if err != nil {
		return nil, err
	}
	out := make([]FolderView, 0, len(folders))
	for _, f := range folders {
		out = append(out, FolderView{ID: f.ID, ParentID: f.ParentID, Name: f.Name, CreatedAt: f.CreatedAt})
	}
	return out, nil
}

func (m *Media) GetFolder(ctx context.Context, ownerSub, orgID, folderID string) (FolderView, error) {
	f, err := m.store.GetFolder(ctx, folderID)
	if err != nil {
		return FolderView{}, err
	}
	if f.OwnerSub != ownerSub || f.OrgID != orgID {
		return FolderView{}, domain.ErrForbidden
	}
	return FolderView{ID: f.ID, ParentID: f.ParentID, Name: f.Name, CreatedAt: f.CreatedAt}, nil
}

func (m *Media) DeleteFolder(ctx context.Context, ownerSub, orgID, folderID string) error {
	f, err := m.store.GetFolder(ctx, folderID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if f.OwnerSub != ownerSub || f.OrgID != orgID {
		return domain.ErrForbidden
	}
	return m.deleteFolderTree(ctx, ownerSub, orgID, folderID)
}

func (m *Media) deleteFolderTree(ctx context.Context, ownerSub, orgID, folderID string) error {
	children, err := m.store.ListFolders(ctx, ownerSub, orgID, folderID)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := m.deleteFolderTree(ctx, ownerSub, orgID, child.ID); err != nil {
			return err
		}
	}
	const batch = 100
	for {
		files, err := m.store.ListFilesByOwner(ctx, ownerSub, orgID, folderID, batch)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			break
		}
		for _, file := range files {
			if err := m.DeleteFile(ctx, ownerSub, orgID, file.ID); err != nil {
				return err
			}
		}
		if len(files) < batch {
			break
		}
	}
	return m.store.DeleteFolder(ctx, folderID)
}

func (m *Media) DeleteFile(ctx context.Context, ownerSub, orgID, fileID string) error {
	f, err := m.store.GetFile(ctx, fileID)
	if errors.Is(err, domain.ErrNotFound) {
		// Next の server action が二重に飛ぶことがある。既に消えていれば成功と同じ。
		return nil
	}
	if err != nil {
		return err
	}
	if f.OwnerSub != ownerSub || f.OrgID != orgID {
		return domain.ErrNotFound
	}
	if err := m.objects.DeletePrefix(ctx, objectstore.ObjectPrefix(f.OwnerSub, f.ID)); err != nil {
		return err
	}
	if err := m.store.DeleteFile(ctx, f.ID); err != nil {
		return err
	}
	if f.SizeBytes == 0 {
		return nil
	}
	return m.store.AddQuota(ctx, quotaSubject(orgID, ownerSub), -f.SizeBytes, m.quotaLimit)
}

func (m *Media) GetJob(ctx context.Context, ownerSub, orgID, jobID string) (JobView, error) {
	j, f, err := m.jobOwnedBy(ctx, ownerSub, orgID, jobID)
	if err != nil {
		return JobView{}, err
	}
	_ = f
	return JobView{ID: j.ID, FileID: j.FileID, Status: j.Status, Error: j.Error, UpdatedAt: j.UpdatedAt}, nil
}

func (m *Media) RetryJob(ctx context.Context, ownerSub, orgID, jobID string) (JobView, error) {
	j, f, err := m.jobOwnedBy(ctx, ownerSub, orgID, jobID)
	if err != nil {
		return JobView{}, err
	}
	if j.Status != domain.JobFailed {
		return JobView{}, domain.ErrConflict
	}
	if err := m.store.UpdateJob(ctx, jobID, domain.JobQueued, ""); err != nil {
		return JobView{}, err
	}
	if err := m.store.SetFileStatus(ctx, f.ID, domain.FilePending); err != nil {
		return JobView{}, err
	}
	if m.queue != nil {
		if err := m.queue.Enqueue(ctx, queue.JobMessage{
			JobID:     j.ID,
			FileID:    f.ID,
			ObjectKey: f.ObjectKey,
			Bucket:    m.objects.Bucket(),
		}); err != nil {
			_ = m.store.UpdateJob(ctx, jobID, domain.JobFailed, "enqueue failed")
			_ = m.store.SetFileStatus(ctx, f.ID, domain.FileFailed)
			return JobView{}, fmt.Errorf("enqueue job: %w", err)
		}
	}
	j.Status = domain.JobQueued
	j.Error = ""
	return JobView{ID: j.ID, FileID: j.FileID, Status: j.Status, Error: j.Error, UpdatedAt: time.Now().UTC()}, nil
}

func (m *Media) jobOwnedBy(ctx context.Context, ownerSub, orgID, jobID string) (domain.Job, domain.File, error) {
	j, err := m.store.GetJob(ctx, jobID)
	if err != nil {
		return domain.Job{}, domain.File{}, err
	}
	f, err := m.store.GetFile(ctx, j.FileID)
	if err != nil {
		return domain.Job{}, domain.File{}, err
	}
	if f.OwnerSub != ownerSub || f.OrgID != orgID {
		return domain.Job{}, domain.File{}, domain.ErrForbidden
	}
	return j, f, nil
}

func (m *Media) withJob(ctx context.Context, f domain.File) (FileView, error) {
	view, err := m.fileView(ctx, f)
	if err != nil {
		return FileView{}, err
	}
	j, err := m.store.GetLatestJobByFile(ctx, f.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return view, nil
		}
		return FileView{}, err
	}
	view.JobID = j.ID
	view.JobStatus = j.Status
	view.JobError = j.Error
	return view, nil
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
		FolderID:    f.FolderID,
		ContentType: f.ContentType,
		SizeBytes:   f.SizeBytes,
		Purpose:     f.Purpose,
		Status:      f.Status,
		Variants:    map[string]VariantView{},
		CreatedAt:   f.CreatedAt,
	}
	if len(f.Variants) == 0 {
		if f.Status == domain.FileFailed {
			return view, nil
		}
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
