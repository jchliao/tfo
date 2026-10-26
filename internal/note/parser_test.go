package note

import (
	"strings"
	"testing"
)

func TestParserParse(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name      string
		content   string
		wantTitle string
		wantTags  []string
	}{
		{
			"普通笔记",
			"今天想到了产品设计灵感 #产品 #灵感\n\n具体内容...",
			"今天想到了产品设计灵感",
			[]string{"产品", "灵感"},
		},
		{
			"仅标签",
			"#Go #Rust",
			"无标题",
			[]string{"Go", "Rust"},
		},
		{
			"空内容",
			"",
			"无标题",
			nil,
		},
		{
			"长文本",
			strings.Repeat("测试", 60),
			strings.Repeat("测试", 15) + "…",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(tt.content)
			if result.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", result.Title, tt.wantTitle)
			}
			if !sliceEqual(result.Tags, tt.wantTags) {
				t.Errorf("Tags = %v, want %v", result.Tags, tt.wantTags)
			}
		})
	}
}

func TestParserExtractsImages(t *testing.T) {
	p := NewParser()

	content := "界面草图\n\n![](../attachments/2026-10/1f2e3d4c5b6a7890.png)\n\n再补充一句 #设计"
	result := p.Parse(content)

	want := []string{"../attachments/2026-10/1f2e3d4c5b6a7890.png"}
	if !sliceEqual(result.Images, want) {
		t.Errorf("Images = %v, want %v", result.Images, want)
	}
	if result.Title != "界面草图" {
		t.Errorf("Title = %q, want 界面草图", result.Title)
	}
	if strings.Contains(result.Preview, "![]") {
		t.Errorf("Preview 不应包含图片语法: %q", result.Preview)
	}
	if !strings.Contains(result.Preview, "界面草图") {
		t.Errorf("Preview = %q, want it to contain text", result.Preview)
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
