package mdutil

import (
	"strings"
	"testing"
)

func TestExtractImageRefs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"无图片", "纯文本内容", nil},
		{"单张图片", "看图\n![](../attachments/2026-10/abc.png)", []string{"../attachments/2026-10/abc.png"}},
		{"多张图片", "![](a.png)\n文字\n![](b.jpg)", []string{"a.png", "b.jpg"}},
		{"带标题", `![截图](../attachments/2026-10/x.png "标题")`, []string{"../attachments/2026-10/x.png"}},
		{"尖括号包裹", "![x](<../attachments/2026-10/y.png>)", []string{"../attachments/2026-10/y.png"}},
		{"网络图片", "![](https://example.com/a.png)", []string{"https://example.com/a.png"}},
		{"普通链接不算图片", "[文档](https://example.com)", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractImageRefs(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractImageRefs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ref[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestStripImages(t *testing.T) {
	got := StripImages("前文 ![](../attachments/2026-10/a.png) 后文")
	if strings.Contains(got, "![") {
		t.Errorf("StripImages() = %q, still contains markdown image", got)
	}
	if !strings.Contains(got, "前文") || !strings.Contains(got, "后文") {
		t.Errorf("StripImages() = %q, dropped surrounding text", got)
	}
}

func TestExtractTitleSkipsImageLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"首行是图片", "![](../attachments/2026-10/a.png)\n今天的会议纪要", "今天的会议纪要"},
		{"全是图片", "![](a.png)\n![](b.png)", "无标题"},
		{"图片与文字同行", "![](a.png) 好点子", "好点子"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractTitle(tt.content); got != tt.want {
				t.Errorf("ExtractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
