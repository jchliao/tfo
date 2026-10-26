package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/libi/tfo/internal/attachment"
)

func newTestRouter(t *testing.T) (*gin.Engine, *attachment.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := attachment.NewStore(t.TempDir())
	engine := New(&Dependencies{Attachments: store})
	return engine, store
}

func pngPayload() []byte {
	return append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x01}, 24)...)
}

func TestUploadAndFetchAttachment(t *testing.T) {
	engine, _ := newTestRouter(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "shot.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(pngPayload()); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var result attachment.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasPrefix(result.Markdown, "![](") {
		t.Errorf("markdown = %q", result.Markdown)
	}

	// 上传后应能通过返回的 URL 读回图片
	req = httptest.NewRequest(http.MethodGet, result.URL, nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("fetch status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q, want image/png", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), pngPayload()) {
		t.Error("fetched bytes differ from upload")
	}
}

func TestUploadRejectsNonImage(t *testing.T) {
	engine, _ := newTestRouter(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "note.txt")
	_, _ = part.Write([]byte("plain text"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415, body = %s", rec.Code, rec.Body.String())
	}
}

func TestUploadWithoutMultipartField(t *testing.T) {
	engine, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/attachments", strings.NewReader("hello"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestGetAttachmentRejectsTraversal(t *testing.T) {
	engine, _ := newTestRouter(t)

	paths := []string{
		"/api/attachments/2026-10/..%2f..%2f.config.json",
		"/api/attachments/..%2f..%2f/2026-10/x.png",
		"/api/attachments/2026-10/missing.png",
	}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", p, rec.Code)
		}
	}
}

func TestAttachmentEndpointsWithoutStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := New(&Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/api/attachments/2026-10/abcdef0123456789.png", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
