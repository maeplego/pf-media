package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/portfolio/pf-media/api/internal/auth"
	"github.com/portfolio/pf-media/api/internal/service"
	mem "github.com/portfolio/pf-media/api/internal/store/memory"
)

func driveHandler() http.Handler {
	store := mem.New()
	svc := service.NewMedia(store, stubObjects{}, nil, 10_000, 5000, time.Minute)
	return New(svc).Routes(auth.New(true, "", ""), "processor-token")
}

func doJSON(t *testing.T, h http.Handler, method, path, sub, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if sub != "" {
		req.Header.Set("X-Dev-User-Sub", sub)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func completePNG(t *testing.T, h http.Handler, sub string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/uploads/presign", sub, `{"contentType":"image/png","size":10,"purpose":"drive"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("presign %d %s", rec.Code, rec.Body.String())
	}
	var presign struct {
		FileID string `json:"fileId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &presign); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/uploads/complete", sub, `{"fileId":"`+presign.FileID+`","etag":"etag"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete %d %s", rec.Code, rec.Body.String())
	}
	return presign.FileID
}

func TestQuotaAndDeleteHTTP(t *testing.T) {
	h := driveHandler()

	rec := doJSON(t, h, http.MethodGet, "/v1/quota", "owner", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("empty quota %d", rec.Code)
	}
	var quota struct {
		UsedBytes  int64 `json:"usedBytes"`
		LimitBytes int64 `json:"limitBytes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &quota); err != nil {
		t.Fatal(err)
	}
	if quota.UsedBytes != 0 || quota.LimitBytes != 10_000 {
		t.Fatalf("quota %+v", quota)
	}

	fileID := completePNG(t, h, "owner")

	rec = doJSON(t, h, http.MethodGet, "/v1/files", "owner", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
		Quota struct {
			UsedBytes int64 `json:"usedBytes"`
		} `json:"quota"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Files) != 1 || list.Files[0].ID != fileID {
		t.Fatalf("files %+v", list.Files)
	}
	if list.Quota.UsedBytes != 10 {
		t.Fatalf("list quota %d", list.Quota.UsedBytes)
	}

	rec = doJSON(t, h, http.MethodDelete, "/v1/files/"+fileID, "other", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("other delete %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodDelete, "/v1/files/"+fileID, "owner", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodDelete, "/v1/files/"+fileID, "owner", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("idempotent delete %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/quota", "owner", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &quota); err != nil {
		t.Fatal(err)
	}
	if quota.UsedBytes != 0 {
		t.Fatalf("quota after delete %d", quota.UsedBytes)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/files", "owner", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Files) != 0 {
		t.Fatalf("files left %+v", list.Files)
	}
}

func TestQuotaRequiresAuth(t *testing.T) {
	h := driveHandler()
	rec := doJSON(t, h, http.MethodGet, "/v1/quota", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestFoldersHTTP(t *testing.T) {
	h := driveHandler()
	rec := doJSON(t, h, http.MethodPost, "/v1/folders", "owner", `{"name":"docs"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var folder struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &folder); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/uploads/presign", "owner", `{"contentType":"image/png","size":10,"purpose":"drive","folderId":"`+folder.ID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("presign %d %s", rec.Code, rec.Body.String())
	}
	var presign struct {
		FileID string `json:"fileId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &presign); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/uploads/complete", "owner", `{"fileId":"`+presign.FileID+`","etag":"etag"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/files", "owner", "")
	var root struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
		Folders []struct {
			ID string `json:"id"`
		} `json:"folders"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	if len(root.Files) != 0 || len(root.Folders) != 1 {
		t.Fatalf("root %+v", root)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/files?folderId="+folder.ID, "owner", "")
	var inside struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &inside); err != nil {
		t.Fatal(err)
	}
	if len(inside.Files) != 1 || inside.Files[0].ID != presign.FileID {
		t.Fatalf("inside %+v", inside)
	}
}
