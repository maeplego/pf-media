// Package domain is the persistence-agnostic media model.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrConflict  = errors.New("conflict")
	ErrQuota     = errors.New("quota exceeded")
	ErrInvalid   = errors.New("invalid")
	ErrTooLarge  = errors.New("too large")
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
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	Purpose     string
	Status      FileStatus
	Variants    Variants
	CreatedAt   time.Time
}

type Job struct {
	ID        string
	FileID    string
	Status    JobStatus
	Error     string
	UpdatedAt time.Time
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
	ListFilesByOwner(ctx context.Context, ownerSub string, limit int) ([]File, error)
	MarkFileReady(ctx context.Context, id string, size int64) error
	SetFileStatus(ctx context.Context, id string, status FileStatus) error
	UpdateVariants(ctx context.Context, id string, variants Variants) error
	CreateJob(ctx context.Context, j Job) error
	GetJob(ctx context.Context, id string) (Job, error)
	UpdateJob(ctx context.Context, id string, status JobStatus, errMsg string) error
	AddQuota(ctx context.Context, ownerSub string, delta int64, limit int64) error
	GetQuotaUsed(ctx context.Context, ownerSub string) (int64, error)
}
