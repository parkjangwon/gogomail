package imapgw

import (
	"bytes"
	"sync"
)

const (
	imapLiteralBufferSize       = 4096
	imapMaxPooledBufferSize     = 1 << 20
	imapCommandLiteralPoolSize  = 16
	imapResponseBufferPoolSize  = 16
	imapSectionLiteralPoolSize  = 8
)

var (
	// literalBufferPool reuses []byte buffers for reading command literals and section data
	literalBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, imapLiteralBufferSize)
		},
	}

	// responseWriterPool reuses bytes.Buffer instances for building IMAP responses
	responseWriterPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
)

// acquireLiteralBuffer gets a reusable []byte buffer for reading literals
func acquireLiteralBuffer() []byte {
	return literalBufferPool.Get().([]byte)
}

// releaseLiteralBuffer returns a []byte buffer to the pool
func releaseLiteralBuffer(b []byte) {
	if cap(b) <= imapMaxPooledBufferSize {
		literalBufferPool.Put(b)
	}
}

// acquireResponseWriter gets a reusable bytes.Buffer for building responses
func acquireResponseWriter() *bytes.Buffer {
	buf := responseWriterPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// releaseResponseWriter returns a bytes.Buffer to the pool
func releaseResponseWriter(buf *bytes.Buffer) {
	if buf.Cap() <= imapMaxPooledBufferSize {
		responseWriterPool.Put(buf)
	}
}
