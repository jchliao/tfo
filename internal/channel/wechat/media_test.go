package wechat

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

var testPNG = []byte("\x89PNG\r\n\x1a\n" + "fake-png-payload")

// pkcs7Pad 与微信 CDN 的加密方式保持一致（AES-128-ECB + PKCS7）。
func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	return append(append([]byte{}, data...), bytes.Repeat([]byte{byte(pad)}, pad)...)
}

// encryptECB 仅用于测试，模拟微信 CDN 的密文。
func encryptECB(t *testing.T, plain, key []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	padded := pkcs7Pad(plain, aes.BlockSize)
	out := make([]byte, len(padded))
	for i := 0; i < len(padded); i += aes.BlockSize {
		block.Encrypt(out[i:i+aes.BlockSize], padded[i:i+aes.BlockSize])
	}
	return out
}

func TestParseAESKey_Formats(t *testing.T) {
	key := []byte("0123456789abcdef")
	hexKey := hex.EncodeToString(key)

	tests := []struct {
		name string
		raw  string
	}{
		{"raw hex", hexKey},
		{"base64 raw 16 bytes", base64.StdEncoding.EncodeToString(key)},
		{"base64 of hex string", base64.StdEncoding.EncodeToString([]byte(hexKey))},
		{"raw base64 without padding", base64.RawStdEncoding.EncodeToString(key)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAESKey(tt.raw)
			if err != nil {
				t.Fatalf("ParseAESKey(%q): %v", tt.raw, err)
			}
			if !bytes.Equal(got, key) {
				t.Fatalf("key = %x, want %x", got, key)
			}
		})
	}
}

func TestParseAESKey_Invalid(t *testing.T) {
	for _, raw := range []string{"", "not-a-key", "313233"} {
		if _, err := ParseAESKey(raw); err == nil {
			t.Fatalf("ParseAESKey(%q) should fail", raw)
		}
	}
}

func TestDecryptMedia_RoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	ciphertext := encryptECB(t, testPNG, key)

	got, err := DecryptMedia(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptMedia: %v", err)
	}
	if !bytes.Equal(got, testPNG) {
		t.Fatalf("plaintext = %q, want %q", got, testPNG)
	}
}

func TestDecryptMedia_InvalidInput(t *testing.T) {
	key := []byte("0123456789abcdef")
	if _, err := DecryptMedia([]byte("short"), key); err == nil {
		t.Fatal("expected error for non block aligned ciphertext")
	}
	if _, err := DecryptMedia([]byte("0123456789abcdef"), []byte("bad")); err == nil {
		t.Fatal("expected error for bad key length")
	}
}

func TestDownloadURL(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		media     *CDNMedia
		plainURL  string
		want      string
		wantCrypt bool
		wantErr   bool
	}{
		{
			name:      "encrypted query param",
			base:      "https://cdn.example.com/c2c",
			media:     &CDNMedia{EncryptQueryParam: "a b&c", AESKey: "key"},
			want:      "https://cdn.example.com/c2c/download?encrypted_query_param=a+b%26c",
			wantCrypt: true,
		},
		{
			name:      "full url wins",
			base:      "https://cdn.example.com/c2c",
			media:     &CDNMedia{FullURL: "https://cdn.example.com/x", EncryptQueryParam: "p", AESKey: "key"},
			want:      "https://cdn.example.com/x",
			wantCrypt: true,
		},
		{
			name:     "plain url without media",
			plainURL: "https://example.com/a.png",
			want:     "https://example.com/a.png",
		},
		{
			name:    "nothing usable",
			base:    "https://cdn.example.com/c2c",
			media:   &CDNMedia{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, encrypted, err := downloadURL(tt.base, tt.media, tt.plainURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("downloadURL: %v", err)
			}
			if got != tt.want {
				t.Fatalf("url = %q, want %q", got, tt.want)
			}
			if encrypted != tt.wantCrypt {
				t.Fatalf("encrypted = %v, want %v", encrypted, tt.wantCrypt)
			}
		})
	}
}

func TestClient_DownloadImage_Encrypted(t *testing.T) {
	key := []byte("0123456789abcdef")
	ciphertext := encryptECB(t, testPNG, key)

	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/download" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotQuery = r.URL.Query().Get("encrypted_query_param")
		_, _ = w.Write(ciphertext)
	}))
	defer server.Close()

	item := &ImageItem{
		Media: &CDNMedia{
			EncryptQueryParam: "param-123",
			AESKey:            base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(key))),
		},
	}

	got, err := NewClient().DownloadImage(context.Background(), server.URL, item)
	if err != nil {
		t.Fatalf("DownloadImage: %v", err)
	}
	if gotQuery != "param-123" {
		t.Fatalf("query = %q, want param-123", gotQuery)
	}
	if !bytes.Equal(got, testPNG) {
		t.Fatalf("image = %q, want %q", got, testPNG)
	}
}

func TestClient_DownloadImage_PlainURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testPNG)
	}))
	defer server.Close()

	item := &ImageItem{URL: server.URL + "/plain.png"}
	got, err := NewClient().DownloadImage(context.Background(), server.URL, item)
	if err != nil {
		t.Fatalf("DownloadImage: %v", err)
	}
	if !bytes.Equal(got, testPNG) {
		t.Fatalf("image = %q, want %q", got, testPNG)
	}
}

func TestClient_DownloadImage_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("gone"))
	}))
	defer server.Close()

	item := &ImageItem{URL: server.URL + "/missing.png"}
	if _, err := NewClient().DownloadImage(context.Background(), server.URL, item); err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestImageItem_Unmarshal(t *testing.T) {
	raw := `{"type":2,"image_item":{"media":{"encrypt_query_param":"abc","aes_key":"MDAxMTIyMzM0NDU1NjY3Nzg4OTlhYWJiY2NkZGVlZmY="},"aeskey":"00112233445566778899aabbccddeeff","mid_size":1024}}`
	var item MessageItem
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if item.Type != ItemTypeImage {
		t.Fatalf("type = %d, want %d", item.Type, ItemTypeImage)
	}
	if item.ImageItem == nil || item.ImageItem.Media == nil {
		t.Fatal("image_item/media should be parsed")
	}
	if item.ImageItem.Media.EncryptQueryParam != "abc" {
		t.Fatalf("encrypt_query_param = %q", item.ImageItem.Media.EncryptQueryParam)
	}
	if item.ImageItem.AESKey != "00112233445566778899aabbccddeeff" {
		t.Fatalf("aeskey = %q", item.ImageItem.AESKey)
	}
	if item.ImageItem.MidSize != 1024 {
		t.Fatalf("mid_size = %d", item.ImageItem.MidSize)
	}
}

func TestClient_DownloadImage_AESKeyOnItem(t *testing.T) {
	key := []byte("0123456789abcdef")
	ciphertext := encryptECB(t, testPNG, key)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ciphertext)
	}))
	defer server.Close()

	// media 只带下载参数，密钥放在 image_item.aeskey（32 位 hex）
	item := &ImageItem{
		Media:  &CDNMedia{EncryptQueryParam: "p"},
		AESKey: hex.EncodeToString(key),
	}

	got, err := NewClient().DownloadImage(context.Background(), server.URL, item)
	if err != nil {
		t.Fatalf("DownloadImage: %v", err)
	}
	if !bytes.Equal(got, testPNG) {
		t.Fatalf("image = %q, want %q", got, testPNG)
	}
}
