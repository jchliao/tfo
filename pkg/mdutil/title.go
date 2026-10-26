package mdutil

import (
	"strings"
	"unicode/utf8"
)

const (
	// MaxTitleLen 标题最大字符数（rune 级别）
	MaxTitleLen = 30
)

// ExtractTitle 从 Markdown 内容中提取标题。
// 提取时跳过空行、纯标签行与纯图片行，取第一行有效文本。
func ExtractTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// 去掉图片与标签标记后再判断该行是否有实际文字
		cleaned := imageRefRegexp.ReplaceAllString(trimmed, " ")
		cleaned = tagRegexp.ReplaceAllString(cleaned, "")
		cleaned = strings.TrimSpace(cleaned)
		if cleaned == "" {
			continue
		}

		return truncate(cleaned, MaxTitleLen)
	}
	return "无标题"
}

func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "…"
}
