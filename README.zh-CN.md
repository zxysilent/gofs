# gofs

[English](README.md) | **简体中文**

一个别具一格的文件服务器 —— [dufs](https://github.com/sigoden/dufs) 的 Go 重实现，
带一套现代化的 Vue 3 网页界面。

支持静态文件服务、上传、搜索、访问控制与 WebDAV。整个项目 —— Go 后端**和** 前端 SPA
—— 都编译进**单个二进制文件**，运行时除 Go 标准库和一个极小的日志库外没有任何依赖。

![gofs 网页界面 —— 亮色主题](docs/screenshots/01-list-light-zh.png)

## 功能特性

**网页界面**（Vue 3 + TypeScript，内嵌进二进制）

- 左右分栏：左侧为按需加载的目录树，右侧为列表 / 网格视图
- 面包屑导航 + hash 路由（`#/path`），深链和刷新都不丢当前位置，全程无整页加载
- **亮色 / 暗色 / 自动主题** 与 **中文 / 英文界面**，偏好保存在浏览器本地
- 在线预览：图片、视频、音频、PDF、Markdown、代码、JSON、纯文本
- 文本就地编辑，`Ctrl/Cmd+S` 保存，Markdown 可切换预览
- 上传实时进度、任意目录打包下载、递归搜索
- 分隔条可拖动调整侧栏宽度；过长名称自动省略号截断

**协议 / 接口**

- 静态服务支持 HTTP range 请求（`Range`、`If-Range`、ETag、`Last-Modified`、条件请求）
- 目录列表支持 HTML、JSON（`?json`）、纯名称（`?simple`）以及面向文本客户端的页面（`?noscript`）
- 上传（`PUT`）、断点续传（`PATCH` + `X-Update-Range`）、删除、`MKCOL`、`COPY`、`MOVE`
  等完整 WebDAV 支持
- 目录打包下载 `.zip`（`?zip`）、递归搜索（`?q=`）、SHA256（`?hash`）、在线编辑器（`?edit` / `?view`）
- 基于账号的访问控制：Basic / Digest 认证、sha512-crypt 密码（`$6$...`）、按路径的读写权限与令牌
- 通过 `--tls-cert` / `--tls-key` 启用 HTTPS，通过 `--bind /path.sock` 使用 unix socket
- 健康检查：`__dufs__/health`
- **两个内置虚拟 Markdown 文档**（见下文），按请求动态生成，且支持中英双语

## 界面截图

以下截图均为真实运行界面的捕获。

| 亮色 | 暗色 |
|------|------|
| ![列表亮色](docs/screenshots/01-list-light-zh.png) | ![列表暗色](docs/screenshots/02-list-dark-zh.png) |

网格视图（暗色主题）：

![网格视图](docs/screenshots/03-grid-dark-zh.png)

Markdown 预览与在线编辑：

| 预览 | 编辑 |
|------|------|
| ![预览](docs/screenshots/04-preview-markdown.png) | ![编辑](docs/screenshots/05-editor.png) |

内置使用说明与 Agent 手册（帮助弹窗中的两个标签页）：

| 使用说明 | Agent 说明 |
|----------|------------|
| ![帮助](docs/screenshots/06-help-dialog.png) | ![Agent](docs/screenshots/07-agent-manual.png) |

英文界面：

| 亮色 | 暗色 |
|------|------|
| ![英文亮色](docs/screenshots/08-list-light-en.png) | ![英文暗色](docs/screenshots/09-list-dark-en.png) |

## 构建

需要 Go 1.24+。

```sh
# 前端产物已提交在 web/dist，因此普通构建完全不需要 Node.js：
go build -o gofs .
```

网页界面源码在 `web/`（Vue 3 + Vite + TypeScript）。编译产物 `web/dist/` 已提交，
并通过 `//go:embed web/dist` 内嵌进二进制，所以 `go build` 无需 npm。

只有在修改前端时才需要重新构建：

```sh
cd web
bash install-deps.sh          # npm install + npm run build
```

## 使用

```
gofs [OPTIONS] [serve-path]

Options:
  -c, --config <file>        Specify configuration file
  -b, --bind <addrs>         Specify bind address or unix socket
  -p, --port <port>          Specify port to listen on [default: 5000]
      --path-prefix <path>   Specify a path prefix
      --hidden <value>       Hide paths from directory listings, e.g. tmp,*.log,*.lock
  -a, --auth <rules>         Add auth roles, e.g. user:pass@/dir1:rw,/dir2
  -A, --allow-all            Allow all operations
      --allow-upload         Allow upload files/folders
      --allow-delete         Allow delete files/folders
      --allow-search         Allow search files/folders
      --allow-symlink        Allow symlink to files/folders outside root directory
      --allow-archive        Allow download folders as archive file
      --allow-hash           Allow ?hash query to get file sha256 hash
      --enable-cors          Enable CORS, sets `Access-Control-Allow-Origin: *`
      --render-index         Serve index.html when requesting a directory, returns 404 if not found index.html
      --render-try-index     Serve index.html when requesting a directory, returns directory listing if not found index.html
      --render-spa           Serve SPA (Single Page Application)
      --assets <path>        Set the path to the assets directory for overriding the built-in assets
      --log-format <format>  Customize http log format
      --log-file <file>      Specify the file to save logs to, other than stdout/stderr
      --compress <level>     Set zip compress level [default: low] [possible values: none, low, medium, high]
      --completions <shell>  Print shell completion script for <shell>
      --tls-cert <path>      Path to an SSL/TLS certificate to serve with HTTPS
      --tls-key <path>       Path to the SSL/TLS certificate's private key
  -h, --help                 Print help
  -V, --version              Print version
```

所有选项也可通过 `DUFS_*` 环境变量设置（例如 `DUFS_PORT=5000`、
`DUFS_AUTH=admin:123@/:rw`），或用 `--config` 指定 YAML 配置文件。

### 示例

```sh
# 只读方式服务当前目录
gofs

# 对指定目录开放全部操作
gofs -A /srv/data

# 只读分享，开启搜索与打包下载（典型的公开共享配置）
gofs --allow-search --allow-archive --allow-hash --port 80 /srv/data

# 要求用户名/密码并具备写权限
gofs -a 'admin:123@/:rw'

# HTTPS
gofs --tls-cert cert.pem --tls-key key.pem
```

## 内置虚拟文档

两个 Markdown 文档按请求动态生成，因此其中的地址始终与客户端实际访问的地址一致
（对反向代理和 `--path-prefix` 部署友好）。若磁盘上存在同名真实文件，则优先返回真实文件。

| 路径 | 面向对象 | 内容 |
|------|----------|------|
| `/gofs-readme.md` | 使用者 | 功能、界面用法、部署说明 |
| `/gofs-agent.md` | 脚本 / AI Agent | 完整的 HTTP + curl 接口参考 |

两者都支持中文和英文：

```sh
curl http://localhost:5000/gofs-agent.md             # 中文（默认）
curl 'http://localhost:5000/gofs-agent.md?lang=en'   # 英文
```

未指定 `?lang=` 时按 `Accept-Language` 请求头协商，最终回退为中文。响应会带上
`Content-Language` 与 `Vary: Accept-Language`。

在网页界面中也可以打开 Agent 手册：**使用说明 → Agent说明**，支持一键复制全文。

## curl 速查

```sh
# 以 JSON 列出目录
curl "http://localhost:5000/?json="

# 递归搜索
curl "http://localhost:5000/?json=&q=keyword"

# 下载文件
curl -O http://localhost:5000/path/to/file.ext

# 上传文件（会自动创建缺失的父目录）
curl -T local/file.ext http://localhost:5000/path/to/file.ext

# 新建目录
curl -X MKCOL http://localhost:5000/path/to/dir

# 重命名 / 移动
curl -X MOVE -H "Destination: http://localhost:5000/new/path" http://localhost:5000/old/path

# 复制
curl -X COPY -H "Destination: http://localhost:5000/new/path" http://localhost:5000/old/path

# 删除（目录会被递归删除）
curl -X DELETE http://localhost:5000/path/to/file.ext

# 目录打包下载
curl -o dir.zip "http://localhost:5000/path/to/dir?zip"

# 获取文件 SHA256
curl "http://localhost:5000/path/to/file.ext?hash"
```

状态码、认证方式、分片上传以及完整参数表见 `/gofs-agent.md`。

## 目录结构

| 文件 | 用途 |
|------|------|
| `main.go` | 入口、监听设置、信号处理 |
| `args.go` | 命令行 / 环境变量 / 配置文件解析 |
| `server.go` | HTTP + WebDAV 请求处理、内嵌资源服务 |
| `docs.go` | 内置虚拟 Markdown 文档（中英双语） |
| `auth.go` | 访问控制、Basic/Digest 认证、sha512-crypt |
| `logger.go` | 可自定义的访问日志格式 |
| `utils.go` | URI 编码、glob、range 解析、内容类型 |
| `noscript.go` | 面向文本客户端的目录列表 |
| `web/` | Vue 3 + TypeScript SPA 源码（Vite 构建） |
| `web/dist/` | 构建后的前端产物，通过 `go:embed` 内嵌进二进制 |
| `docs/screenshots/` | 本 README 使用的截图 |

## 许可证

MIT —— 见 [LICENSE](LICENSE)。源自 [dufs](https://github.com/sigoden/dufs)。
