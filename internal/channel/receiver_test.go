package channel

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockNoteCreator 模拟笔记创建器
type mockNoteCreator struct {
	mu    sync.Mutex
	notes []string
}

func (m *mockNoteCreator) Create(ctx context.Context, content string) (interface{}, error) {
	m.mu.Lock()
	m.notes = append(m.notes, content)
	m.mu.Unlock()
	return nil, nil
}

// mockAdapter 模拟适配器
type mockAdapter struct {
	mu      sync.Mutex
	name    string
	handler MessageHandler
	state   ConnState
	replies []string
}

func (m *mockAdapter) Name() string                       { return m.name }
func (m *mockAdapter) Start(ctx context.Context) error    { m.state = ConnStateConnected; return nil }
func (m *mockAdapter) Stop() error                        { m.state = ConnStateDisconnected; return nil }
func (m *mockAdapter) State() ConnState                   { return m.state }
func (m *mockAdapter) SetMessageHandler(h MessageHandler) { m.handler = h }

func (m *mockAdapter) Reply(ctx context.Context, replyToken string, text string) error {
	m.mu.Lock()
	m.replies = append(m.replies, text)
	m.mu.Unlock()
	return nil
}

func (m *mockAdapter) SimulateMessage(ctx context.Context, content string) error {
	if m.handler == nil {
		return nil
	}
	return m.handler(ctx, &IncomingMessage{
		ChannelType: m.name,
		SenderID:    "test-user",
		Content:     content,
		Type:        MessageTypeText,
		Timestamp:   time.Now(),
		ReplyToken:  "test-reply-token",
	})
}

func TestReceiver_HandleMessage(t *testing.T) {
	nc := &mockNoteCreator{}
	r := NewReceiver(nc)

	adapter := &mockAdapter{name: "test"}
	r.RegisterAdapter(adapter)

	if err := r.StartAll(context.Background()); err != nil {
		t.Fatalf("StartAll: %v", err)
	}

	// 模拟收到消息
	if err := adapter.SimulateMessage(context.Background(), "测试消息内容"); err != nil {
		t.Fatalf("SimulateMessage: %v", err)
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	if len(nc.notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(nc.notes))
	}

	if !contains(nc.notes[0], "测试消息内容") {
		t.Fatalf("note should contain message content, got: %s", nc.notes[0])
	}
	if !contains(nc.notes[0], "#test收集") {
		t.Fatalf("note should contain channel tag, got: %s", nc.notes[0])
	}
}

func TestReceiver_EmptyMessage(t *testing.T) {
	nc := &mockNoteCreator{}
	r := NewReceiver(nc)

	adapter := &mockAdapter{name: "test"}
	r.RegisterAdapter(adapter)
	r.StartAll(context.Background())

	// 空消息不应创建笔记
	adapter.SimulateMessage(context.Background(), "")

	nc.mu.Lock()
	defer nc.mu.Unlock()
	if len(nc.notes) != 0 {
		t.Fatalf("expected 0 notes for empty message, got %d", len(nc.notes))
	}
}

func TestReceiver_AdapterStates(t *testing.T) {
	nc := &mockNoteCreator{}
	r := NewReceiver(nc)

	a1 := &mockAdapter{name: "wechat"}
	a2 := &mockAdapter{name: "feishu"}
	r.RegisterAdapter(a1)
	r.RegisterAdapter(a2)

	r.StartAll(context.Background())
	states := r.GetAdapterStates()

	if states["wechat"] != ConnStateConnected {
		t.Fatalf("expected wechat connected, got %v", states["wechat"])
	}
	if states["feishu"] != ConnStateConnected {
		t.Fatalf("expected feishu connected, got %v", states["feishu"])
	}

	r.StopAll()
	states = r.GetAdapterStates()
	if states["wechat"] != ConnStateDisconnected {
		t.Fatalf("expected wechat disconnected after stop, got %v", states["wechat"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestReceiver_ReplyAfterCreate(t *testing.T) {
	nc := &mockNoteCreator{}
	r := NewReceiver(nc)

	adapter := &mockAdapter{name: "test"}
	r.RegisterAdapter(adapter)
	r.StartAll(context.Background())

	adapter.SimulateMessage(context.Background(), "需要回复的消息")

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if len(adapter.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(adapter.replies))
	}
	if adapter.replies[0] != ReceiptText {
		t.Fatalf("reply = %q, want %q", adapter.replies[0], ReceiptText)
	}
}

func TestReceiver_NoReplyWhenDisabled(t *testing.T) {
	nc := &mockNoteCreator{}
	r := NewReceiver(nc)
	r.SetEnableReply(false)

	adapter := &mockAdapter{name: "test"}
	r.RegisterAdapter(adapter)
	r.StartAll(context.Background())

	adapter.SimulateMessage(context.Background(), "不需要回复")

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if len(adapter.replies) != 0 {
		t.Fatalf("expected 0 replies when disabled, got %d", len(adapter.replies))
	}
}

func TestReceiver_ImageMessage(t *testing.T) {
	nc := &mockNoteCreator{}
	r := NewReceiver(nc)
	r.SetImageSaver(func(data []byte) (string, error) {
		if string(data) != "image-bytes" {
			t.Errorf("saver got %q", data)
		}
		return "![](../attachments/2026-10/0123456789abcdef.png)", nil
	})

	adapter := &mockAdapter{name: "wechat"}
	r.RegisterAdapter(adapter)
	r.StartAll(context.Background())

	if err := adapter.handler(context.Background(), &IncomingMessage{
		ChannelType: "wechat",
		SenderID:    "user1",
		Content:     "看这张图",
		Type:        MessageTypeImage,
		Images:      []IncomingImage{{Data: []byte("image-bytes")}},
		Timestamp:   time.Now(),
		ReplyToken:  "token",
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()
	if len(nc.notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(nc.notes))
	}
	note := nc.notes[0]
	if !contains(note, "看这张图") {
		t.Fatalf("note should keep text, got: %s", note)
	}
	if !contains(note, "![](../attachments/2026-10/0123456789abcdef.png)") {
		t.Fatalf("note should contain image markdown, got: %s", note)
	}
	if !contains(note, "#wechat") {
		t.Fatalf("note should contain channel tag, got: %s", note)
	}

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if len(adapter.replies) != 1 || !contains(adapter.replies[0], "1 张图片") {
		t.Fatalf("reply = %v, want receipt mentioning the image", adapter.replies)
	}
}

func TestReceiver_ImageOnlyWithoutSaver(t *testing.T) {
	nc := &mockNoteCreator{}
	r := NewReceiver(nc) // 未注入 ImageSaver

	adapter := &mockAdapter{name: "wechat"}
	r.RegisterAdapter(adapter)
	r.StartAll(context.Background())

	if err := adapter.handler(context.Background(), &IncomingMessage{
		ChannelType: "wechat",
		Images:      []IncomingImage{{Data: []byte("image-bytes")}},
		ReplyToken:  "token",
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()
	if len(nc.notes) != 0 {
		t.Fatalf("expected no note when image cannot be stored, got %d", len(nc.notes))
	}
}

func TestReceiver_SaveImageErrorKeepsText(t *testing.T) {
	nc := &mockNoteCreator{}
	r := NewReceiver(nc)
	r.SetImageSaver(func([]byte) (string, error) {
		return "", context.DeadlineExceeded
	})

	adapter := &mockAdapter{name: "wechat"}
	r.RegisterAdapter(adapter)
	r.StartAll(context.Background())

	adapter.handler(context.Background(), &IncomingMessage{
		ChannelType: "wechat",
		Content:     "图片没存上但文字要留下",
		Images:      []IncomingImage{{Data: []byte("image-bytes")}},
	})

	nc.mu.Lock()
	defer nc.mu.Unlock()
	if len(nc.notes) != 1 || !contains(nc.notes[0], "图片没存上但文字要留下") {
		t.Fatalf("text should still be saved, got: %v", nc.notes)
	}
	if contains(nc.notes[0], "![](") {
		t.Fatalf("no image markdown expected, got: %s", nc.notes[0])
	}
}
