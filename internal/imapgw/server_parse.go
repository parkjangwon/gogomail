package imapgw

import (
	"fmt"
	"strings"
)

func imapParenthesizedAtomListShapeValid(fields []string) bool {
	value := strings.TrimSpace(strings.Join(fields, " "))
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return false
	}
	if strings.HasPrefix(value, "((") || strings.HasSuffix(value, "))") {
		return false
	}
	depth := 0
	for _, r := range value {
		switch r {
		case '(':
			depth++
			if depth > 1 {
				return false
			}
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func imapParenthesizedAtomListTokens(inner string) ([]string, bool) {
	if inner == "" {
		return nil, true
	}
	if strings.TrimSpace(inner) != inner {
		return nil, false
	}
	tokens := strings.Split(inner, " ")
	for _, token := range tokens {
		if token == "" || strings.TrimSpace(token) != token {
			return nil, false
		}
	}
	return tokens, true
}

func imapFlagList(flags []string) string {
	if len(flags) == 0 {
		return "()"
	}
	return "(" + strings.Join(flags, " ") + ")"
}

func imapQuotedString(value string) string {
	value = strings.ToValidUTF8(value, "?")
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		if r >= 0x80 {
			return '?'
		}
		return r
	}, value)
	return `"` + value + `"`
}

func parseIMAPFields(line string) ([]string, error) {
	return parseIMAPFieldsWithLiteral(line, nil)
}

func parseIMAPFieldsWithLiteral(line string, literals []string) ([]string, error) {
	fields := make([]string, 0, 4)
	literalIndex := 0
	for i := 0; i < len(line); {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			break
		}
		if line[i] == '"' {
			i++
			var b strings.Builder
			closed := false
			for i < len(line) {
				switch line[i] {
				case '\\':
					i++
					if i >= len(line) {
						return nil, fmt.Errorf("unterminated quoted string")
					}
					if line[i] != '\\' && line[i] != '"' {
						return nil, fmt.Errorf("invalid quoted escape")
					}
					b.WriteByte(line[i])
					i++
				case '"':
					i++
					if i < len(line) && line[i] != ' ' && line[i] != '\t' && line[i] != ')' {
						return nil, fmt.Errorf("quoted string must be delimited")
					}
					fields = append(fields, b.String())
					closed = true
				default:
					if line[i] < 0x20 || line[i] >= 0x7f {
						return nil, fmt.Errorf("invalid quoted control character")
					}
					b.WriteByte(line[i])
					i++
				}
				if closed {
					break
				}
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quoted string")
			}
			continue
		}
		if line[i] == '(' && imapParenthesizedFieldNeedsGrouping(line, i) {
			field, next, err := parseIMAPParenthesizedField(line, i, literals, &literalIndex)
			if err != nil {
				return nil, err
			}
			fields = append(fields, field)
			i = next
			continue
		}
		start := i
		for i < len(line) && line[i] != ' ' && line[i] != '\t' {
			if line[i] == '"' && line[start] != '(' {
				return nil, fmt.Errorf("invalid embedded atom quote character")
			}
			if line[i] < 0x20 || line[i] >= 0x7f {
				return nil, fmt.Errorf("invalid atom control character")
			}
			i++
		}
		field := line[start:i]
		if imapLooksLikeLiteral(field) {
			if literalIndex >= len(literals) {
				return nil, fmt.Errorf("imap literal is not available")
			}
			fields = append(fields, literals[literalIndex])
			literalIndex++
			continue
		}
		if strings.HasSuffix(field, ")") && imapLooksLikeLiteral(strings.TrimSuffix(field, ")")) {
			if literalIndex >= len(literals) {
				return nil, fmt.Errorf("imap literal is not available")
			}
			fields = append(fields, literals[literalIndex]+")")
			literalIndex++
			continue
		}
		if imapLooksLikeLiteralPrefix(field) {
			return nil, fmt.Errorf("imap literal syntax is unsupported")
		}
		fields = append(fields, field)
	}
	if literalIndex != len(literals) {
		return nil, fmt.Errorf("unused imap literal")
	}
	return fields, nil
}

func imapRawFieldIsAtom(line string, fieldIndex int) bool {
	kind, ok := imapRawFieldKind(line, fieldIndex)
	return ok && kind == imapRawFieldAtom
}

type imapRawFieldKindValue int

func imapRawFieldKind(line string, fieldIndex int) (imapRawFieldKindValue, bool) {
	current := 0
	for i := 0; i < len(line); {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			return 0, false
		}
		kind := imapRawFieldAtom
		switch line[i] {
		case '"':
			kind = imapRawFieldQuoted
			i++
			for i < len(line) {
				if line[i] == '\\' {
					i += 2
					continue
				}
				if line[i] == '"' {
					i++
					break
				}
				i++
			}
		case '(':
			kind = imapRawFieldList
			depth := 0
			for i < len(line) {
				switch line[i] {
				case '"':
					i++
					for i < len(line) {
						if line[i] == '\\' {
							i += 2
							continue
						}
						if line[i] == '"' {
							i++
							break
						}
						i++
					}
					continue
				case '(':
					depth++
				case ')':
					depth--
					if depth == 0 {
						i++
						goto fieldDone
					}
				}
				i++
			}
		default:
			start := i
			for i < len(line) && line[i] != ' ' && line[i] != '\t' {
				i++
			}
			if imapLooksLikeLiteral(line[start:i]) {
				kind = imapRawFieldLiteral
			}
		}
	fieldDone:
		if current == fieldIndex {
			return kind, true
		}
		current++
	}
	return 0, false
}

func imapParenthesizedFieldNeedsGrouping(line string, start int) bool {
	depth := 0
	quoted := false
	escaped := false
	quotedHasWhitespace := false
	hasLiteralMarker := false
	for i := start; i < len(line); i++ {
		c := line[i]
		if quoted {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				quoted = false
			case ' ', '\t':
				quotedHasWhitespace = true
			}
			continue
		}
		switch c {
		case '"':
			quoted = true
		case '{':
			end := strings.IndexByte(line[i:], '}')
			if end >= 0 && imapParenthesizedLiteralMarkerDelimited(line, start, i) && imapLooksLikeLiteral(line[i:i+end+1]) {
				hasLiteralMarker = true
				i += end
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth <= 0 {
				return quotedHasWhitespace || hasLiteralMarker
			}
		}
	}
	return false
}

func parseIMAPParenthesizedField(line string, start int, literals []string, literalIndex *int) (string, int, error) {
	depth := 0
	quoted := false
	escaped := false
	var field strings.Builder
	for i := start; i < len(line); i++ {
		c := line[i]
		if c < 0x20 || c >= 0x7f {
			return "", 0, fmt.Errorf("invalid parenthesized control character")
		}
		if quoted {
			if escaped {
				if c != '\\' && c != '"' {
					return "", 0, fmt.Errorf("invalid quoted escape")
				}
				field.WriteByte(c)
				escaped = false
				continue
			}
			switch c {
			case '\\':
				field.WriteByte(c)
				escaped = true
			case '"':
				field.WriteByte(c)
				quoted = false
			default:
				field.WriteByte(c)
			}
			continue
		}
		switch c {
		case '"':
			field.WriteByte(c)
			quoted = true
		case '{':
			end := strings.IndexByte(line[i:], '}')
			if end >= 0 {
				marker := line[i : i+end+1]
				if imapParenthesizedLiteralMarkerDelimited(line, start, i) && imapLooksLikeLiteral(marker) {
					if literalIndex == nil || *literalIndex >= len(literals) {
						return "", 0, fmt.Errorf("imap literal is not available")
					}
					literal := literals[*literalIndex]
					if !imapParenthesizedLiteralValueValid(literal) {
						return "", 0, fmt.Errorf("invalid parenthesized literal value")
					}
					field.WriteString(imapQuotedString(literal))
					*literalIndex = *literalIndex + 1
					i += end
					continue
				}
			}
			field.WriteByte(c)
		case '(':
			field.WriteByte(c)
			depth++
		case ')':
			field.WriteByte(c)
			depth--
			if depth < 0 {
				return "", 0, fmt.Errorf("unbalanced parenthesized field")
			}
			if depth == 0 {
				next := i + 1
				if next < len(line) && line[next] != ' ' && line[next] != '\t' {
					return "", 0, fmt.Errorf("parenthesized field must be delimited")
				}
				return field.String(), next, nil
			}
		default:
			field.WriteByte(c)
		}
	}
	if quoted || escaped {
		return "", 0, fmt.Errorf("unterminated quoted string")
	}
	return "", 0, fmt.Errorf("unterminated parenthesized field")
}

func imapParenthesizedLiteralValueValid(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] >= 0x7f {
			return false
		}
	}
	return true
}

func imapParenthesizedLiteralMarkerDelimited(line string, start int, marker int) bool {
	if marker <= start || marker > len(line) {
		return false
	}
	prev := line[marker-1]
	return prev == '(' || prev == ' ' || prev == '\t'
}

func imapLooksLikeLiteral(field string) bool {
	if len(field) < 3 || field[0] != '{' || field[len(field)-1] != '}' {
		return false
	}
	end := len(field) - 1
	if end > 1 && field[end-1] == '+' {
		end--
	}
	if end == 1 {
		return false
	}
	for i := 1; i < end; i++ {
		if field[i] < '0' || field[i] > '9' {
			return false
		}
	}
	return true
}

func imapLooksLikeLiteralPrefix(field string) bool {
	return len(field) >= 2 && field[0] == '{'
}

func imapCommandArgumentString(line string) string {
	line = strings.TrimSpace(line)
	first := strings.IndexAny(line, " \t")
	if first < 0 {
		return ""
	}
	rest := strings.TrimLeft(line[first:], " \t")
	second := strings.IndexAny(rest, " \t")
	if second < 0 {
		return ""
	}
	return strings.TrimSpace(rest[second:])
}

func imapTagValid(tag string) bool {
	return imapAtomValid(tag) && !strings.Contains(tag, "+")
}

func imapAtomValid(tag string) bool {
	if tag == "" {
		return false
	}
	for i := 0; i < len(tag); i++ {
		switch tag[i] {
		case '(', ')', '{', ' ', '\t', '%', '*', '"', '\\', ']':
			return false
		default:
			if tag[i] < 0x20 || tag[i] >= 0x7f {
				return false
			}
		}
	}
	return true
}

func imapParseQuotedToken(value string, start int) (string, int, bool) {
	i := start + 1
	var b strings.Builder
	for i < len(value) {
		switch value[i] {
		case '\\':
			i++
			if i >= len(value) {
				return "", 0, false
			}
			if value[i] != '\\' && value[i] != '"' {
				return "", 0, false
			}
			b.WriteByte(value[i])
			i++
		case '"':
			return b.String(), i + 1, true
		default:
			if value[i] < 0x20 || value[i] >= 0x7f {
				return "", 0, false
			}
			b.WriteByte(value[i])
			i++
		}
	}
	return "", 0, false
}

