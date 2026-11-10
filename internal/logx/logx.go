package logx

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/zxysilent/gofs/internal/urlutil"
	"github.com/zxysilent/logs"
)

// 访问日志使用固定的结构化字段，不支持自定义模板。
//
// 旧实现（Nginx 风格模板）把所有信息拼成一行塞进 msg=，日志里还会再出现一份
// 时间戳，引号也必须转义，机器很难解析：
//
//	time=2026-11-10T13:04:28.044 level=INF msg="2026-11-10T13:04:28+08:00 INFO - 2.0.0.208 \"GET /a.md\" 200"
//
// 现在每个请求输出两行，共享同一个 trace：
//
//	time=... level=INF trace=x48572hm caller=xbegin ip=127.0.0.1 GET=/a.md?x=1
//	time=... level=INF trace=x48572hm caller=xclose status=200 size=260 dur=824.084µs
//
// 字段含义：
//
//	trace       本次请求的 id，同时通过响应头 X-Request-ID 返回，便于对照
//	caller      xbegin / xclose，同一次请求的两行共享 trace
//	ip          客户端地址（仅 begin）
//	<METHOD>    请求方法作为字段名，值是路径与查询串，例如 GET=/a.md?x=1（仅 begin）
//	status      响应状态码（仅 end）
//	size        响应体字节数（仅 end）
//	dur         处理耗时（仅 end）
//	err         仅出错时出现，错误信息单独成字段
//
// 请求信息只写在 begin，结果只写在 end：两行通过 trace 关联，
// 各自只带自己的字段，不重复。
//
// 静态资源（内置前端产物）不记录访问日志。

// AccessLogger 为每个请求输出访问记录；nil 表示关闭（测试用）。
type AccessLogger struct{}

// remoteIP 从 RemoteAddr 中取出客户端地址（去掉端口；不带端口的原样返回）。
func remoteIP(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

const (
	callerBegin = "xbegin"
	callerEnd   = "xclose"
)

// begin 在请求开始时输出请求信息：来源地址，以及"方法=路径"。
// 方法作为字段名（GET=/a.md），和常见的结构化访问日志保持一致。
func (l *AccessLogger) Begin(r *http.Request, trace string) {
	logs.With(trace).
		Str("caller", callerBegin).
		Str("ip", remoteIP(r.RemoteAddr)).
		Str(r.Method, logURI(r)).
		Info()
}

// end 输出请求结果：状态码、响应体大小、耗时。errMsg 非空表示出错，
// 以 error 级别记录，错误信息放在独立的 err 字段里。
func (l *AccessLogger) End(trace string, status int, size int64, dur time.Duration, errMsg string) {
	f := logs.With(trace).
		Str("caller", callerEnd).
		Int("status", status).
		Int64("size", size).
		Dur("dur", dur)

	if errMsg != "" {
		f.Str("err", sanitizeLogValue(errMsg)).Error()
		return
	}
	f.Info()
}

// RequestIDHeader 把本次请求的 trace 返回给客户端，用户报问题时可以引用它，
// 并与日志对上。
const RequestIDHeader = "X-Request-ID"

// RequestTrace 复用上游的 X-Request-ID（部署在反向代理后面时），
// 不合法则生成一个。
func RequestTrace(r *http.Request) string {
	if id := r.Header.Get(RequestIDHeader); validTraceID(id) {
		return id
	}
	return logs.TraceId()
}

// validTraceID 只接受 8..64 位字母数字，避免客户端往这次请求的每一行日志里
// 注入任意内容。
func validTraceID(id string) bool {
	if len(id) < 8 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !isAlnumOrUnderscore(c) || c == '_' {
			return false
		}
	}
	return true
}

func isAlnumOrUnderscore(c rune) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// logURI 返回请求 URI：百分号解码后剔除控制字符，避免构造出来的路径
// 破坏日志行的结构。
func logURI(r *http.Request) string {
	uri := r.URL.RequestURI()
	if d, ok := urlutil.DecodeURI(uri); ok {
		return sanitizeLogValue(d)
	}
	return sanitizeLogValue(uri)
}

// sanitizeLogValue 去掉控制字符，防止路径或错误信息伪造日志行，
// 也避免把终端转义序列写进日志。引号与反斜杠由日志库负责转义，这里不动。
func sanitizeLogValue(s string) string {
	var b strings.Builder
	for _, c := range s {
		if unicode.IsControl(c) {
			b.WriteString(`\x`)
			b.WriteString(strconv.FormatInt(int64(c), 16))
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}
