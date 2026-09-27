package carddavgw

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

var photoLineRegex = regexp.MustCompile(`(?i)^(?:[A-Za-z0-9-]+\.)?PHOTO(?:\;[^\:]*)?:`)

// extractPhotoFromVCard pulls an inline (base64-encoded) PHOTO out of a vCard
// body so it can be stored in a dedicated column and re-merged on read. It
// supports the inline forms emitted by every major native client:
//
//   - vCard 3.0 (Apple Contacts, DAVx5, GNOME/Evolution, Thunderbird):
//     PHOTO;ENCODING=b;TYPE=JPEG:<base64>      (also ENCODING=BASE64, ENCODING=B)
//   - vCard 4.0 (Apple Contacts, DAVx5):
//     PHOTO:data:image/jpeg;base64,<base64>
//     PHOTO;MEDIATYPE=image/jpeg;ENCODING=b:<base64>
//
// Photo *references* (PHOTO:https://…, or any non-base64 value) are left in the
// vCard body untouched so they round-trip without loss. If the inline photo
// cannot be decoded or exceeds MaxPhotoBytes, the PHOTO line is likewise left in
// place rather than silently dropped.
func extractPhotoFromVCard(vcard []byte) ([]byte, string, []byte, error) {
	lines := strings.Split(string(vcard), "\r\n")
	var photoMediaType string
	var photoData []byte
	filteredLines := make([]string, 0, len(lines))
	extracting := false // currently consuming (dropping) a folded PHOTO we extracted
	captured := false   // already captured one inline photo

	for _, line := range lines {
		if line == "" {
			// Preserve blank/terminator lines; they are not photo continuations.
			extracting = false
			filteredLines = append(filteredLines, line)
			continue
		}
		isContinuation := line[0] == ' ' || line[0] == '\t'
		if isContinuation {
			if extracting {
				// Drop folded continuation of the extracted PHOTO line.
				continue
			}
			filteredLines = append(filteredLines, line)
			continue
		}
		extracting = false
		if captured || !photoLineRegex.MatchString(line) {
			filteredLines = append(filteredLines, line)
			continue
		}
		parsed, err := parseVCardContentLineParts(line)
		if err != nil || parsed.Name != "PHOTO" {
			filteredLines = append(filteredLines, line)
			continue
		}
		mediaType, data, ok := decodeInlinePhoto(parsed)
		if !ok {
			// Reference photo or undecodable inline value: keep it verbatim.
			filteredLines = append(filteredLines, line)
			continue
		}
		photoMediaType = mediaType
		photoData = data
		captured = true
		extracting = true // subsequent folded lines belong to this PHOTO
	}

	cleanVCard := []byte(strings.Join(filteredLines, "\r\n"))
	return cleanVCard, photoMediaType, photoData, nil
}

// decodeInlinePhoto returns the normalized media type ("image/<subtype>") and
// decoded bytes for an inline PHOTO content line, or ok=false when the value is
// a reference (URI) or cannot be decoded within MaxPhotoBytes.
func decodeInlinePhoto(line vCardContentLine) (string, []byte, bool) {
	value := strings.TrimSpace(line.Value)
	if value == "" {
		return "", nil, false
	}

	// vCard 4.0 data: URI form — PHOTO:data:image/jpeg;base64,<base64>
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		spec := value[len("data:"):]
		comma := strings.IndexByte(spec, ',')
		if comma < 0 {
			return "", nil, false
		}
		meta := spec[:comma]
		encoded := spec[comma+1:]
		metaParts := strings.Split(meta, ";")
		isBase64 := false
		mediaType := ""
		for i, part := range metaParts {
			part = strings.TrimSpace(part)
			if i == 0 {
				mediaType = strings.ToLower(part)
				continue
			}
			if strings.EqualFold(part, "base64") {
				isBase64 = true
			}
		}
		if !isBase64 {
			return "", nil, false
		}
		data, ok := decodeBase64Photo(encoded)
		if !ok {
			return "", nil, false
		}
		return normalizePhotoMediaType(mediaType, line.Params), data, true
	}

	// vCard 3.0 / 4.0 parameter form — requires an explicit base64 ENCODING.
	// Values without a base64 encoding are treated as URI references.
	if !photoParamsIndicateBase64(line.Params) {
		return "", nil, false
	}
	data, ok := decodeBase64Photo(value)
	if !ok {
		return "", nil, false
	}
	return normalizePhotoMediaType("", line.Params), data, true
}

func photoParamsIndicateBase64(params map[string][]string) bool {
	for _, key := range []string{"ENCODING"} {
		for _, v := range params[key] {
			switch strings.ToUpper(strings.TrimSpace(v)) {
			case "B", "BASE64":
				return true
			}
		}
	}
	return false
}

// normalizePhotoMediaType produces a canonical "image/<subtype>" media type.
// It prefers an explicit data: URI media type, then a MEDIATYPE parameter, then
// a TYPE parameter (vCard 3.0 bare subtype such as JPEG). Falls back to
// application/octet-stream when nothing usable is present.
func normalizePhotoMediaType(dataURIMediaType string, params map[string][]string) string {
	if dataURIMediaType != "" && strings.Contains(dataURIMediaType, "/") {
		return strings.ToLower(dataURIMediaType)
	}
	if v := firstParamValue(params, "MEDIATYPE"); v != "" {
		if strings.Contains(v, "/") {
			return strings.ToLower(v)
		}
		return "image/" + strings.ToLower(v)
	}
	for _, v := range params["TYPE"] {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		lower := strings.ToLower(v)
		switch lower {
		case "b", "base64", "uri", "url", "inline":
			continue
		}
		if strings.Contains(lower, "/") {
			return lower
		}
		return "image/" + lower
	}
	return "application/octet-stream"
}

func firstParamValue(params map[string][]string, key string) string {
	for _, v := range params[key] {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func decodeBase64Photo(encoded string) ([]byte, bool) {
	// Remove any whitespace introduced by line folding or client formatting.
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, encoded)
	if cleaned == "" {
		return nil, false
	}
	decoders := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding}
	for _, dec := range decoders {
		if data, err := dec.DecodeString(cleaned); err == nil {
			if len(data) == 0 || len(data) > MaxPhotoBytes {
				return nil, false
			}
			return data, true
		}
	}
	return nil, false
}

// mergePhotoIntoVCard re-inserts a previously extracted inline PHOTO. The media
// type is stored canonically ("image/<subtype>"); the emitted form is the vCard
// 3.0-compatible parameterized base64 line, which every major native client
// (Apple Contacts, DAVx5, GNOME/Evolution, Thunderbird) accepts in both 3.0 and
// 4.0 documents. The base64 payload is folded at 75 octets per RFC 6350 §3.2.
func mergePhotoIntoVCard(vcard []byte, photoData []byte, photoMediaType string) []byte {
	if len(photoData) == 0 || photoMediaType == "" {
		return vcard
	}

	vCardStr := string(vcard)
	endIdx := strings.LastIndex(vCardStr, "END:VCARD")
	if endIdx == -1 {
		return vcard
	}

	subtype := photoMediaType
	if slash := strings.IndexByte(subtype, '/'); slash >= 0 {
		subtype = subtype[slash+1:]
	}
	subtype = strings.ToUpper(strings.TrimSpace(subtype))
	if subtype == "" {
		subtype = "OCTET-STREAM"
	}

	encoded := base64.StdEncoding.EncodeToString(photoData)
	var b strings.Builder
	b.WriteString("PHOTO;ENCODING=b;TYPE=")
	b.WriteString(subtype)
	b.WriteString(":")
	b.WriteString(foldVCardValue(encoded, b.Len()))
	b.WriteString("\r\n")

	result := vCardStr[:endIdx] + b.String() + vCardStr[endIdx:]
	return []byte(result)
}

// foldVCardValue folds a long value onto continuation lines at 75 octets,
// accounting for the length already written on the first physical line.
func foldVCardValue(value string, firstLinePrefix int) string {
	const maxLine = 75
	if firstLinePrefix+len(value) <= maxLine {
		return value
	}
	var b strings.Builder
	// Fill the remainder of the first line.
	first := maxLine - firstLinePrefix
	if first < 0 {
		first = 0
	}
	if first > len(value) {
		first = len(value)
	}
	b.WriteString(value[:first])
	rest := value[first:]
	for len(rest) > 0 {
		chunk := maxLine - 1 // one octet reserved for the leading space
		if chunk > len(rest) {
			chunk = len(rest)
		}
		b.WriteString("\r\n ")
		b.WriteString(rest[:chunk])
		rest = rest[chunk:]
	}
	return b.String()
}

var categoriesLineRegex = regexp.MustCompile(`(?i)^CATEGORIES(?:\;[^\:]*)?:`)
var groupLineRegex = regexp.MustCompile(`(?i)^GROUP(?:\;[^\:]*)?:`)

func extractCategoriesAndGroupFromVCard(vcard []byte) ([]byte, []string, string, error) {
	lines := strings.Split(string(vcard), "\r\n")
	var categories []string
	var group string
	var filteredLines []string
	categoriesFound := false
	groupFound := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if !categoriesFound && !groupFound {
				filteredLines = append(filteredLines, line)
			}
			continue
		}

		if categoriesLineRegex.MatchString(line) {
			categoriesFound = true
			name, value, err := parseVCardContentLine(line)
			if err == nil && strings.EqualFold(name, "CATEGORIES") {
				cats := strings.Split(value, ",")
				for _, cat := range cats {
					cat = strings.TrimSpace(cat)
					if cat != "" {
						categories = append(categories, cat)
					}
				}
			}
		} else if groupLineRegex.MatchString(line) {
			groupFound = true
			name, value, err := parseVCardContentLine(line)
			if err == nil && strings.EqualFold(name, "GROUP") {
				group = strings.TrimSpace(value)
			}
		} else if (categoriesFound || groupFound) && (strings.HasPrefix(strings.TrimSpace(line), " ") || strings.HasPrefix(strings.TrimSpace(line), "\t")) {
			continue
		} else {
			categoriesFound = false
			groupFound = false
			filteredLines = append(filteredLines, line)
		}
	}

	cleanVCard := []byte(strings.Join(filteredLines, "\r\n"))
	return cleanVCard, categories, group, nil
}

func mergeCategoriesAndGroupIntoVCard(vcard []byte, categories []string, group string) []byte {
	vCardStr := string(vcard)
	endIdx := strings.LastIndex(vCardStr, "END:VCARD")
	if endIdx == -1 {
		return vcard
	}

	var additions string
	if len(categories) > 0 {
		categoryStr := strings.Join(categories, ",")
		additions += fmt.Sprintf("CATEGORIES:%s\r\n", categoryStr)
	}
	if group != "" {
		additions += fmt.Sprintf("GROUP:%s\r\n", group)
	}

	if additions == "" {
		return vcard
	}

	result := vCardStr[:endIdx] + additions + vCardStr[endIdx:]
	return []byte(result)
}
