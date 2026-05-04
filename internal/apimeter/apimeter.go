package apimeter

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
)

const defaultTimeout = 100 * time.Millisecond

// Event is the API usage record emitted by the metering middleware.
type Event struct {
	Method        string
	RoutePattern  string
	Status        int
	RequestBytes  int64
	ResponseBytes int64
	Latency       time.Duration
	Timestamp     time.Time
	UserID        string
}

// Sink receives API metering events.
type Sink interface {
	Record(ctx context.Context, event Event) error
}

// NoopSink discards API metering events.
type NoopSink struct{}

func (NoopSink) Record(context.Context, Event) error {
	return nil
}

type SlogSink struct {
	Logger *slog.Logger
}

func (s SlogSink) Record(_ context.Context, event Event) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info(
		"api usage recorded",
		"method", event.Method,
		"route", event.RoutePattern,
		"status", event.Status,
		"request_bytes", event.RequestBytes,
		"response_bytes", event.ResponseBytes,
		"latency_ms", event.Latency.Milliseconds(),
		"user_id", event.UserID,
		"timestamp", event.Timestamp.Format(time.RFC3339Nano),
	)
	return nil
}

type config struct {
	timeout time.Duration
}

// Option configures the metering middleware.
type Option func(*config)

// WithTimeout sets the maximum time allowed for a sink call.
func WithTimeout(timeout time.Duration) Option {
	return func(cfg *config) {
		if timeout > 0 {
			cfg.timeout = timeout
		}
	}
}

// Handler wraps next with asynchronous fail-open API metering.
func Handler(next http.Handler, sink Sink, opts ...Option) http.Handler {
	if sink == nil {
		sink = NoopSink{}
	}
	cfg := config{timeout: defaultTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		mw := &meteredResponseWriter{ResponseWriter: w, status: http.StatusOK}
		var requestBytes int64
		if r.Body != nil {
			r.Body = &countingReadCloser{ReadCloser: r.Body, bytes: &requestBytes}
		}

		next.ServeHTTP(mw, r)

		if r.ContentLength > requestBytes {
			requestBytes = r.ContentLength
		}
		event := Event{
			Method:        r.Method,
			RoutePattern:  r.Pattern,
			Status:        mw.status,
			RequestBytes:  requestBytes,
			ResponseBytes: mw.bytes,
			Latency:       time.Since(start),
			Timestamp:     start,
			UserID:        r.URL.Query().Get("user_id"),
		}
		go recordFailOpen(sink, cfg.timeout, event)
	})
}

func recordFailOpen(sink Sink, timeout time.Duration, event Event) {
	defer func() {
		_ = recover()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = sink.Record(ctx, event)
}

type countingReadCloser struct {
	io.ReadCloser
	bytes *int64
}

func (r *countingReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	*r.bytes += int64(n)
	return n, err
}

type meteredResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
	wrote  bool
}

func (w *meteredResponseWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *meteredResponseWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

func (w *meteredResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *meteredResponseWriter) Flush() {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *meteredResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *meteredResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *meteredResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		n, err := readerFrom.ReadFrom(r)
		w.bytes += n
		return n, err
	}
	return io.Copy(w, r)
}
