package server

// Virtual Markdown documents served without touching the filesystem.
//
// Both documents are generated per request so the URLs inside always reflect
// the host the client actually used (reverse proxy / path prefix friendly), and
// they are available in Chinese and English.
//
// Language selection order:
//  1. `?lang=zh|en` query parameter (explicit override)
//  2. `Accept-Language` request header
//  3. Chinese (default)

import (
	"net/http"
	"strconv"
	"strings"
)

// Virtual document paths.
const (
	readmeDocPath = "gofs-readme.md" // human-facing: features, deployment, UI usage
	agentDocPath  = "gofs-agent.md"  // machine-facing: full curl/HTTP API reference
)

// docGenFunc generates a Markdown document for the given base URL and language.
type docGenFunc func(s *Server, base string, lang string) string

// virtualDocs maps a virtual path to its content generator.
// A real file with the same name on disk always takes precedence.
var virtualDocs = map[string]docGenFunc{
	readmeDocPath: genReadmeDoc,
	agentDocPath:  genAgentDoc,
}

// docLang picks the document language for a request.
func docLang(r *http.Request) string {
	if v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("lang"))); v != "" {
		if strings.HasPrefix(v, "en") {
			return "en"
		}
		if strings.HasPrefix(v, "zh") {
			return "zh"
		}
	}
	return langFromAcceptLanguage(r.Header.Get("Accept-Language"))
}

// langFromAcceptLanguage returns "en" when English is preferred over Chinese,
// otherwise "zh". It inspects the ordered list of language tags, so
// "fr,en;q=0.8,zh;q=0.5" resolves to English and "zh-CN,zh;q=0.9,en;q=0.8"
// resolves to Chinese.
func langFromAcceptLanguage(header string) string {
	for _, part := range strings.Split(header, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch {
		case strings.HasPrefix(tag, "zh"):
			return "zh"
		case strings.HasPrefix(tag, "en"):
			return "en"
		}
	}
	return "zh"
}

// mdDoc is a tiny helper for building Markdown documents.
type mdDoc struct{ b strings.Builder }

func (d *mdDoc) line(s string) { d.b.WriteString(s); d.b.WriteByte('\n') }
func (d *mdDoc) blank()        { d.b.WriteByte('\n') }

// h writes a heading, preceded by a blank line so Markdown renders cleanly.
func (d *mdDoc) h(level int, s string) {
	if d.b.Len() > 0 {
		d.blank()
	}
	d.line(strings.Repeat("#", level) + " " + s)
}

func (d *mdDoc) list(items ...string) {
	for _, it := range items {
		d.line("- " + it)
	}
}

func (d *mdDoc) String() string { return d.b.String() }

// yesNo renders a capability flag for the manual in the given language.
func yesNo(ok bool, lang string) string {
	if lang == "en" {
		if ok {
			return "allowed"
		}
		return "denied"
	}
	if ok {
		return "允许"
	}
	return "不允许"
}

// capabilityTable renders the per-instance capability table.
func capabilityTable(d *mdDoc, s *Server, lang string) {
	if lang == "en" {
		d.line("| Capability | Status |")
		d.line("| --- | --- |")
		d.line("| Upload / create | " + yesNo(s.args.AllowUpload, lang) + " |")
		d.line("| Delete / rename / move | " + yesNo(s.args.AllowDelete, lang) + " |")
		d.line("| Search | " + yesNo(s.args.AllowSearch, lang) + " |")
		d.line("| Download as zip | " + yesNo(s.args.AllowArchive, lang) + " |")
		d.line("| File hash (SHA256) | " + yesNo(s.args.AllowHash, lang) + " |")
		return
	}
	d.line("| 能力 | 状态 |")
	d.line("| --- | --- |")
	d.line("| 上传 / 新建 | " + yesNo(s.args.AllowUpload, lang) + " |")
	d.line("| 删除 / 重命名 / 移动 | " + yesNo(s.args.AllowDelete, lang) + " |")
	d.line("| 搜索 | " + yesNo(s.args.AllowSearch, lang) + " |")
	d.line("| 打包下载 | " + yesNo(s.args.AllowArchive, lang) + " |")
	d.line("| 文件哈希（SHA256） | " + yesNo(s.args.AllowHash, lang) + " |")
}

// genReadmeDoc renders the human-facing manual (features and UI usage).
func genReadmeDoc(s *Server, base string, lang string) string {
	if lang == "en" {
		return genReadmeDocEN(s, base)
	}
	return genReadmeDocZH(s, base)
}

// genAgentDoc renders the machine-facing manual (HTTP/curl API reference).
func genAgentDoc(s *Server, base string, lang string) string {
	if lang == "en" {
		return genAgentDocEN(s, base)
	}
	return genAgentDocZH(s, base)
}

func genReadmeDocZH(s *Server, base string) string {
	var d mdDoc
	d.h(1, "gofs 使用说明")
	d.blank()
	d.line("服务地址：" + base)
	d.line("本文件地址：" + base + "/" + readmeDocPath)
	d.line("程序接口手册：" + base + "/" + agentDocPath)
	d.line("English version: " + base + "/" + readmeDocPath + "?lang=en")
	d.blank()
	d.line("gofs 是单二进制的 Go 文件服务器：支持网页浏览、上传下载、")
	d.line("在线预览与编辑、打包下载、搜索、WebDAV 以及 curl 调用。")

	d.h(2, "Web 界面")
	d.h(3, "浏览与导航")
	d.list(
		"左侧目录树点击文件夹进入；右侧列表点击文件夹进入、点击文件预览。",
		"顶部面包屑可快速回到任意上级目录；地址用 `#/路径` 记录，可收藏、刷新不丢位置。",
		"侧栏与内容之间的分隔条可左右拖动调整宽度。",
	)
	d.h(3, "搜索与排序")
	d.list(
		"搜索框输入关键字后回车，在当前目录递归搜索。",
		"按名称 / 修改时间 / 大小排序，再次点击同一列切换升序降序。",
	)
	d.h(3, "文件操作")
	d.list(
		"上传：点「上传」选择文件，显示实时进度。",
		"新建文件夹 / 新建文件；重命名、删除（删除有二次确认）。",
		"打包下载：把当前目录或某个子目录打包为 zip。",
		"复制完整路径、复制 curl 下载命令、复制 curl 上传命令。",
		"每行右侧的「新窗口打开」会在浏览器新标签页直接打开该文件。",
	)
	d.h(3, "预览与编辑")
	d.list(
		"支持图片、视频、音频、PDF、Markdown、代码、JSON、文本在线预览。",
		"文本类文件可点「编辑」在线修改，改完点「保存」生效；Markdown 编辑时可切换预览。",
		"预览窗口底部显示文件 SHA256，可一键复制。",
	)
	d.h(3, "外观与语言")
	d.list(
		"侧栏底部可切换主题：自动 / 亮色 / 暗色。",
		"侧栏底部可切换界面语言：中文 / English。",
		"偏好保存在浏览器本地，刷新后保持不变。",
	)

	d.h(2, "本实例能力")
	capabilityTable(&d, s, "zh")
	d.blank()
	d.line("未开放的能力在界面上不显示对应按钮，服务端也会拒绝相应请求。")

	d.h(2, "命令行")
	d.line("```sh")
	d.line("gofs [OPTIONS] [serve-path]")
	d.line("```")
	d.blank()
	d.list(
		"`-b, --bind <addrs>` 监听地址；`-p, --port <port>` 监听端口（默认 5000）。",
		"`--path-prefix <path>` 部署在子路径下时使用，虚拟文件的地址也会随之变化。",
		"`-A, --allow-all` 放开全部操作；也可分别用 `--allow-upload`、`--allow-delete`、`--allow-search`、`--allow-archive`、`--allow-hash` 精细控制。",
		"`-a, --auth <rules>` 认证与权限，例如 `admin:pass@/:rw,guest:pass@/:r`。",
		"`--hidden <value>` 在列表中隐藏指定路径。",
	)

	d.h(2, "WebDAV")
	d.line("挂载地址即服务根 `" + base + "/`，可用任意支持 WebDAV 的客户端连接")
	d.line("（Windows 映射网络驱动器、macOS Finder、Cyberduck、rclone 等）。")

	d.h(2, "内置虚拟文件")
	d.line("以下文件不存在于磁盘上，由服务按请求动态生成，因此其中的地址始终与当前访问地址一致；")
	d.line("若磁盘中存在同名真实文件，则优先返回真实文件。")
	d.blank()
	d.line("| 路径 | 用途 |")
	d.line("| --- | --- |")
	d.line("| `/" + readmeDocPath + "` | 本文件，面向使用者 |")
	d.line("| `/" + agentDocPath + "` | 程序接口手册，面向脚本 / Agent |")
	d.blank()
	d.line("两份文档都支持 `?lang=zh` / `?lang=en`，未指定时按 `Accept-Language` 协商。")
	return d.String()
}

func genReadmeDocEN(s *Server, base string) string {
	var d mdDoc
	d.h(1, "gofs User Guide")
	d.blank()
	d.line("Service URL: " + base)
	d.line("This document: " + base + "/" + readmeDocPath)
	d.line("API manual: " + base + "/" + agentDocPath)
	d.line("Chinese version: " + base + "/" + readmeDocPath + "?lang=zh")
	d.blank()
	d.line("gofs is a single-binary Go file server with a web UI, uploads and")
	d.line("downloads, in-browser preview and editing, zip download, search,")
	d.line("WebDAV and a curl-friendly API.")

	d.h(2, "Web interface")
	d.h(3, "Browsing and navigation")
	d.list(
		"Click a folder in the left tree to enter it; in the right pane, click a folder to enter it or a file to preview it.",
		"The breadcrumb at the top jumps back to any parent folder. The location is stored in the hash (`#/path`), so it survives reloads and can be bookmarked.",
		"Drag the divider between the sidebar and the content to resize the sidebar.",
	)
	d.h(3, "Search and sort")
	d.list(
		"Type a keyword in the search box and press Enter to search recursively under the current folder.",
		"Sort by name / modified time / size; click the same column again to reverse the order.",
	)
	d.h(3, "File operations")
	d.list(
		"Upload: click \"Upload\" and pick files; progress is shown live.",
		"New folder / new file; rename and delete (deletion asks for confirmation).",
		"Download zip: package the current folder — or any subfolder — as a zip archive.",
		"Copy the full path, the curl download command, or the curl upload command.",
		"\"Open in new tab\" on each row opens the file directly in a new browser tab.",
	)
	d.h(3, "Preview and edit")
	d.list(
		"Images, video, audio, PDF, Markdown, code, JSON and plain text can be previewed in the browser.",
		"Text-like files can be edited in place: click \"Edit\", then \"Save\" to apply. Markdown editing can be toggled to preview.",
		"The preview footer shows the file SHA256 and offers a one-click copy button.",
	)
	d.h(3, "Appearance and language")
	d.list(
		"The sidebar footer switches the theme: auto / light / dark.",
		"The sidebar footer also switches the UI language: Chinese / English.",
		"Preferences are stored locally in the browser and persist across reloads.",
	)

	d.h(2, "Capabilities of this instance")
	capabilityTable(&d, s, "en")
	d.blank()
	d.line("Disabled capabilities hide the corresponding buttons in the UI and are rejected by the server.")

	d.h(2, "Command line")
	d.line("```sh")
	d.line("gofs [OPTIONS] [serve-path]")
	d.line("```")
	d.blank()
	d.list(
		"`-b, --bind <addrs>` listen address; `-p, --port <port>` listen port (default 5000).",
		"`--path-prefix <path>` for deployments under a sub-path; the virtual documents follow it automatically.",
		"`-A, --allow-all` enables every operation; or use `--allow-upload`, `--allow-delete`, `--allow-search`, `--allow-archive`, `--allow-hash` individually.",
		"`-a, --auth <rules>` authentication and permissions, e.g. `admin:pass@/:rw,guest:pass@/:r`.",
		"`--hidden <value>` hides matching paths from listings.",
	)

	d.h(2, "WebDAV")
	d.line("Mount the service root `" + base + "/` with any WebDAV-capable client")
	d.line("(Windows mapped drive, macOS Finder, Cyberduck, rclone, ...).")

	d.h(2, "Built-in virtual files")
	d.line("The files below do not exist on disk — they are generated per request, so the URLs")
	d.line("inside them always match the address you used. A real file with the same name wins.")
	d.blank()
	d.line("| Path | Purpose |")
	d.line("| --- | --- |")
	d.line("| `/" + readmeDocPath + "` | This document, for humans |")
	d.line("| `/" + agentDocPath + "` | API manual, for scripts / agents |")
	d.blank()
	d.line("Both documents accept `?lang=zh` / `?lang=en`; otherwise `Accept-Language` is used.")
	return d.String()
}

func genAgentDocZH(s *Server, base string) string {
	var d mdDoc
	d.h(1, "gofs HTTP 接口手册")
	d.blank()
	d.line("服务地址：" + base)
	d.line("本文件地址：" + base + "/" + agentDocPath)
	d.line("人类向说明：" + base + "/" + readmeDocPath)
	d.line("English version: " + base + "/" + agentDocPath + "?lang=en")
	d.blank()
	d.line("面向脚本与 Agent：所有操作都是标准 HTTP 请求，无需 SDK。")

	d.h(2, "通用约定")
	d.list(
		"路径相对于服务根；若部署在子路径，所有地址以 `"+base+"` 为前缀。",
		"目录信息统一用 `?json=` 取 JSON；不带该参数返回 HTML 页面。",
		"上传会自动创建缺失的父目录，无需先建目录。",
		"写操作需要服务端打开对应的 `--allow-upload` / `--allow-delete` 开关（或直接 `-A`）。认证规则在此基础上限定某个账号能写哪些路径 —— 只给 `rw` 并不会自动开启写权限。",
	)

	d.h(2, "读取")
	d.h(3, "目录列表")
	d.line("```sh")
	d.line(`curl "` + base + `/?json="`)
	d.line("```")
	d.line("返回 JSON，关键字段：")
	d.list(
		"`href` 当前路径；`kind` 为 `Index` / `Edit` / `View`。",
		"`uri_prefix` 部署前缀；`allow_*` 为各能力的布尔开关。",
		"`paths[]` 条目数组，每项含 `path_type`（`Dir`、`SymlinkDir`、`File`、`SymlinkFile`）、`name`、`mtime`（Unix 毫秒）、`size`（文件为字节数，目录为条目数）。",
	)
	d.h(3, "搜索")
	d.line("```sh")
	d.line(`curl "` + base + `/?json=&q=keyword"`)
	d.line("```")
	d.line("在当前目录递归搜索名称匹配 `keyword` 的条目，结果同样以 `paths[]` 返回；")
	d.line("单次最多 " + strconv.Itoa(maxSubpathsCount) + " 条。")
	d.h(3, "下载文件")
	d.line("```sh")
	d.line("curl -O " + base + "/path/to/file.ext")
	d.line("```")

	d.h(2, "写入")
	d.h(3, "上传文件")
	d.line("```sh")
	d.line("curl -T local/path/to/file.ext " + base + "/path/to/file.ext")
	d.line("```")
	d.line("- 成功返回 `201 Created`。")
	d.line("- 覆盖已有文件且内容非空时，需要同时具备删除权限（`--allow-delete` 或 `rw`），否则返回 `403`。")
	d.h(3, "分片 / 续传上传")
	d.line("```sh")
	d.line("curl -T local/path/to/file.ext -H \"X-Update-Range: append\" " + base + "/path/to/file.ext")
	d.line("```")
	d.line("用 `PATCH` 加 `X-Update-Range: bytes=<start>-` 从指定偏移续写，`append` 表示追加到末尾；小于 " +
		strconv.Itoa(resumableUploadMinSize/1024/1024) + " MiB 的文件不保留续传状态。")
	d.h(3, "新建目录")
	d.line("```sh")
	d.line("curl -X MKCOL " + base + "/path/to/dir")
	d.line("```")
	d.line("成功返回 `201`；目录已存在返回 `405 Already exists`。")
	d.h(3, "重命名 / 移动")
	d.line("```sh")
	d.line(`curl -X MOVE -H "Destination: ` + base + `/new/path" ` + base + `/old/path`)
	d.line("```")
	d.h(3, "复制")
	d.line("```sh")
	d.line(`curl -X COPY -H "Destination: ` + base + `/new/path" ` + base + `/old/path`)
	d.line("```")
	d.line("`Destination` 建议做 URI 编码，以便正确处理中文等非 ASCII 文件名。")
	d.h(3, "删除")
	d.line("```sh")
	d.line("curl -X DELETE " + base + "/path/to/file.ext")
	d.line("```")
	d.line("目录会被递归删除，成功返回 `204`。")

	d.h(2, "附加查询参数")
	d.line("| 参数 | 适用对象 | 说明 |")
	d.line("| --- | --- | --- |")
	d.line("| `?json=` | 目录 / 文件 | 输出 JSON 元信息 |")
	d.line("| `?q=keyword` | 目录 | 递归搜索 |")
	d.line("| `?zip` | 目录 | 打包为 zip 下载 |")
	d.line("| `?hash` | 文件 | 返回 SHA256 十六进制摘要 |")
	d.line("| `?simple` / `?noscript` | 目录 | 返回无脚本的简单 HTML 页面 |")
	d.blank()
	d.line("```sh")
	d.line(`curl -o dir.zip "` + base + `/path/to/dir?zip"`)
	d.line(`curl "` + base + `/path/to/file.ext?hash"`)
	d.line("```")

	d.h(2, "认证")
	d.line("服务启用 `--auth` 后，未授权请求返回 `401`。以下两种方式等价：")
	d.line("```sh")
	d.line("curl -u user:pass " + base + "/path/to/file.ext")
	d.line(`curl -H "Authorization: Basic $(echo -n user:pass | base64)" ` + base + `/path/to/file.ext`)
	d.line("```")
	d.line("也支持 `?token=<token>` 查询参数方式（由 `--auth` 规则生成的令牌）。")

	d.h(2, "状态码")
	d.line("| 状态码 | 含义 |")
	d.line("| --- | --- |")
	d.line("| `200` | 成功 |")
	d.line("| `201` | 创建成功（`PUT`、`MKCOL`） |")
	d.line("| `204` | 成功且无响应体（`DELETE`、`MOVE`、`COPY`、分片 `PUT`） |")
	d.line("| `400` | 路径非法或请求头参数错误 |")
	d.line("| `401` | 需要认证 |")
	d.line("| `403` | 无权限（能力未开启或账号权限不足） |")
	d.line("| `404` | 目标不存在 |")
	d.line("| `405` | 方法不允许，或目标已存在 |")
	d.line("| `412` | WebDAV 前置条件失败（`If-Match` / `If-None-Match`） |")

	d.h(2, "本实例能力")
	capabilityTable(&d, s, "zh")
	d.blank()
	d.line("未开放的能力对应请求会返回 `403`。")

	d.h(2, "相关文档")
	d.list(
		"`/"+readmeDocPath+"`：面向人的功能与部署说明。",
		"`/"+agentDocPath+"`：本文件。",
	)
	return d.String()
}

func genAgentDocEN(s *Server, base string) string {
	var d mdDoc
	d.h(1, "gofs HTTP API Manual")
	d.blank()
	d.line("Service URL: " + base)
	d.line("This document: " + base + "/" + agentDocPath)
	d.line("User guide: " + base + "/" + readmeDocPath)
	d.line("Chinese version: " + base + "/" + agentDocPath + "?lang=zh")
	d.blank()
	d.line("For scripts and agents: every operation is a plain HTTP request, no SDK needed.")

	d.h(2, "Conventions")
	d.list(
		"Paths are relative to the service root; when deployed under a sub-path, prefix every URL with `"+base+"`.",
		"Request directory metadata as JSON with `?json=`; without it an HTML page is returned.",
		"Uploads create missing parent directories automatically — no need to create them first.",
		"Write operations require the matching `--allow-upload` / `--allow-delete` switch (or `-A`). Auth rules then limit *which paths* a given account may write to; `rw` alone does not enable writes.",
	)

	d.h(2, "Reading")
	d.h(3, "Directory listing")
	d.line("```sh")
	d.line(`curl "` + base + `/?json="`)
	d.line("```")
	d.line("Returns JSON with these key fields:")
	d.list(
		"`href` current path; `kind` is `Index` / `Edit` / `View`.",
		"`uri_prefix` deployment prefix; `allow_*` are booleans for each capability.",
		"`paths[]` array of entries; each has `path_type` (`Dir`, `SymlinkDir`, `File`, `SymlinkFile`), `name`, `mtime` (Unix milliseconds) and `size` (bytes for files, entry count for directories).",
	)
	d.h(3, "Search")
	d.line("```sh")
	d.line(`curl "` + base + `/?json=&q=keyword"`)
	d.line("```")
	d.line("Searches recursively under the current folder for names matching `keyword`;")
	d.line("results use the same `paths[]` shape, capped at " + strconv.Itoa(maxSubpathsCount) + " entries per request.")
	d.h(3, "Download a file")
	d.line("```sh")
	d.line("curl -O " + base + "/path/to/file.ext")
	d.line("```")

	d.h(2, "Writing")
	d.h(3, "Upload a file")
	d.line("```sh")
	d.line("curl -T local/path/to/file.ext " + base + "/path/to/file.ext")
	d.line("```")
	d.line("- Returns `201 Created` on success.")
	d.line("- Overwriting an existing non-empty file also requires delete permission (`--allow-delete` or `rw`), otherwise `403`.")
	d.h(3, "Chunked / resumable upload")
	d.line("```sh")
	d.line("curl -T local/path/to/file.ext -H \"X-Update-Range: append\" " + base + "/path/to/file.ext")
	d.line("```")
	d.line("Resume with `PATCH` and `X-Update-Range: bytes=<start>-` to continue from an offset,\n" +
		"or `X-Update-Range: append` to append; files smaller than " +
		strconv.Itoa(resumableUploadMinSize/1024/1024) + " MiB do not keep resumable state.")
	d.h(3, "Create a directory")
	d.line("```sh")
	d.line("curl -X MKCOL " + base + "/path/to/dir")
	d.line("```")
	d.line("Returns `201` on success, `405 Already exists` when it exists.")
	d.h(3, "Rename / move")
	d.line("```sh")
	d.line(`curl -X MOVE -H "Destination: ` + base + `/new/path" ` + base + `/old/path`)
	d.line("```")
	d.h(3, "Copy")
	d.line("```sh")
	d.line(`curl -X COPY -H "Destination: ` + base + `/new/path" ` + base + `/old/path`)
	d.line("```")
	d.line("URI-encode `Destination` so non-ASCII (e.g. Chinese) file names work correctly.")
	d.h(3, "Delete")
	d.line("```sh")
	d.line("curl -X DELETE " + base + "/path/to/file.ext")
	d.line("```")
	d.line("Directories are removed recursively; success returns `204`.")

	d.h(2, "Extra query parameters")
	d.line("| Parameter | Applies to | Description |")
	d.line("| --- | --- | --- |")
	d.line("| `?json=` | directory / file | output JSON metadata |")
	d.line("| `?q=keyword` | directory | recursive search |")
	d.line("| `?zip` | directory | package as a zip download |")
	d.line("| `?hash` | file | return the SHA256 hex digest |")
	d.line("| `?simple` / `?noscript` | directory | return the script-free simple HTML page |")
	d.blank()
	d.line("```sh")
	d.line(`curl -o dir.zip "` + base + `/path/to/dir?zip"`)
	d.line(`curl "` + base + `/path/to/file.ext?hash"`)
	d.line("```")

	d.h(2, "Authentication")
	d.line("With `--auth` enabled, unauthorized requests return `401`. These two forms are equivalent:")
	d.line("```sh")
	d.line("curl -u user:pass " + base + "/path/to/file.ext")
	d.line(`curl -H "Authorization: Basic $(echo -n user:pass | base64)" ` + base + `/path/to/file.ext`)
	d.line("```")
	d.line("A `?token=<token>` query parameter is also supported (tokens are produced by the `--auth` rules).")

	d.h(2, "Status codes")
	d.line("| Code | Meaning |")
	d.line("| --- | --- |")
	d.line("| `200` | success |")
	d.line("| `201` | created (`PUT`, `MKCOL`) |")
	d.line("| `204` | success without a body (`DELETE`, `MOVE`, `COPY`, chunked `PUT`) |")
	d.line("| `400` | invalid path or request header parameters |")
	d.line("| `401` | authentication required |")
	d.line("| `403` | forbidden (capability disabled or insufficient permissions) |")
	d.line("| `404` | not found |")
	d.line("| `405` | method not allowed, or target already exists |")
	d.line("| `412` | WebDAV precondition failed (`If-Match` / `If-None-Match`) |")

	d.h(2, "Capabilities of this instance")
	capabilityTable(&d, s, "en")
	d.blank()
	d.line("Requests for disabled capabilities return `403`.")

	d.h(2, "Related documents")
	d.list(
		"`/"+readmeDocPath+"`: features and deployment notes for humans.",
		"`/"+agentDocPath+"`: this document.",
	)
	return d.String()
}
