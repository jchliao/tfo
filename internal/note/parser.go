package note

import (
	"strings"
	"unicode/utf8"

	"github.com/libi/tfo/pkg/mdutil"
)

const maxPreviewLen = 100

// Parser 负责从 Markdown 内容中提取结构化信息
type Parser struct{}

// NewParser 创建解析器实例
func NewParser() *Parser {
	return &Parser{}
}

// ParseResult 解析结果
type ParseResult struct {
	Title   string
	Tags    []string
	Preview string
	// Images 是正文中引用的图片地址（按出现顺序），保持 Markdown 原文写法。
	Images []string
}

// Parse 解析 Markdown 内容
func (p *Parser) Parse(content string) *ParseResult {
	return &ParseResult{
		Title:   mdutil.ExtractTitle(content),
		Tags:    mdutil.ExtractTags(content),
		Preview: extractPreview(content),
		Images:  mdutil.ExtractImageRefs(content),
	}
}

// extractPreview 生成纯文本摘要，图片语法会被剔除，避免摘要里出现 ![](...)
func extractPreview(content string) string {
	text := mdutil.StripImages(content)
	preview := strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(preview) <= maxPreviewLen {
		return preview
	}
	runes := []rune(preview)
	return string(runes[:maxPreviewLen]) + "…"
}
