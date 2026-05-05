package storage

import "io"

func readStorageCheckBody(reader io.Reader, expectedSize int) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, int64(expectedSize)+1))
}
