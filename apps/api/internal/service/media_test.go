package service

import (
	"context"
	"testing"
	"time"

	"github.com/portfolio/pf-media/api/internal/domain"
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

func (f *fakeObjects) Bucket() string { return "media" }

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
