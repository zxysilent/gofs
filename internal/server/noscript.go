package server

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/zxysilent/gofs/internal/urlutil"
)

// detectNoScript reports whether the user agent is a text-mode client.
func detectNoScript(userAgent string) bool {
	for _, prefix := range []string{"lynx/", "w3m/", "links ", "elinks/", "curl/", "wget/", "httpie/", "aria2/"} {
		if strings.HasPrefix(userAgent, prefix) {
			return true
		}
	}
	return false
}

// generateNoScriptHTML renders a plain HTML directory listing for text clients.
func generateNoScriptHTML(data *IndexData) string {
	title := "Index of " + html.EscapeString(data.Href)
	var b strings.Builder
	b.WriteString("<html>\n<head>\n")
	b.WriteString("<title>")
	b.WriteString(title)
	b.WriteString("</title>\n")
	b.WriteString(`<style>
  td {
    padding: 0.2rem;
    text-align: left;
  }
  td:nth-child(3) {
    text-align: right;
  }
</style>
`)
	b.WriteString("</head>\n<body>\n")
	b.WriteString("<h1>")
	b.WriteString(title)
	b.WriteString("</h1>\n")
	b.WriteString("<table>\n  <tbody>\n")
	b.WriteString("    ")
	b.WriteString(renderNoScriptParent())
	b.WriteString("\n")
	for _, path := range data.Paths {
		b.WriteString("    ")
		b.WriteString(renderNoScriptPathItem(&path))
		b.WriteString("\n")
	}
	b.WriteString("  </tbody>\n</table>\n</body>\n")
	return b.String()
}

func renderNoScriptParent() string {
	const value = "../"
	return fmt.Sprintf(`<tr><td><a href="%s?noscript">%s</a></td><td></td><td></td></tr>`, value, value)
}

func renderNoScriptPathItem(p *PathItem) string {
	href := urlutil.EncodeURI(p.Name)
	name := html.EscapeString(p.Name)
	if p.isDir() {
		href += "/?noscript"
		name += "/"
	}
	mtime := formatNoScriptMtime(p.Mtime)
	size := formatNoScriptSize(p.Size, p.PathType)
	return fmt.Sprintf(`<tr><td><a href="%s">%s</a></td><td>%s</td><td>%s</td></tr>`, href, name, mtime, size)
}

// 与网页界面保持同一种时间格式（YYYY-MM-DD HH:mm:ss），文本浏览器里也便于
// 直接复制比对；原来的 ISO 8601 带毫秒和 T/Z，和界面列对不上。
func formatNoScriptMtime(mtime uint64) string {
	return time.UnixMilli(int64(mtime)).Local().Format("2006-01-02 15:04:05")
}

func formatNoScriptSize(size uint64, pathType PathType) string {
	if pathType.isDir() {
		unit := "items"
		if size == 1 {
			unit = "item"
		}
		num := fmt.Sprintf("%d", size)
		if size >= maxSubpathsCount {
			num = fmt.Sprintf(">%d", maxSubpathsCount-1)
		}
		return num + " " + unit
	}
	if size == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	// i = floor(log2(size) / 10)
	i := 0
	for v := size; v >= 1024; v >>= 10 {
		i++
	}
	if i >= len(units) {
		return fmt.Sprintf("%.2f PB", float64(size)/pow1024(5))
	}
	return fmt.Sprintf("%.2f %s", float64(size)/pow1024(i), units[i])
}

func pow1024(n int) float64 {
	r := 1.0
	for i := 0; i < n; i++ {
		r *= 1024
	}
	return r
}
