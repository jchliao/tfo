// Package attachment 管理笔记中引用的图片等附件。
//
// 附件以普通文件形式存放在数据目录的 attachments/<YYYY-MM>/ 下，文件名取自内容的
// 哈希（内容寻址，天然去重），笔记正文里用相对路径引用（如
// ../attachments/2026-10/1f2e3d4c5b6a7890.png）。这样：
//   - 整个数据目录复制到别的设备后，Markdown 里的图片依然能打开；
//   - 用 Typora / Obsidian / VS Code 直接打开 .md 文件也能看到图片；
//   - 相同图片重复上传只占一份空间。
package attachment

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// DirName 是附件目录在数据目录下的名称。
	DirName = "attachments"
	// MaxSize 是单个附件的字节上限（10 MiB）。
	MaxSize int64 = 10 << 20
	// HashLength 是文件名中保留的哈希前缀长度（16 个 hex 字符）。
	HashLength = 16

	monthDirFormat = "2006-01"
)

var (
	// ErrTooLarge 表示上传内容超过 MaxSize。
	ErrTooLarge = errors.New("attachment too large")
	// ErrUnsupportedType 表示上传内容不是受支持的图片格式。
	ErrUnsupportedType = errors.New("unsupported attachment type")
	// ErrInvalidPath 表示附件路径不合法（含目录穿越等）。
	ErrInvalidPath = errors.New("invalid attachment path")

	fileNameRegexp = regexp.MustCompile(`^[a-f0-9]{16}\.(png|jpg|gif|webp|bmp)$`)
	monthRegexp    = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)
)

// Store 负责附件的落盘与定位。
type Store struct {
	rootDir string
}

// NewStore 创建以 dataDir 为根目录的附件存储。
func NewStore(dataDir string) *Store {
	return &Store{rootDir: dataDir}
}

// Root 返回数据目录。
func (s *Store) Root() string { return s.rootDir }

// Result 描述一次成功保存的附件。
type Result struct {
	// Path 相对数据目录的路径，形如 attachments/2026-10/1f2e3d4c5b6a7890.png
	Path string `json:"path"`
	// URL 后端提供的访问地址，供前端 <img> 使用。
	URL string `json:"url"`
	// Markdown 可直接插入笔记正文的片段（相对笔记文件的路径）。
	Markdown string `json:"markdown"`
	// Size 文件字节数。
	Size int64 `json:"size"`
	// Mime 嗅探得到的 MIME 类型。
	Mime string `json:"mime"`
}

// Save 把 r 中的图片写入附件目录并返回引用信息。
// 只接受 png / jpeg / gif / webp / bmp，类型由文件头嗅探决定，不信任文件名。
func (s *Store) Save(r io.Reader) (*Result, error) {
	if s.rootDir == "" {
		return nil, fmt.Errorf("attachment: data dir is not configured")
	}

	data, err := io.ReadAll(io.LimitReader(r, MaxSize+1))
	if err != nil {
		return nil, fmt.Errorf("attachment: read upload: %w", err)
	}
	if int64(len(data)) > MaxSize {
		return nil, ErrTooLarge
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("attachment: empty upload")
	}

	ext, mime, ok := detectImage(data)
	if !ok {
		return nil, ErrUnsupportedType
	}

	month := time.Now().Format(monthDirFormat)
	sum := sha1.Sum(data)
	name := hex.EncodeToString(sum[:])[:HashLength] + ext

	dir := filepath.Join(s.rootDir, DirName, month)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("attachment: create dir: %w", err)
	}

	absPath := filepath.Join(dir, name)
	if err := writeFileIfAbsent(absPath, data); err != nil {
		return nil, err
	}

	relPath := path.Join(DirName, month, name)
	return &Result{
		Path: relPath,
		// 前端通过 /api/attachments/<month>/<name> 访问
		URL:      "/api/" + path.Join(DirName, month, name),
		Markdown: "![](" + "../" + relPath + ")",
		Size:     int64(len(data)),
		Mime:     mime,
	}, nil
}

// AbsPath 校验参数并返回附件的绝对路径。
// month 必须形如 2026-10，name 必须形如 1f2e3d4c5b6a7890.png，杜绝目录穿越。
func (s *Store) AbsPath(month, name string) (string, error) {
	if s.rootDir == "" {
		return "", fmt.Errorf("attachment: data dir is not configured")
	}
	if !monthRegexp.MatchString(month) {
		return "", fmt.Errorf("%w: bad month %q", ErrInvalidPath, month)
	}
	if !fileNameRegexp.MatchString(strings.ToLower(name)) {
		return "", fmt.Errorf("%w: bad name %q", ErrInvalidPath, name)
	}

	absPath := filepath.Join(s.rootDir, DirName, month, strings.ToLower(name))
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return "", os.ErrNotExist
	}
	return absPath, nil
}

// ContentType 根据扩展名返回响应用的 Content-Type。
func ContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}

// --- helpers ---

// detectImage 通过文件头判断图片类型。
func detectImage(data []byte) (ext, mime string, ok bool) {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		return ".png", "image/png", true
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return ".jpg", "image/jpeg", true
	case len(data) >= 6 && (bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))):
		return ".gif", "image/gif", true
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return ".webp", "image/webp", true
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return ".bmp", "image/bmp", true
	default:
		return "", "", false
	}
}

// writeFileIfAbsent 仅在文件不存在时写入（内容寻址，已存在即为同一内容）。
func writeFileIfAbsent(absPath string, data []byte) error {
	if _, err := os.Stat(absPath); err == nil {
		return nil
	}

	tmp := absPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("attachment: write file: %w", err)
	}
	if err := os.Rename(tmp, absPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("attachment: write file: %w", err)
	}
	return nil
}
