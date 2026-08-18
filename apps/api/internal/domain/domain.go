// Package domain is the persistence-agnostic media model.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrForbidden        = errors.New("forbidden")
	ErrConflict         = errors.New("conflict")
	ErrQuota            = errors.New("quota exceeded")
	ErrInvalid          = errors.New("invalid")
	ErrTooLarge         = errors.New("too large")
	ErrExpired          = errors.New("expired")
	ErrPasswordRequired = errors.New("password required")
)

type FileStatus string

const (
	FilePending FileStatus = "pending"
	FileReady   FileStatus = "ready"
	FileFailed  FileStatus = "failed"
)

type JobStatus string

const (
	JobQueued     JobStatus = "queued"
	JobProcessing JobStatus = "processing"
	JobSucceeded  JobStatus = "succeeded"
	JobFailed     JobStatus = "failed"
)

type Variant struct {
	Key         string `json:"key"`
	ContentType string `json:"contentType"`
}

type Variants map[string]Variant

type File struct {
	ID          string
	OwnerSub    string
	FolderID    string
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	Purpose     string
	Status      FileStatus
	Variants    Variants
	CreatedAt   time.Time
}

type Folder struct {
	ID        string
	OwnerSub  string
	ParentID  string
	Name      string
	CreatedAt time.Time
}

type Job struct {
	ID        string
	FileID    string
	Status    JobStatus
	Error     string
	UpdatedAt time.Time
}

type ShareLink struct {
	ID           string
	Token        string
	FileID       string
	OwnerSub     string
	PasswordHash string
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

func (v Variants) JSON() json.RawMessage {
	if v == nil {
		return json.RawMessage(`{}`)
	}
	b, _ := json.Marshal(v)
	return b
}

func ParseVariants(raw json.RawMessage) Variants {
	if len(raw) == 0 {
		return Variants{}
	}
	var out Variants
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		return Variants{}
	}
	return out
}

type Store interface {
	CreatePendingFile(ctx context.Context, f File) error
	GetFile(ctx context.Context, id string) (File, error)
	ListFilesByOwner(ctx context.Context, ownerSub, folderID string, limit int) ([]File, error)
	MarkFileReady(ctx context.Context, id string, size int64) error
	SetFileStatus(ctx context.Context, id string, status FileStatus) error
	UpdateVariants(ctx context.Context, id string, variants Variants) error
	CreateJob(ctx context.Context, j Job) error
	GetJob(ctx context.Context, id string) (Job, error)
	GetLatestJobByFile(ctx context.Context, fileID string) (Job, error)
	UpdateJob(ctx context.Context, id string, status JobStatus, errMsg string) error
	AddQuota(ctx context.Context, ownerSub string, delta int64, limit int64) error
	GetQuotaUsed(ctx context.Context, ownerSub string) (int64, error)
	DeleteFile(ctx context.Context, id string) error
	CreateShareLink(ctx context.Context, l ShareLink) error
	GetShareLinkByToken(ctx context.Context, token string) (ShareLink, error)
	CreateFolder(ctx context.Context, f Folder) error
	GetFolder(ctx context.Context, id string) (Folder, error)
	ListFolders(ctx context.Context, ownerSub, parentID string) ([]Folder, error)
	FolderIsEmpty(ctx context.Context, folderID string) (bool, error)
	DeleteFolder(ctx context.Context, id string) error
}
