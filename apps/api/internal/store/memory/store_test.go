package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/portfolio/pf-media/api/internal/domain"
)

func TestAddQuotaClampsAndIgnoresLimitOnReclaim(t *testing.T) {
	s := New()
	ctx := context.Background()
	if err := s.AddQuota(ctx, "u", 80, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.AddQuota(ctx, "u", 30, 100); !errors.Is(err, domain.ErrQuota) {
		t.Fatalf("expected quota, got %v", err)
	}
	used, _ := s.GetQuotaUsed(ctx, "u")
	if used != 80 {
		t.Fatalf("used after rejected add %d", used)
	}
	if err := s.AddQuota(ctx, "u", -200, 100); err != nil {
		t.Fatal(err)
	}
	used, _ = s.GetQuotaUsed(ctx, "u")
	if used != 0 {
		t.Fatalf("clamped used %d", used)
	}
}

func TestDeleteFileRemovesJobsAndShares(t *testing.T) {
	s := New()
	ctx := context.Background()
	f := domain.File{ID: "f1", OwnerSub: "u", ObjectKey: "k", ContentType: "image/png", SizeBytes: 1, Purpose: "drive", Status: domain.FileReady, CreatedAt: time.Now().UTC()}
	if err := s.CreatePendingFile(ctx, f); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateJob(ctx, domain.Job{ID: "j1", FileID: "f1", Status: domain.JobQueued, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateShareLink(ctx, domain.ShareLink{ID: "s1", Token: "tok", FileID: "f1", OwnerSub: "u", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFile(ctx, "f1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetFile(ctx, "f1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("file: %v", err)
	}
	if _, err := s.GetJob(ctx, "j1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("job: %v", err)
	}
	if _, err := s.GetShareLinkByToken(ctx, "tok"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("share: %v", err)
	}
}
