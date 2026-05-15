package carddavgw

import (
	"bytes"
	"sync"
)

const (
	carddavXMLBufferSize     = 8192
	carddavMaxPooledBufSize  = 1 << 20
	carddavXMLBufferPoolSize = 16
)

var (
	// xmlBufferPool reuses bytes.Buffer instances for building DAV/CardDAV XML responses
	xmlBufferPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
)

// acquireXMLBuffer gets a reusable bytes.Buffer for building XML responses
func acquireXMLBuffer() *bytes.Buffer {
	buf := xmlBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// releaseXMLBuffer returns a bytes.Buffer to the pool
func releaseXMLBuffer(buf *bytes.Buffer) {
	if buf != nil && buf.Cap() <= carddavMaxPooledBufSize {
		xmlBufferPool.Put(buf)
	}
}
