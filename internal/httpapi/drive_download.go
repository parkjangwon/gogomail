package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gogomail/gogomail/internal/drive"
)

func writeDriveFileDownloadHeaders(w http.ResponseWriter, node drive.Node) {
	w.Header().Set("Content-Type", attachmentContentType(node.MIMEType))
	w.Header().Set("Content-Disposition", contentDispositionAttachment(node.Name))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Accept-Ranges", "bytes")
	if sha256Hex := safeSHA256Header(node.ChecksumSHA256); sha256Hex != "" {
		w.Header().Set("X-Gogomail-Drive-SHA256", sha256Hex)
	}
	if node.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(node.Size, 10))
	}
}

type httpByteRange struct {
	Offset int64
	Length int64
	Total  int64
}

func writeDriveFilePartialDownloadHeaders(w http.ResponseWriter, node drive.Node, byteRange httpByteRange) {
	partial := driveNodeWithStatSize(node, byteRange.Length)
	writeDriveFileDownloadHeaders(w, partial)
	w.Header().Set("Content-Range", "bytes "+strconv.FormatInt(byteRange.Offset, 10)+"-"+strconv.FormatInt(byteRange.Offset+byteRange.Length-1, 10)+"/"+strconv.FormatInt(byteRange.Total, 10))
}

func writeDriveRangeError(w http.ResponseWriter, total int64, message string) {
	w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(total, 10))
	writeError(w, http.StatusRequestedRangeNotSatisfiable, message)
}

func parseSingleHTTPByteRange(value string, total int64) (httpByteRange, error) {
	value = strings.TrimSpace(value)
	if total < 0 {
		return httpByteRange{}, errInvalidDriveRange()
	}
	if !strings.HasPrefix(strings.ToLower(value), "bytes=") {
		return httpByteRange{}, errInvalidDriveRange()
	}
	spec := strings.TrimSpace(value[len("bytes="):])
	if spec == "" || strings.Contains(spec, ",") {
		return httpByteRange{}, errInvalidDriveRange()
	}
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return httpByteRange{}, errInvalidDriveRange()
	}
	startValue := strings.TrimSpace(parts[0])
	endValue := strings.TrimSpace(parts[1])
	if startValue == "" {
		suffix, ok := parseHTTPRangeNumber(endValue)
		if !ok || suffix <= 0 || total == 0 {
			return httpByteRange{}, errInvalidDriveRange()
		}
		if suffix >= total {
			return httpByteRange{Offset: 0, Length: total, Total: total}, nil
		}
		return httpByteRange{Offset: total - suffix, Length: suffix, Total: total}, nil
	}
	start, ok := parseHTTPRangeNumber(startValue)
	if !ok {
		return httpByteRange{}, errInvalidDriveRange()
	}
	if start >= total {
		return httpByteRange{}, errInvalidDriveRange()
	}
	end := total - 1
	if endValue != "" {
		parsedEnd, ok := parseHTTPRangeNumber(endValue)
		if !ok || parsedEnd < start {
			return httpByteRange{}, errInvalidDriveRange()
		}
		if parsedEnd < end {
			end = parsedEnd
		}
	}
	return httpByteRange{Offset: start, Length: end - start + 1, Total: total}, nil
}

func parseHTTPRangeNumber(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return parsed, err == nil
}

func errInvalidDriveRange() error {
	return errDriveRangeInvalid{}
}

type errDriveRangeInvalid struct{}

func (errDriveRangeInvalid) Error() string {
	return "range must be a single satisfiable bytes range"
}

func driveNodeWithStatSize(node drive.Node, size int64) drive.Node {
	node.Size = size
	return node
}
