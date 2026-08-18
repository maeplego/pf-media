package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/portfolio/pf-media/api/internal/auth"
	"github.com/portfolio/pf-media/api/internal/domain"
	"github.com/portfolio/pf-media/api/internal/service"
)

type Server struct {
	media *service.Media
}

func New(media *service.Media) *Server {
	return &Server{media: media}
}

func (s *Server) Routes(mw *auth.Middleware, processorToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	user := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/uploads/presign":
			s.presign(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/uploads/complete":
			s.complete(w, r)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/files":
			s.listFiles(w, r)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/quota":
			s.getQuota(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/folders":
			s.createFolder(w, r)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/folders":
			s.listFolders(w, r)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/folders/"):
			s.getFolder(w, r)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/files/"):
			s.deleteFile(w, r)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/files/"):
			s.getFile(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/share-links":
			s.createShare(w, r)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/jobs/"):
			s.getJob(w, r)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/jobs/") && strings.HasSuffix(r.URL.Path, "/retry"):
			s.retryJob(w, r)
		default:
			http.NotFound(w, r)
		}
	}))

	mux.Handle("GET /v1/s/{token}", http.HandlerFunc(s.getShare))
	mux.Handle("GET /v1/s/{token}/download", http.HandlerFunc(s.downloadShare))
	mux.Handle("/v1/", user)
	mux.Handle("POST /internal/v1/jobs/{id}/finish", auth.ProcessorToken(processorToken)(http.HandlerFunc(s.finishJob)))
	return mux
}

func (s *Server) presign(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		ContentType string `json:"contentType"`
		Size        int64  `json:"size"`
		Purpose     string `json:"purpose"`
		FolderID    string `json:"folderId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	res, err := s.media.Presign(r.Context(), u.Sub, service.PresignInput{
		ContentType: body.ContentType,
		Size:        body.Size,
		Purpose:     body.Purpose,
		FolderID:    body.FolderID,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) complete(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		FileID string `json:"fileId"`
		ETag   string `json:"etag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	res, err := s.media.Complete(r.Context(), u.Sub, body.FileID, body.ETag)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	folderID := r.URL.Query().Get("folderId")
	files, err := s.media.ListFiles(r.Context(), u.Sub, folderID, 50)
	if err != nil {
		writeErr(w, err)
		return
	}
	quota, err := s.media.GetQuota(r.Context(), u.Sub)
	if err != nil {
		writeErr(w, err)
		return
	}
	folders, err := s.media.ListFolders(r.Context(), u.Sub, folderID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files, "folders": folders, "quota": quota})
}

func (s *Server) getQuota(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	quota, err := s.media.GetQuota(r.Context(), u.Sub)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, quota)
}

func (s *Server) createFolder(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Name     string `json:"name"`
		ParentID string `json:"parentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	res, err := s.media.CreateFolder(r.Context(), u.Sub, body.Name, body.ParentID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) listFolders(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	folders, err := s.media.ListFolders(r.Context(), u.Sub, r.URL.Query().Get("parentId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"folders": folders})
}

func (s *Server) getFolder(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/folders/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	res, err := s.media.GetFolder(r.Context(), u.Sub, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) getFile(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/files/")
	res, err := s.media.GetFile(r.Context(), u.Sub, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/files/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	if err := s.media.DeleteFile(r.Context(), u.Sub, id); err != nil {
		writeErr(w, err)
		return
	}
	// 204 は Next の server action が空ボディを JSON として読んで落ちることがある。
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) createShare(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		FileID           string `json:"fileId"`
		ExpiresInSeconds int64  `json:"expiresInSeconds"`
		Password         string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ttl := time.Duration(body.ExpiresInSeconds) * time.Second
	res, err := s.media.CreateShareLink(r.Context(), u.Sub, body.FileID, ttl, body.Password)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	res, err := s.media.GetJob(r.Context(), u.Sub, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) retryJob(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/jobs/"), "/retry")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	res, err := s.media.RetryJob(r.Context(), u.Sub, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func sharePassword(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Share-Password"))
}

func (s *Server) getShare(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	res, err := s.media.ResolveShare(r.Context(), token, sharePassword(r))
	if err != nil {
		writeShareErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) downloadShare(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	url, err := s.media.ShareDownloadURL(r.Context(), token, r.URL.Query().Get("variant"), sharePassword(r))
	if err != nil {
		writeShareErr(w, err)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Server) finishJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	var body struct {
		Variants map[string]struct {
			Key         string `json:"key"`
			ContentType string `json:"contentType"`
		} `json:"variants"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	variants := domain.Variants{}
	for name, v := range body.Variants {
		variants[name] = domain.Variant{Key: v.Key, ContentType: v.ContentType}
	}
	if err := s.media.FinishJob(r.Context(), jobID, variants, body.Error); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrForbidden):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, domain.ErrQuota):
		http.Error(w, "quota exceeded", http.StatusForbidden)
	case errors.Is(err, domain.ErrInvalid):
		http.Error(w, "invalid", http.StatusBadRequest)
	case errors.Is(err, domain.ErrTooLarge):
		http.Error(w, "too large", http.StatusRequestEntityTooLarge)
	case errors.Is(err, domain.ErrConflict):
		http.Error(w, "conflict", http.StatusConflict)
	case errors.Is(err, domain.ErrExpired):
		http.Error(w, "expired", http.StatusGone)
	case errors.Is(err, domain.ErrPasswordRequired):
		http.Error(w, "password required", http.StatusUnauthorized)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func writeShareErr(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrPasswordRequired) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"passwordRequired": true})
		return
	}
	writeErr(w, err)
}
