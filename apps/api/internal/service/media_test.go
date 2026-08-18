package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/portfolio/pf-media/api/internal/domain"
	"github.com/portfolio/pf-media/api/internal/queue"
	mem "github.com/portfolio/pf-media/api/internal/store/memory"
)

type fakeObjects struct {
	keys map[string]int64
}

func (f *fakeObjects) PresignPut(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	if f.keys == nil {
		f.keys = map[string]int64{}
	}
	f.keys[key] = 0
	return "http://upload/" + key, nil
}

func (f *fakeObjects) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "http://get/" + key, nil
}

func (f *fakeObjects) Stat(_ context.Context, key string) (int64, string, error) {
	return f.keys[key], "etag", nil
}

func (f *fakeObjects) Delete(_ context.Context, key string) error {
	delete(f.keys, key)
	return nil
}

func (f *fakeObjects) DeletePrefix(_ context.Context, prefix string) error {
	for k := range f.keys {
		if strings.HasPrefix(k, prefix) {
			delete(f.keys, k)
		}
	}
	return nil
}

func (f *fakeObjects) Bucket() string { return "media" }

type failQueue struct {
	err error
	n   int
}

func (q *failQueue) Enqueue(_ context.Context, _ queue.JobMessage) error {
	q.n++
	if q.err != nil {
		return q.err
	}
	return nil
}

func TestPresignCompleteQuota(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	svc := NewMedia(store, objs, nil, 1000, 2000, time.Minute)

	res, err := svc.Presign(context.Background(), "user-a", PresignInput{
		ContentType: "image/png",
		Size:        100,
		Purpose:     "drive",
	})
	if err != nil {
		t.Fatal(err)
	}
	objs.keys[res.ObjectKey] = 100

	view, err := svc.Complete(context.Background(), "user-a", res.FileID, "etag")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != domain.FilePending {
		t.Fatalf("expected pending for image processing, got %s", view.Status)
	}

	_, err = svc.Presign(context.Background(), "user-a", PresignInput{
		ContentType: "image/jpeg",
		Size:        950,
		Purpose:     "drive",
	})
	if err != domain.ErrQuota {
		t.Fatalf("expected quota error, got %v", err)
	}
}

func TestCompleteForbidden(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	svc := NewMedia(store, objs, nil, 10000, 5000, time.Minute)
	res, _ := svc.Presign(context.Background(), "owner", PresignInput{
		ContentType: "image/png", Size: 10, Purpose: "drive",
	})
	objs.keys[res.ObjectKey] = 10
	_, err := svc.Complete(context.Background(), "other", res.FileID, "")
	if err != domain.ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestPresignTooLarge(t *testing.T) {
	svc := NewMedia(mem.New(), &fakeObjects{}, nil, 10_000, 100, time.Minute)
	_, err := svc.Presign(context.Background(), "u", PresignInput{
		ContentType: "image/png", Size: 101, Purpose: "drive",
	})
	if err != domain.ErrTooLarge {
		t.Fatalf("expected too large, got %v", err)
	}
}

func TestCompleteTooLargeDeletesObject(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	svc := NewMedia(store, objs, nil, 10_000, 50, time.Minute)
	res, err := svc.Presign(context.Background(), "u", PresignInput{
		ContentType: "image/png", Size: 40, Purpose: "drive",
	})
	if err != nil {
		t.Fatal(err)
	}
	objs.keys[res.ObjectKey] = 80
	_, err = svc.Complete(context.Background(), "u", res.FileID, "etag")
	if err != domain.ErrTooLarge {
		t.Fatalf("expected too large, got %v", err)
	}
	if _, ok := objs.keys[res.ObjectKey]; ok {
		t.Fatal("oversized object should be deleted")
	}
}

func TestCompleteEnqueueFailureMarksFailed(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	q := &failQueue{err: errors.New("redis down")}
	svc := NewMedia(store, objs, q, 10_000, 5000, time.Minute)
	res, err := svc.Presign(context.Background(), "u", PresignInput{
		ContentType: "image/png", Size: 10, Purpose: "drive",
	})
	if err != nil {
		t.Fatal(err)
	}
	objs.keys[res.ObjectKey] = 10
	_, err = svc.Complete(context.Background(), "u", res.FileID, "etag")
	if err == nil {
		t.Fatal("expected enqueue error")
	}
	if q.n != 1 {
		t.Fatalf("enqueue called %d times", q.n)
	}
	f, err := store.GetFile(context.Background(), res.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if f.Status != domain.FileFailed {
		t.Fatalf("expected failed, got %s", f.Status)
	}
}

func completeOwnedPNG(t *testing.T, svc *Media, objs *fakeObjects, owner string) string {
	t.Helper()
	res, err := svc.Presign(context.Background(), owner, PresignInput{
		ContentType: "image/png", Size: 10, Purpose: "drive",
	})
	if err != nil {
		t.Fatal(err)
	}
	objs.keys[res.ObjectKey] = 10
	if _, err := svc.Complete(context.Background(), owner, res.FileID, "etag"); err != nil {
		t.Fatal(err)
	}
	return res.FileID
}

func TestCreateShareLinkForbidden(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	svc := NewMedia(store, objs, nil, 10_000, 5000, time.Minute)
	fileID := completeOwnedPNG(t, svc, objs, "owner")
	_, err := svc.CreateShareLink(context.Background(), "other", fileID, time.Hour, "")
	if err != domain.ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestShareLinkResolveAndExpiry(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	svc := NewMedia(store, objs, nil, 10_000, 5000, time.Minute)
	fileID := completeOwnedPNG(t, svc, objs, "owner")

	link, err := svc.CreateShareLink(context.Background(), "owner", fileID, time.Hour, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(link.Token) < 43 {
		t.Fatalf("token too short: %s", link.Token)
	}

	pub, err := svc.ResolveShare(context.Background(), link.Token, "")
	if err != nil {
		t.Fatal(err)
	}
	if pub.Variants["orig"].URL == "" {
		t.Fatal("expected signed orig url")
	}

	expired := domain.ShareLink{
		ID:        "expired-id",
		Token:     "expired-token-value-that-is-not-guessable",
		FileID:    fileID,
		OwnerSub:  "owner",
		ExpiresAt: time.Now().UTC().Add(-time.Minute),
		CreatedAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := store.CreateShareLink(context.Background(), expired); err != nil {
		t.Fatal(err)
	}
	_, err = svc.ResolveShare(context.Background(), expired.Token, "")
	if err != domain.ErrExpired {
		t.Fatalf("expected expired, got %v", err)
	}
}

func TestShareDownloadURL(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	svc := NewMedia(store, objs, nil, 10_000, 5000, time.Minute)
	fileID := completeOwnedPNG(t, svc, objs, "owner")
	link, err := svc.CreateShareLink(context.Background(), "owner", fileID, time.Minute, "")
	if err != nil {
		t.Fatal(err)
	}
	url, err := svc.ShareDownloadURL(context.Background(), link.Token, "orig", "")
	if err != nil {
		t.Fatal(err)
	}
	if url == "" {
		t.Fatal("empty download url")
	}
}

func TestShareLinkPassword(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	svc := NewMedia(store, objs, nil, 10_000, 5000, time.Minute)
	fileID := completeOwnedPNG(t, svc, objs, "owner")
	link, err := svc.CreateShareLink(context.Background(), "owner", fileID, time.Hour, "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if !link.PasswordSet {
		t.Fatal("expected passwordSet")
	}
	_, err = svc.ResolveShare(context.Background(), link.Token, "")
	if err != domain.ErrPasswordRequired {
		t.Fatalf("expected password required, got %v", err)
	}
	_, err = svc.ResolveShare(context.Background(), link.Token, "nope")
	if err != domain.ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
	pub, err := svc.ResolveShare(context.Background(), link.Token, "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if pub.Variants["orig"].URL == "" {
		t.Fatal("expected variants after correct password")
	}
}

func TestRetryFailedJob(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	q := &failQueue{}
	svc := NewMedia(store, objs, q, 10_000, 5000, time.Minute)
	fileID := completeOwnedPNG(t, svc, objs, "owner")
	view, err := svc.GetFile(context.Background(), "owner", fileID)
	if err != nil {
		t.Fatal(err)
	}
	if view.JobID == "" {
		t.Fatal("expected job id after complete")
	}
	if err := svc.FinishJob(context.Background(), view.JobID, nil, "boom"); err != nil {
		t.Fatal(err)
	}
	_, err = svc.RetryJob(context.Background(), "other", view.JobID)
	if err != domain.ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
	got, err := svc.RetryJob(context.Background(), "owner", view.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.JobQueued {
		t.Fatalf("status %s", got.Status)
	}
	if q.n < 1 {
		t.Fatal("expected re-enqueue")
	}
	_, err = svc.RetryJob(context.Background(), "owner", view.JobID)
	if err != domain.ErrConflict {
		t.Fatalf("retry of queued job: %v", err)
	}
}

func TestGetQuotaAfterComplete(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	svc := NewMedia(store, objs, nil, 10_000, 5000, time.Minute)
	q, err := svc.GetQuota(context.Background(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	if q.UsedBytes != 0 || q.LimitBytes != 10_000 {
		t.Fatalf("empty quota %+v", q)
	}
	_ = completeOwnedPNG(t, svc, objs, "owner")
	q, err = svc.GetQuota(context.Background(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	if q.UsedBytes != 10 {
		t.Fatalf("used %d", q.UsedBytes)
	}
}

func TestDeleteFileReclaimsQuotaAndObjects(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	svc := NewMedia(store, objs, nil, 10_000, 5000, time.Minute)
	fileID := completeOwnedPNG(t, svc, objs, "owner")
	objs.keys["user/owner/"+fileID+"/thumb"] = 4
	if _, err := svc.CreateShareLink(context.Background(), "owner", fileID, time.Hour, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteFile(context.Background(), "other", fileID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("other user: %v", err)
	}
	q, err := svc.GetQuota(context.Background(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	if q.UsedBytes != 10 {
		t.Fatalf("quota changed after forbidden delete: %d", q.UsedBytes)
	}
	if err := svc.DeleteFile(context.Background(), "owner", fileID); err != nil {
		t.Fatal(err)
	}
	q, err = svc.GetQuota(context.Background(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	if q.UsedBytes != 0 {
		t.Fatalf("used after delete %d", q.UsedBytes)
	}
	prefix := "user/owner/" + fileID + "/"
	for k := range objs.keys {
		if strings.HasPrefix(k, prefix) {
			t.Fatalf("object left %s", k)
		}
	}
	if _, err := store.GetFile(context.Background(), fileID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("file still stored: %v", err)
	}
	if err := svc.DeleteFile(context.Background(), "owner", fileID); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestFolderHoldsFiles(t *testing.T) {
	store := mem.New()
	objs := &fakeObjects{}
	svc := NewMedia(store, objs, nil, 10_000, 5000, time.Minute)
	ctx := context.Background()
	if _, err := svc.CreateFolder(ctx, "owner", "../x", ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("slash name: %v", err)
	}
	folder, err := svc.CreateFolder(ctx, "owner", "photos", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateFolder(ctx, "other", "photos", folder.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("other parent: %v", err)
	}
	listed, err := svc.ListFolders(ctx, "owner", "")
	if err != nil || len(listed) != 1 || listed[0].ID != folder.ID {
		t.Fatalf("list %+v %v", listed, err)
	}
	res, err := svc.Presign(ctx, "owner", PresignInput{ContentType: "image/png", Size: 10, Purpose: "drive", FolderID: folder.ID})
	if err != nil {
		t.Fatal(err)
	}
	objs.keys[res.ObjectKey] = 10
	if _, err := svc.Complete(ctx, "owner", res.FileID, "etag"); err != nil {
		t.Fatal(err)
	}
	root, err := svc.ListFiles(ctx, "owner", "", 50)
	if err != nil || len(root) != 0 {
		t.Fatalf("root files %+v %v", root, err)
	}
	inside, err := svc.ListFiles(ctx, "owner", folder.ID, 50)
	if err != nil || len(inside) != 1 || inside[0].ID != res.FileID {
		t.Fatalf("folder files %+v %v", inside, err)
	}
	if _, err := svc.ListFiles(ctx, "other", folder.ID, 50); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("other list folder: %v", err)
	}
}
