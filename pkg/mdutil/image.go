package mdutil

import "regexp"

// imageRefRegexp 匹配 Markdown 图片语法：![alt](src) / ![alt](<src>) / ![alt](src "title")
// 仅提取 src，alt 与可选的 title 会被忽略。
var imageRefRegexp = regexp.MustCompile(`!\[[^\]]*\]\(\s*<?([^)\s>]+)>?(?:\s+(?:"[^"]*"|'[^']*'))?\s*\)`)

// ExtractImageRefs 按出现顺序提取 Markdown 正文中的图片地址（允许重复）。
// 返回的地址保持原文写法，可能是相对路径、绝对路径或 http(s) 链接。
func ExtractImageRefs(content string) []string {
	matches := imageRefRegexp.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	refs := make([]string, 0, len(matches))
	for _, m := range matches {
		src := m[1]
		if src == "" {
			continue
		}
		refs = append(refs, src)
	}
	if len(refs) == 0 {
		return nil
	}
	return refs
}

// StripImages 移除 Markdown 中的图片语法（替换为空格），用于生成纯文本摘要。
func StripImages(content string) string {
	return imageRefRegexp.ReplaceAllString(content, " ")
}
