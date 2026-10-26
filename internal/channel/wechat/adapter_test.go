package wechat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/libi/tfo/internal/channel"
	"github.com/libi/tfo/internal/config"
)

func TestExtractText(t *testing.T) {
	tests := []struct {
		name string
		msg  WeixinMessage
		want string
	}{
		{
			name: "text message",
			msg: WeixinMessage{
				ItemList: []MessageItem{
					{Type: ItemTypeText, TextItem: &TextItem{Text: "hello world"}},
				},
			},
			want: "hello world",
		},
		{
			name: "voice with text",
			msg: WeixinMessage{
				ItemList: []MessageItem{
					{Type: ItemTypeVoice, VoiceItem: &VoiceItem{Text: "语音转文字"}},
				},
			},
			want: "语音转文字",
		},
		{
			name: "mixed items",
			msg: WeixinMessage{
				ItemList: []MessageItem{
					{Type: ItemTypeText, TextItem: &TextItem{Text: "text part"}},
					{Type: ItemTypeImage}, // 图片忽略
					{Type: ItemTypeVoice, VoiceItem: &VoiceItem{Text: "voice part"}},
				},
			},
			want: "text part\nvoice part",
		},
		{
			name: "empty message",
			msg:  WeixinMessage{},
			want: "",
		},
		{
			name: "image only - no text",
			msg: WeixinMessage{
				ItemList: []MessageItem{
					{Type: ItemTypeImage},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractText(tt.msg)
			if got != tt.want {
				t.Errorf("extractText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetMessageID(t *testing.T) {
	msg := WeixinMessage{MessageID: 12345}
	if id := GetMessageID(msg); id != "12345" {
		t.Errorf("GetMessageID() = %q, want %q", id, "12345")
	}
}

func TestWeChatAdapter_StartWithoutConfig(t *testing.T) {
	a := NewWeChatAdapter(config.WeChatConfig{})
	err := a.Start(nil)
	if err == nil {
		t.Fatal("expected error for empty baseUrl")
	}
}

func TestWeChatAdapter_StartWithoutToken(t *testing.T) {
	a := NewWeChatAdapter(config.WeChatConfig{BaseURL: "http://localhost"})
	err := a.Start(nil)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestWeChatAdapter_StateTransitions(t *testing.T) {
	a := NewWeChatAdapter(config.WeChatConfig{})
	if a.State() != channel.ConnStateDisconnected {
		t.Fatalf("initial state should be disconnected, got %v", a.State())
	}
	if a.Name() != "wechat" {
		t.Fatalf("Name() should be wechat, got %q", a.Name())
	}
}

func TestWeChatAdapter_UpdateConfig(t *testing.T) {
	a := NewWeChatAdapter(config.WeChatConfig{BaseURL: "http://old"})
	a.UpdateConfig(config.WeChatConfig{BaseURL: "http://new", Token: "tok"})

	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.cfg.BaseURL != "http://new" {
		t.Fatalf("expected updated baseUrl, got %q", a.cfg.BaseURL)
	}
}

func TestSleepCtx_Immediate(t *testing.T) {
	// sleepCtx 应该能正常完成一个极短的 sleep
	start := time.Now()
	sleepCtx(context.Background(), 1*time.Millisecond)
	if time.Since(start) > 1*time.Second {
		t.Fatal("sleepCtx took too long")
	}
}

func TestSleepCtx_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	start := time.Now()
	sleepCtx(ctx, 10*time.Second)
	if time.Since(start) > 1*time.Second {
		t.Fatal("sleepCtx should return immediately on cancelled context")
	}
}

func TestBuildParseReplyToken(t *testing.T) {
	msg := WeixinMessage{FromUserID: "user123", ContextToken: "ctx-abc"}
	token := buildReplyToken(msg)
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	uid, ct, ok := parseReplyToken(token)
	if !ok {
		t.Fatal("parseReplyToken failed")
	}
	if uid != "user123" {
		t.Fatalf("userID = %q, want user123", uid)
	}
	if ct != "ctx-abc" {
		t.Fatalf("contextToken = %q, want ctx-abc", ct)
	}
}

func TestBuildReplyToken_NoContextToken(t *testing.T) {
	msg := WeixinMessage{FromUserID: "user1"}
	token := buildReplyToken(msg)
	if token == "" {
		t.Fatal("expected non-empty token even without ContextToken")
	}
	uid, ct, ok := parseReplyToken(token)
	if !ok {
		t.Fatal("parseReplyToken failed")
	}
	if uid != "user1" {
		t.Fatalf("userID = %q, want user1", uid)
	}
	if ct != "" {
		t.Fatalf("contextToken = %q, want empty", ct)
	}
}

func TestBuildReplyToken_Empty(t *testing.T) {
	msg := WeixinMessage{}
	if token := buildReplyToken(msg); token != "" {
		t.Fatalf("expected empty token for missing FromUserID, got %q", token)
	}
}

func TestParseReplyToken_Invalid(t *testing.T) {
	_, _, ok := parseReplyToken("")
	if ok {
		t.Fatal("expected false for empty token")
	}
	_, _, ok = parseReplyToken("no-separator")
	if ok {
		t.Fatal("expected false for token without separator")
	}
}

func TestProcessMessage_ImageOnly(t *testing.T) {
	key := []byte("0123456789abcdef")
	ciphertext := encryptECB(t, testPNG, key)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ciphertext)
	}))
	defer server.Close()

	a := NewWeChatAdapter(config.WeChatConfig{})
	a.cdnBaseURL = server.URL

	var got *channel.IncomingMessage
	a.SetMessageHandler(func(_ context.Context, msg *channel.IncomingMessage) error {
		got = msg
		return nil
	})

	a.processMessage(context.Background(), WeixinMessage{
		MessageID:  1,
		FromUserID: "user1",
		ItemList: []MessageItem{{
			Type: ItemTypeImage,
			ImageItem: &ImageItem{Media: &CDNMedia{
				EncryptQueryParam: "param",
				AESKey:            base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(key))),
			}},
		}},
	})

	if got == nil {
		t.Fatal("handler should be called for an image-only message")
	}
	if got.Type != channel.MessageTypeImage {
		t.Fatalf("type = %v, want image", got.Type)
	}
	if len(got.Images) != 1 || !bytes.Equal(got.Images[0].Data, testPNG) {
		t.Fatalf("images = %+v, want one decrypted image", got.Images)
	}
}

func TestProcessMessage_TextAndImages(t *testing.T) {
	key := []byte("0123456789abcdef")
	ciphertext := encryptECB(t, testPNG, key)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ciphertext)
	}))
	defer server.Close()

	a := NewWeChatAdapter(config.WeChatConfig{})
	a.cdnBaseURL = server.URL

	var got *channel.IncomingMessage
	a.SetMessageHandler(func(_ context.Context, msg *channel.IncomingMessage) error {
		got = msg
		return nil
	})

	aesKey := base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(key)))
	a.processMessage(context.Background(), WeixinMessage{
		MessageID:  2,
		FromUserID: "user1",
		ItemList: []MessageItem{
			{Type: ItemTypeText, TextItem: &TextItem{Text: "看这张图"}},
			{Type: ItemTypeImage, ImageItem: &ImageItem{Media: &CDNMedia{EncryptQueryParam: "p1", AESKey: aesKey}}},
			{Type: ItemTypeImage, ImageItem: &ImageItem{Media: &CDNMedia{EncryptQueryParam: "p2", AESKey: aesKey}}},
		},
	})

	if got == nil {
		t.Fatal("handler should be called")
	}
	if got.Content != "看这张图" {
		t.Fatalf("content = %q", got.Content)
	}
	if len(got.Images) != 2 {
		t.Fatalf("images = %d, want 2", len(got.Images))
	}
}

func TestProcessMessage_ImageDownloadFailureIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := NewWeChatAdapter(config.WeChatConfig{})
	a.cdnBaseURL = server.URL

	var got *channel.IncomingMessage
	a.SetMessageHandler(func(_ context.Context, msg *channel.IncomingMessage) error {
		got = msg
		return nil
	})

	a.processMessage(context.Background(), WeixinMessage{
		MessageID: 3,
		ItemList: []MessageItem{{
			Type:      ItemTypeImage,
			ImageItem: &ImageItem{Media: &CDNMedia{EncryptQueryParam: "p", AESKey: "00112233445566778899aabbccddeeff"}},
		}},
	})

	if got != nil {
		t.Fatalf("handler should not be called when the only image fails, got %+v", got)
	}
}

func TestProcessMessage_ImageWithoutMediaIgnored(t *testing.T) {
	a := NewWeChatAdapter(config.WeChatConfig{})
	called := false
	a.SetMessageHandler(func(_ context.Context, msg *channel.IncomingMessage) error {
		called = true
		return nil
	})

	a.processMessage(context.Background(), WeixinMessage{
		MessageID: 4,
		ItemList:  []MessageItem{{Type: ItemTypeImage, ImageItem: &ImageItem{}}},
	})

	if called {
		t.Fatal("handler should not be called for an image item without download url")
	}
}
