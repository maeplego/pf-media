package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func apiBase() string {
	if v := os.Getenv("MEDIA_API_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:8090"
}

func webBase() string {
	if v := os.Getenv("MEDIA_WEB_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:3004"
}

func skipUnlessUp(t *testing.T) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Get(apiBase() + "/health")
	if err != nil {
		t.Skip("media API not running: ", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Skipf("media API health %d", res.StatusCode)
	}
}

func apiJSON(t *testing.T, method, path, sub string, body any) (int, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, apiBase()+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Dev-User-Sub", sub)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, b
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDriveUploadQuotaDeleteE2E(t *testing.T) {
	skipUnlessUp(t)
	sub := fmt.Sprintf("e2e-%d", time.Now().UnixNano())
	pngBytes := tinyPNG(t)

	code, body := apiJSON(t, http.MethodGet, "/v1/quota", sub, nil)
	if code != http.StatusOK {
		t.Fatalf("quota %d %s", code, body)
	}
	var quota struct {
		UsedBytes int64 `json:"usedBytes"`
	}
	if err := json.Unmarshal(body, &quota); err != nil {
		t.Fatal(err)
	}
	if quota.UsedBytes != 0 {
		t.Fatalf("fresh user used %d", quota.UsedBytes)
	}

	code, body = apiJSON(t, http.MethodPost, "/v1/uploads/presign", sub, map[string]any{
		"contentType": "image/png",
		"size":        len(pngBytes),
		"purpose":     "drive",
	})
	if code != http.StatusOK {
		t.Fatalf("presign %d %s", code, body)
	}
	var presign struct {
		FileID    string `json:"fileId"`
		UploadURL string `json:"uploadUrl"`
	}
	if err := json.Unmarshal(body, &presign); err != nil {
		t.Fatal(err)
	}

	put, err := http.NewRequest(http.MethodPut, presign.UploadURL, bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	put.Header.Set("Content-Type", "image/png")
	putRes, err := (&http.Client{Timeout: 15 * time.Second}).Do(put)
	if err != nil {
		t.Fatal(err)
	}
	putRes.Body.Close()
	if putRes.StatusCode/100 != 2 {
		t.Fatalf("garage put %d", putRes.StatusCode)
	}

	code, body = apiJSON(t, http.MethodPost, "/v1/uploads/complete", sub, map[string]any{
		"fileId": presign.FileID,
		"etag":   strings.Trim(putRes.Header.Get("Etag"), `"`),
	})
	if code != http.StatusOK {
		t.Fatalf("complete %d %s", code, body)
	}

	code, body = apiJSON(t, http.MethodGet, "/v1/quota", sub, nil)
	if err := json.Unmarshal(body, &quota); err != nil {
		t.Fatal(err)
	}
	if quota.UsedBytes != int64(len(pngBytes)) {
		t.Fatalf("used after complete %d want %d", quota.UsedBytes, len(pngBytes))
	}

	code, body = apiJSON(t, http.MethodGet, "/v1/files/"+presign.FileID, "other-"+sub, nil)
	if code != http.StatusForbidden {
		t.Fatalf("other get %d %s", code, body)
	}

	code, body = apiJSON(t, http.MethodDelete, "/v1/files/"+presign.FileID, sub, nil)
	if code != http.StatusOK {
		t.Fatalf("delete %d %s", code, body)
	}
	code, body = apiJSON(t, http.MethodDelete, "/v1/files/"+presign.FileID, sub, nil)
	if code != http.StatusOK {
		t.Fatalf("second delete %d %s", code, body)
	}

	code, body = apiJSON(t, http.MethodGet, "/v1/quota", sub, nil)
	if err := json.Unmarshal(body, &quota); err != nil {
		t.Fatal(err)
	}
	if quota.UsedBytes != 0 {
		t.Fatalf("used after delete %d", quota.UsedBytes)
	}
}

func TestDrivePageShowsQuotaE2E(t *testing.T) {
	skipUnlessUp(t)
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(webBase() + "/?user=demo-user-a")
	if err != nil {
		t.Skip("media web not running: ", err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("web %d", res.StatusCode)
	}
	html := string(b)
	if !strings.Contains(html, "容量") {
		t.Fatalf("drive page missing quota label")
	}
	if !strings.Contains(html, "demo-user-a") {
		t.Fatalf("drive page missing user")
	}
}

func tinyPDF(t *testing.T) []byte {
	t.Helper()
	return []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n")
}

func TestPDFUploadE2E(t *testing.T) {
	skipUnlessUp(t)
	sub := fmt.Sprintf("e2e-pdf-%d", time.Now().UnixNano())
	pdfBytes := tinyPDF(t)

	code, body := apiJSON(t, http.MethodPost, "/v1/uploads/presign", sub, map[string]any{
		"contentType": "application/pdf",
		"size":        len(pdfBytes),
		"purpose":     "drive",
	})
	if code != http.StatusOK {
		t.Fatalf("presign %d %s", code, body)
	}
	var presign struct {
		FileID    string `json:"fileId"`
		UploadURL string `json:"uploadUrl"`
	}
	if err := json.Unmarshal(body, &presign); err != nil {
		t.Fatal(err)
	}

	put, err := http.NewRequest(http.MethodPut, presign.UploadURL, bytes.NewReader(pdfBytes))
	if err != nil {
		t.Fatal(err)
	}
	put.Header.Set("Content-Type", "application/pdf")
	putRes, err := (&http.Client{Timeout: 15 * time.Second}).Do(put)
	if err != nil {
		t.Fatal(err)
	}
	putRes.Body.Close()
	if putRes.StatusCode/100 != 2 {
		t.Fatalf("garage put %d", putRes.StatusCode)
	}

	code, body = apiJSON(t, http.MethodPost, "/v1/uploads/complete", sub, map[string]any{
		"fileId": presign.FileID,
		"etag":   strings.Trim(putRes.Header.Get("Etag"), `"`),
	})
	if code != http.StatusOK {
		t.Fatalf("complete %d %s", code, body)
	}
	var file struct {
		Status      string `json:"status"`
		JobID       string `json:"jobId"`
		ContentType string `json:"contentType"`
		Variants    map[string]struct {
			URL string `json:"url"`
		} `json:"variants"`
	}
	if err := json.Unmarshal(body, &file); err != nil {
		t.Fatal(err)
	}
	if file.Status != "ready" || file.JobID != "" {
		t.Fatalf("file %+v", file)
	}
	if file.Variants["orig"].URL == "" {
		t.Fatal("missing orig url")
	}
}

func TestFolderDeleteE2E(t *testing.T) {
	skipUnlessUp(t)
	sub := fmt.Sprintf("e2e-folder-%d", time.Now().UnixNano())

	code, body := apiJSON(t, http.MethodPost, "/v1/folders", sub, map[string]any{"name": "empty-dir"})
	if code != http.StatusOK {
		t.Fatalf("create folder %d %s", code, body)
	}
	var folder struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &folder); err != nil {
		t.Fatal(err)
	}

	code, body = apiJSON(t, http.MethodDelete, "/v1/folders/"+folder.ID, sub, nil)
	if code != http.StatusOK {
		t.Fatalf("delete empty %d %s", code, body)
	}
	code, body = apiJSON(t, http.MethodDelete, "/v1/folders/"+folder.ID, sub, nil)
	if code != http.StatusOK {
		t.Fatalf("idempotent delete %d %s", code, body)
	}

	code, body = apiJSON(t, http.MethodPost, "/v1/folders", sub, map[string]any{"name": "full-dir"})
	if err := json.Unmarshal(body, &folder); err != nil {
		t.Fatal(err)
	}
	pngBytes := tinyPNG(t)
	code, body = apiJSON(t, http.MethodPost, "/v1/uploads/presign", sub, map[string]any{
		"contentType": "image/png",
		"size":        len(pngBytes),
		"purpose":     "drive",
		"folderId":    folder.ID,
	})
	var presign struct {
		FileID    string `json:"fileId"`
		UploadURL string `json:"uploadUrl"`
	}
	if err := json.Unmarshal(body, &presign); err != nil {
		t.Fatal(err)
	}
	put, err := http.NewRequest(http.MethodPut, presign.UploadURL, bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	put.Header.Set("Content-Type", "image/png")
	putRes, err := (&http.Client{Timeout: 15 * time.Second}).Do(put)
	if err != nil {
		t.Fatal(err)
	}
	putRes.Body.Close()
	code, body = apiJSON(t, http.MethodPost, "/v1/uploads/complete", sub, map[string]any{
		"fileId": presign.FileID,
		"etag":   strings.Trim(putRes.Header.Get("Etag"), `"`),
	})
	if code != http.StatusOK {
		t.Fatalf("complete %d %s", code, body)
	}

	code, body = apiJSON(t, http.MethodDelete, "/v1/folders/"+folder.ID, sub, nil)
	if code != http.StatusOK {
		t.Fatalf("recursive delete want 200 got %d %s", code, body)
	}
	code, body = apiJSON(t, http.MethodGet, "/v1/quota", sub, nil)
	var quotaAfter struct {
		UsedBytes int64 `json:"usedBytes"`
	}
	if err := json.Unmarshal(body, &quotaAfter); err != nil {
		t.Fatal(err)
	}
	if quotaAfter.UsedBytes != 0 {
		t.Fatalf("quota after folder delete %d", quotaAfter.UsedBytes)
	}
}
