package wechat

import (
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultCDNBaseURL 是微信 iLink 媒体 CDN 的默认地址。
	DefaultCDNBaseURL = "https://novac2c.cdn.weixin.qq.com/c2c"

	// maxMediaSize 单张图片的解密后字节上限，与附件存储的限制保持一致。
	maxMediaSize int64 = 10 << 20

	mediaTimeout = 30 * time.Second
)

// ErrEmptyMedia 表示图片条目里没有任何可用的下载地址。
var ErrEmptyMedia = fmt.Errorf("wechat: image item has no download url")

// downloadURL 根据 media 构造 CDN 下载地址，并返回内容是否需要解密。
func downloadURL(cdnBaseURL string, media *CDNMedia, plainURL string) (string, bool, error) {
	plain := strings.TrimSpace(plainURL)
	if media == nil {
		if plain == "" {
			return "", false, ErrEmptyMedia
		}
		return plain, false, nil
	}

	encrypted := strings.TrimSpace(media.AESKey) != ""

	if full := strings.TrimSpace(media.FullURL); full != "" {
		return full, encrypted, nil
	}

	if param := strings.TrimSpace(media.EncryptQueryParam); param != "" {
		base := cdnBaseURL
		if strings.TrimSpace(base) == "" {
			base = DefaultCDNBaseURL
		}
		u := strings.TrimRight(base, "/") + "/download?encrypted_query_param=" + url.QueryEscape(param)
		return u, encrypted, nil
	}

	if plain != "" {
		return plain, false, nil
	}

	return "", false, ErrEmptyMedia
}

// aesKeyOf 返回图片对应的密钥原文（可能为空，表示不需要解密）。
// 图片密钥优先取 image_item.aeskey，其次取 media.aes_key。
func aesKeyOf(item *ImageItem) string {
	if item == nil {
		return ""
	}
	if k := strings.TrimSpace(item.AESKey); k != "" {
		return k
	}
	if item.Media != nil {
		return strings.TrimSpace(item.Media.AESKey)
	}
	return ""
}

// ParseAESKey 解析微信下发的媒体密钥，兼容三种编码：
//   - 直接 32 位 hex 字符串（image_item.aeskey）
//   - base64(16 字节原始密钥)
//   - base64(32 位 hex 字符串)（file/voice/video 的 aes_key）
func ParseAESKey(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("wechat: empty aes key")
	}

	// 1) 直接 hex
	if len(s) == 32 {
		if key, err := hex.DecodeString(s); err == nil {
			return key, nil
		}
	}

	// 2) base64 各种变体
	encodings := []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	}
	for _, enc := range encodings {
		decoded, err := enc.DecodeString(s)
		if err != nil {
			continue
		}
		switch len(decoded) {
		case aes.BlockSize:
			return decoded, nil
		case 32: // base64 包裹的 hex 字符串
			if key, err := hex.DecodeString(string(decoded)); err == nil {
				return key, nil
			}
		}
	}

	return nil, fmt.Errorf("wechat: unsupported aes key format")
}

// DecryptMedia 用 AES-128-ECB 解密 CDN 密文，并去掉 PKCS7 填充。
func DecryptMedia(ciphertext, key []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("wechat: invalid aes key length %d", len(key))
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("wechat: invalid ciphertext length %d", len(ciphertext))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("wechat: new cipher: %w", err)
	}

	plain := make([]byte, len(ciphertext))
	for offset := 0; offset < len(ciphertext); offset += aes.BlockSize {
		block.Decrypt(plain[offset:offset+aes.BlockSize], ciphertext[offset:offset+aes.BlockSize])
	}
	return pkcs7Unpad(plain, aes.BlockSize)
}

// pkcs7Unpad 校验并移除 PKCS7 填充。
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("wechat: empty plaintext")
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > blockSize || pad > len(data) {
		return nil, fmt.Errorf("wechat: invalid pkcs7 padding")
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("wechat: invalid pkcs7 padding")
		}
	}
	return data[:len(data)-pad], nil
}

// DownloadImage 下载并解密一条图片消息，返回原始图片字节。
func (c *Client) DownloadImage(ctx context.Context, cdnBaseURL string, item *ImageItem) ([]byte, error) {
	if item == nil {
		return nil, ErrEmptyMedia
	}

	target, encrypted, err := downloadURL(cdnBaseURL, item.Media, item.URL)
	if err != nil {
		return nil, err
	}
	// 密钥也可能只出现在 image_item.aeskey 上
	if aesKeyOf(item) != "" {
		encrypted = true
	}

	ctx, cancel := context.WithTimeout(ctx, mediaTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("wechat: media request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wechat: media fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("wechat: media fetch %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaSize+aes.BlockSize+1))
	if err != nil {
		return nil, fmt.Errorf("wechat: media read: %w", err)
	}
	if int64(len(raw)) > maxMediaSize+aes.BlockSize {
		return nil, fmt.Errorf("wechat: media exceeds %d bytes", maxMediaSize)
	}

	if !encrypted {
		return raw, nil
	}

	key, err := ParseAESKey(aesKeyOf(item))
	if err != nil {
		return nil, err
	}
	plain, err := DecryptMedia(raw, key)
	if err != nil {
		return nil, err
	}
	if int64(len(plain)) > maxMediaSize {
		return nil, fmt.Errorf("wechat: media exceeds %d bytes", maxMediaSize)
	}
	return plain, nil
}
