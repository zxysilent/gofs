package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/zxysilent/logs"
)

const defaultLogFormat = `$time_iso8601 $log_level - $remote_addr "$request" $status`

type logElementKind int

const (
	logLiteral logElementKind = iota
	logVariable
	logHeader
)

type logElement struct {
	kind logElementKind
	lit  string // literal text
	name string // variable name or header name
}

// HttpLogger formats access log lines from a customizable format string.
type HttpLogger struct {
	elements []logElement
}

func defaultHttpLogger() HttpLogger {
	return parseHttpLogger(defaultLogFormat)
}

func parseHttpLogger(format string) HttpLogger {
	var elements []logElement
	s := format + " "
	cache := ""
	isVar := false
	flushLiteral := func() {
		if cache != "" {
			elements = append(elements, logElement{kind: logLiteral, lit: cache})
		}
		cache = ""
	}
	for _, c := range s {
		if c == '$' {
			flushLiteral()
			isVar = true
		} else if isVar && !isAlnumOrUnderscore(c) {
			if v, ok := strings.CutPrefix(cache, "$http_"); ok {
				elements = append(elements, logElement{kind: logHeader, name: strings.ReplaceAll(v, "_", "-")})
			} else if v, ok := strings.CutPrefix(cache, "$"); ok {
				elements = append(elements, logElement{kind: logVariable, name: v})
			}
			cache = ""
			isVar = false
		}
		cache += string(c)
	}
	cache = strings.TrimSpace(cache)
	if cache != "" {
		elements = append(elements, logElement{kind: logLiteral, lit: cache})
	}
	return HttpLogger{elements: elements}
}

func (l *HttpLogger) empty() bool { return len(l.elements) == 0 }

// data collects the request-derived log variables.
func (l *HttpLogger) data(r *http.Request) map[string]string {
	data := map[string]string{}
	for _, el := range l.elements {
		switch el.kind {
		case logVariable:
			switch el.name {
			case "request", "request_method", "request_uri":
				uri := r.URL.RequestURI()
				decoded := uri
				if d, ok := decodeURI(uri); ok {
					decoded = sanitizeLogValue(d)
				}
				if _, ok := data["request"]; !ok {
					data["request"] = r.Method + " " + decoded
				}
				if _, ok := data["request_method"]; !ok {
					data["request_method"] = r.Method
				}
				if _, ok := data["request_uri"]; !ok {
					data["request_uri"] = decoded
				}
			case "remote_user":
				if user := getAuthUser(r.Header.Get("Authorization")); user != "" {
					data[el.name] = user
				}
			}
		case logHeader:
			if v := r.Header.Get(el.name); v != "" {
				data[el.name] = sanitizeLogValue(v)
			}
		}
	}
	return data
}

// log formats and emits an access log line. errMsg non-empty marks an error.
func (l *HttpLogger) log(data map[string]string, errMsg string) {
	if l.empty() {
		return
	}
	isError := errMsg != ""
	now := time.Now()
	timeLocal := now.Format("2006-01-02T15:04:05-07:00")
	timeIso8601 := now.Format("2006-01-02T15:04:05-07:00")
	msec := fmt.Sprintf("%.3f", float64(now.UnixNano())/1e9)
	logLevel := "INFO"
	if isError {
		logLevel = "ERROR"
	}

	var sb strings.Builder
	for _, el := range l.elements {
		switch el.kind {
		case logLiteral:
			sb.WriteString(el.lit)
		case logVariable:
			var val string
			switch el.name {
			case "time_local":
				val = timeLocal
			case "time_iso8601":
				val = timeIso8601
			case "msec":
				val = msec
			case "log_level":
				val = logLevel
			default:
				val = data[el.name]
				if val == "" {
					val = "-"
				}
			}
			sb.WriteString(val)
		case logHeader:
			val := data[el.name]
			if val == "" {
				val = "-"
			}
			sb.WriteString(val)
		}
	}
	line := sb.String()
	if isError {
		logs.Error(line + " " + errMsg)
	} else {
		logs.Info(line)
	}
}

func isAlnumOrUnderscore(c rune) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func sanitizeLogValue(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch {
		case c == '\\':
			b.WriteString(`\\`)
		case c == '"':
			b.WriteString(`\"`)
		case unicode.IsControl(c):
			b.WriteString(fmt.Sprintf("\\x%02x", c))
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}
