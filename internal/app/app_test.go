package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/libi/tfo/internal/channel"
	"github.com/libi/tfo/internal/config"
	"github.com/libi/tfo/internal/note"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	svc := note.NewService(note.NewFileStore(dir))
	return &App{config: &config.Config{DataDir: dir}, noteService: svc}
}

func TestSaveChannelImage(t *testing.T) {
	a := newTestApp(t)

	markdown, err := a.saveChannelImage([]byte("\x89PNG\r\n\x1a\nchannel-image"))
	if err != nil {
		t.Fatalf("saveChannelImage: %v", err)
	}
	if !strings.HasPrefix(markdown, "![](") {
		t.Fatalf("markdown = %q, want image syntax", markdown)
	}

	// 附件应落在 attachments/<YYYY-MM>/ 下
	matches, err := filepath.Glob(filepath.Join(a.config.DataDir, "attachments", "*", "*.png"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 stored attachment, got %v", matches)
	}
	if _, err := os.Stat(matches[0]); err != nil {
		t.Fatalf("attachment file missing: %v", err)
	}
}

func TestSaveChannelImage_RejectsNonImage(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.saveChannelImage([]byte("not an image")); err == nil {
		t.Fatal("expected error for non-image bytes")
	}
}

func TestNewReceiver_SavesImageIntoSameDayNote(t *testing.T) {
	a := newTestApp(t)
	r := a.newReceiver()

	// 通过适配器接口把消息喂给 Receiver，验证图片确实落盘并写进笔记
	adapter := &stubAdapter{name: "wechat"}
	r.RegisterAdapter(adapter)

	msg := &channel.IncomingMessage{
		ChannelType: "wechat",
		SenderID:    "user1",
		Content:     "微信发来的图",
		Images:      []channel.IncomingImage{{Data: []byte("\x89PNG\r\n\x1a\nchannel-image")}},
		Timestamp:   time.Now(),
	}
	if err := adapter.handler(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}

	notes, err := a.noteService.ListByDate(context.Background(), time.Now().Format("2006-01-02"))
	if err != nil {
		t.Fatalf("ListByDate: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if len(notes[0].Images) != 1 {
		t.Fatalf("summary images = %v, want 1", notes[0].Images)
	}
	if !strings.Contains(notes[0].Images[0], "attachments/") {
		t.Fatalf("image ref = %q, want attachment path", notes[0].Images[0])
	}
}

// stubAdapter 只实现 Receiver 用到的能力，handler 由 RegisterAdapter 注入。
type stubAdapter struct {
	name    string
	handler channel.MessageHandler
}

func (s *stubAdapter) Name() string                                { return s.name }
func (s *stubAdapter) Start(context.Context) error                 { return nil }
func (s *stubAdapter) Stop() error                                 { return nil }
func (s *stubAdapter) State() channel.ConnState                    { return channel.ConnStateConnected }
func (s *stubAdapter) SetMessageHandler(h channel.MessageHandler)  { s.handler = h }
func (s *stubAdapter) Reply(context.Context, string, string) error { return nil }
