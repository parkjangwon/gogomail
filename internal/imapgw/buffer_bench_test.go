package imapgw

import (
	"strings"
	"testing"
)

// BenchmarkLiteralBufferPool measures literal buffer pool reuse efficiency
func BenchmarkLiteralBufferPool(b *testing.B) {
	tests := []struct {
		name   string
		size   int
		reuse  bool
	}{
		{"small_no_pool", 512, false},
		{"small_with_pool", 512, true},
		{"medium_no_pool", 4096, false},
		{"medium_with_pool", 4096, true},
		{"large_no_pool", 16384, false},
		{"large_with_pool", 16384, true},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if tt.reuse {
					buf := acquireLiteralBuffer()
					if len(buf) >= tt.size {
						releaseLiteralBuffer(buf[:tt.size])
					} else {
						releaseLiteralBuffer(buf)
					}
				} else {
					_ = make([]byte, tt.size)
				}
			}
		})
	}
}

// BenchmarkReadIMAPSectionLiteral measures section literal reading performance
func BenchmarkReadIMAPSectionLiteral(b *testing.B) {
	tests := []struct {
		name      string
		dataSize  int
		wantHeader bool
	}{
		{"small_header", 512, true},
		{"small_body", 512, false},
		{"medium_header", 16384, true},
		{"medium_body", 16384, false},
		{"large_header", 1048576, true},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			data := strings.Repeat("X", tt.dataSize)
			if tt.wantHeader {
				data += "\r\n\r\nBody content here"
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(data)
				_, _ = readIMAPSectionLiteral(reader, tt.wantHeader)
			}
		})
	}
}

// BenchmarkResponseBufferPool measures response buffer pool reuse efficiency
func BenchmarkResponseBufferPool(b *testing.B) {
	tests := []struct {
		name       string
		dataSize   int
		reuse      bool
	}{
		{"small_no_pool", 256, false},
		{"small_with_pool", 256, true},
		{"medium_no_pool", 4096, false},
		{"medium_with_pool", 4096, true},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if tt.reuse {
					buf := acquireResponseWriter()
					buf.WriteString(strings.Repeat("X", tt.dataSize))
					releaseResponseWriter(buf)
				} else {
					var buf strings.Builder
					buf.WriteString(strings.Repeat("X", tt.dataSize))
					_ = buf.String()
				}
			}
		})
	}
}

// TestLiteralBufferPoolBasic verifies buffer pool acquire/release works
func TestLiteralBufferPoolBasic(t *testing.T) {
	buf1 := acquireLiteralBuffer()
	if len(buf1) != imapLiteralBufferSize {
		t.Errorf("expected buffer size %d, got %d", imapLiteralBufferSize, len(buf1))
	}

	copy(buf1, "test data")
	releaseLiteralBuffer(buf1)

	buf2 := acquireLiteralBuffer()
	if len(buf2) != imapLiteralBufferSize {
		t.Errorf("expected buffer size %d, got %d", imapLiteralBufferSize, len(buf2))
	}
	releaseLiteralBuffer(buf2)
}

// TestResponseWriterPoolBasic verifies response writer pool acquire/release works
func TestResponseWriterPoolBasic(t *testing.T) {
	buf1 := acquireResponseWriter()
	buf1.WriteString("test data")

	releaseResponseWriter(buf1)

	buf2 := acquireResponseWriter()
	if buf2.Len() != 0 {
		t.Errorf("buffer pool returned non-empty buffer, len=%d", buf2.Len())
	}

	buf2.WriteString("new data")
	if buf2.String() != "new data" {
		t.Errorf("expected 'new data', got %q", buf2.String())
	}

	releaseResponseWriter(buf2)
}
