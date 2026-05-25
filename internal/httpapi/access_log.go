package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type accessLogResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *accessLogResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *accessLogResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += int64(n)
	return n, err
}

func (w *accessLogResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// AccessLogMiddleware emits one structured English log event for every HTTP
// request. It intentionally logs route-shaped paths and metadata, never bodies.
func AccessLogMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &accessLogResponseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		status := rw.status
		if status == 0 {
			status = http.StatusOK
		}
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}
		attrs := []slog.Attr{
			slog.String("method", r.Method),
			slog.String("route", normalizePath(r.URL.Path)),
			slog.Int("status", status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.Int64("bytes", rw.bytes),
			slog.String("remote_ip", requestRemoteIP(r)),
			slog.String("user_agent", boundedLogString(r.UserAgent(), 256)),
		}
		attrs = append(attrs, RequestContextAttrs(r.Context())...)
		logger.LogAttrs(r.Context(), level, "http request", attrs...)
	})
}

func requestRemoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		first := strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
		if net.ParseIP(first) != nil {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return boundedLogString(r.RemoteAddr, 128)
}

func boundedLogString(value string, max int) string {
	value = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == '\t' {
			return ' '
		}
		return r
	}, strings.TrimSpace(value))
	if len(value) <= max {
		return value
	}
	return value[:max] + "...(" + strconv.Itoa(len(value)) + " bytes)"
}
