package imapimport

import (
	"strconv"
	"strings"
	"time"
)

// quoteIMAP renders a string as an IMAP quoted-string (RFC 3501 §4.3),
// backslash-escaping quotes and backslashes.
func quoteIMAP(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\':
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	b.WriteByte('"')
	return b.String()
}

// trailingLiteral reports whether a line ends with an IMAP literal marker
// "{n}" and returns n. Non-synchronizing literals ("{n+}") are also matched.
func trailingLiteral(line string) (int, bool) {
	if !strings.HasSuffix(line, "}") {
		return 0, false
	}
	open := strings.LastIndexByte(line, '{')
	if open < 0 {
		return 0, false
	}
	inner := line[open+1 : len(line)-1]
	inner = strings.TrimSuffix(inner, "+")
	n, err := strconv.Atoi(inner)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// parseListLine parses a LIST/LSUB untagged response:
//   * LIST (\HasNoChildren) "/" "INBOX"
//   * LIST (\Noselect \HasChildren) "/" "[Gmail]"
func parseListLine(line string) (SourceFolder, bool) {
	rest := strings.TrimSpace(line)
	if !strings.HasPrefix(rest, "* ") {
		return SourceFolder{}, false
	}
	rest = strings.TrimSpace(rest[2:])
	upper := strings.ToUpper(rest)
	if !strings.HasPrefix(upper, "LIST") && !strings.HasPrefix(upper, "LSUB") {
		return SourceFolder{}, false
	}
	// Drop the LIST/LSUB keyword.
	if sp := strings.IndexByte(rest, ' '); sp >= 0 {
		rest = strings.TrimSpace(rest[sp:])
	} else {
		return SourceFolder{}, false
	}
	// Attributes: (\a \b ...)
	var attrs []string
	if strings.HasPrefix(rest, "(") {
		end := strings.IndexByte(rest, ')')
		if end < 0 {
			return SourceFolder{}, false
		}
		attrStr := strings.TrimSpace(rest[1:end])
		if attrStr != "" {
			attrs = strings.Fields(attrStr)
		}
		rest = strings.TrimSpace(rest[end+1:])
	}
	// Delimiter: quoted string or NIL.
	delim, rest2 := readTokenOrQuoted(rest)
	if strings.EqualFold(delim, "NIL") {
		delim = ""
	}
	// Mailbox name: quoted string, literal (already inlined), or atom.
	name := strings.TrimSpace(rest2)
	if idx := strings.IndexByte(name, '\n'); idx >= 0 {
		// Literal form: {n}\n<name>
		if _, ok := trailingLiteral(strings.TrimSpace(name[:idx])); ok {
			name = strings.TrimRight(name[idx+1:], "\r\n")
		}
	} else {
		name = unquoteIMAP(name)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return SourceFolder{}, false
	}
	return SourceFolder{Name: name, Delimiter: delim, Attributes: attrs}, true
}

// readTokenOrQuoted reads either a quoted-string or a bare atom from the front
// of s and returns (value, remainder).
func readTokenOrQuoted(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if s[0] == '"' {
		// Find the closing unescaped quote.
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '"' {
				return unquoteIMAP(s[:i+1]), strings.TrimSpace(s[i+1:])
			}
		}
		return unquoteIMAP(s), ""
	}
	if sp := strings.IndexByte(s, ' '); sp >= 0 {
		return s[:sp], strings.TrimSpace(s[sp+1:])
	}
	return s, ""
}

func unquoteIMAP(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		inner := s[1 : len(s)-1]
		var b strings.Builder
		for i := 0; i < len(inner); i++ {
			if inner[i] == '\\' && i+1 < len(inner) {
				i++
			}
			b.WriteByte(inner[i])
		}
		return b.String()
	}
	return s
}

// parseStatusMessages parses "* STATUS <mbox> (MESSAGES 42)".
func parseStatusMessages(line string) (uint32, bool) {
	upper := strings.ToUpper(line)
	idx := strings.Index(upper, "MESSAGES ")
	if idx < 0 {
		return 0, false
	}
	tail := line[idx+len("MESSAGES "):]
	tail = strings.TrimLeft(tail, " ")
	end := 0
	for end < len(tail) && tail[end] >= '0' && tail[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.ParseUint(tail[:end], 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

// parseExists parses "* 42 EXISTS".
func parseExists(line string) (uint32, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 3 || fields[0] != "*" || strings.ToUpper(fields[2]) != "EXISTS" {
		return 0, false
	}
	n, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

// parseFetchMessage parses a FETCH untagged response of the form:
//   * 1 FETCH (UID 5 FLAGS (\Seen) INTERNALDATE "01-Jan-2024 10:00:00 +0000" BODY[] {12}\n<bytes>)
// The body literal is expected to already be inlined by readLineWithLiterals.
func parseFetchMessage(line string) (SourceMessage, bool) {
	// Split header portion (before first '\n' introduced by an inlined literal)
	// from the literal payload.
	var header, body string
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		header = line[:idx]
		body = line[idx+1:]
	} else {
		header = line
	}
	upper := strings.ToUpper(header)
	if !strings.Contains(upper, "FETCH") {
		return SourceMessage{}, false
	}

	msg := SourceMessage{}
	if uid, ok := extractFetchUint(header, "UID"); ok {
		msg.UID = uid
	}
	msg.Flags = extractFetchFlags(header)
	if d, ok := extractInternalDate(header); ok {
		msg.InternalDate = d
	}

	// The literal payload is the body. When present, trim a possible trailing
	// ")" that closes the FETCH parentheses on servers that put the body last.
	if body != "" {
		body = strings.TrimRight(body, ")")
		msg.Raw = []byte(body)
	}
	return msg, true
}

// extractFetchUint reads "KEY <n>" from a FETCH header.
func extractFetchUint(header, key string) (uint32, bool) {
	upper := strings.ToUpper(header)
	k := strings.ToUpper(key) + " "
	idx := strings.Index(upper, k)
	if idx < 0 {
		return 0, false
	}
	tail := header[idx+len(k):]
	tail = strings.TrimLeft(tail, " ")
	end := 0
	for end < len(tail) && tail[end] >= '0' && tail[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.ParseUint(tail[:end], 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

// extractFetchFlags reads "FLAGS (\Seen \Flagged)" from a FETCH header.
func extractFetchFlags(header string) []string {
	upper := strings.ToUpper(header)
	idx := strings.Index(upper, "FLAGS ")
	if idx < 0 {
		return nil
	}
	tail := header[idx+len("FLAGS "):]
	tail = strings.TrimLeft(tail, " ")
	if len(tail) == 0 || tail[0] != '(' {
		return nil
	}
	end := strings.IndexByte(tail, ')')
	if end < 0 {
		return nil
	}
	inner := strings.TrimSpace(tail[1:end])
	if inner == "" {
		return nil
	}
	return strings.Fields(inner)
}

// extractInternalDate reads INTERNALDATE "01-Jan-2024 10:00:00 +0000".
func extractInternalDate(header string) (time.Time, bool) {
	upper := strings.ToUpper(header)
	idx := strings.Index(upper, "INTERNALDATE ")
	if idx < 0 {
		return time.Time{}, false
	}
	tail := header[idx+len("INTERNALDATE "):]
	tail = strings.TrimLeft(tail, " ")
	if len(tail) == 0 || tail[0] != '"' {
		return time.Time{}, false
	}
	end := strings.IndexByte(tail[1:], '"')
	if end < 0 {
		return time.Time{}, false
	}
	raw := tail[1 : 1+end]
	// IMAP date-time: dd-Mon-yyyy HH:MM:SS +ZZZZ (RFC 3501 §9). Day may be
	// space-padded ("_2").
	for _, layout := range []string{"02-Jan-2006 15:04:05 -0700", "_2-Jan-2006 15:04:05 -0700"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
