package imapgw

import (
	"bufio"
	"context"
	"encoding/base64"
	"regexp"
	"strings"
	"unicode/utf16"
)

func (s *Server) handleList(writer *bufio.Writer, tag string, fields []string, state *imapConnState, subscribed bool) (bool, error) {
	command := "LIST"
	if subscribed {
		command = "LSUB"
	}
	var listFields []string
	if len(fields) > 2 {
		listFields = fields[2:]
	}
	listOptions, listError, ok := imapListCommandOptions(listFields, subscribed)
	if !ok {
		if listError != "" {
			_, err := writer.WriteString(tag + " BAD " + listError + "\r\n")
			return false, err
		}
		_, err := writer.WriteString(tag + " BAD " + command + " requires reference and mailbox pattern atoms\r\n")
		return false, err
	}
	if len(listOptions.fields) < 2 {
		_, err := writer.WriteString(tag + " BAD " + command + " requires reference and mailbox pattern atoms\r\n")
		return false, err
	}
	patterns, patternOK := imapListPatterns(listOptions.fields)
	if !patternOK {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox pattern is not valid modified UTF-7\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if imapStatusRequestsItem(listOptions.statusItems, "HIGHESTMODSEQ") {
		state.condstoreAware = true
	}
	if len(patterns) == 1 && patterns[0] == "" {
		if listOptions.specialUseOnly {
			_, err := writer.WriteString(tag + " OK " + command + " completed\r\n")
			return false, err
		}
		if _, err := writer.WriteString("* " + command + ` (\Noselect) "/" ""` + "\r\n"); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK " + command + " completed\r\n")
		return false, err
	}
	if subscribed || listOptions.subscribedOnly {
		return s.writeSubscribedListResponses(writer, tag, state, patterns, command, listOptions)
	}
	matcher, ok := imapMailboxPatternMatcherAny(patterns)
	if !ok {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox pattern is invalid\r\n")
		return false, err
	}
	mailboxes, err := s.options.Backend.ListMailboxes(state.ctx, ListMailboxesRequest{UserID: state.session.UserID})
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
		return false, writeErr
	}
	subscribedNames := map[string]struct{}{}
	if listOptions.subscribedReturn {
		var err error
		subscribedNames, err = s.subscribedMailboxWireNames(state.ctx, state)
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
			return false, writeErr
		}
	}
	children := imapMailboxChildren(mailboxes)
	seen := make(map[string]struct{}, len(mailboxes))
	if imapMailboxPatternListContainsRoot(patterns) && !listOptions.specialUseOnly {
		if _, err := writer.WriteString("* " + command + ` (\Noselect) "/" ""` + "\r\n"); err != nil {
			return false, err
		}
	}
	for _, mailbox := range mailboxes {
		displayName := imapMailboxWireName(imapMailboxDisplayName(mailbox))
		if !matcher(displayName) {
			continue
		}
		key := strings.ToLower(displayName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		wireDisplayName := imapEncodeMailboxName(displayName)
		attributes := imapMailboxListAttributes(mailbox, children[mailbox.ID])
		if _, ok := subscribedNames[strings.ToLower(displayName)]; ok {
			attributes = append(attributes, `\Subscribed`)
		}
		if listOptions.specialUseOnly && len(attributes) == 1 {
			continue
		}
		if _, err := writer.WriteString("* " + command + " " + imapFlagList(attributes) + ` "/" ` + imapQuotedString(wireDisplayName) + "\r\n"); err != nil {
			return false, err
		}
		if len(listOptions.statusItems) > 0 {
			if err := writeIMAPStatusLine(writer, imapQuotedString(wireDisplayName), imapStatusData(mailbox, listOptions.statusItems)); err != nil {
				return false, err
			}
		}
	}
	_, err = writer.WriteString(tag + " OK " + command + " completed\r\n")
	return false, err
}

func (s *Server) writeSubscribedListResponses(writer *bufio.Writer, tag string, state *imapConnState, patterns []string, command string, listOptions imapListOptions) (bool, error) {
	subscriptions, err := s.options.Backend.ListSubscribedMailboxes(state.ctx, ListMailboxesRequest{UserID: state.session.UserID})
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
		return false, writeErr
	}
	mailboxes := make([]Mailbox, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		if subscription.Exists {
			mailboxes = append(mailboxes, subscription.Mailbox)
		}
	}
	children := imapMailboxChildren(mailboxes)
	seen := make(map[string]struct{}, len(subscriptions))
	matcher, ok := imapMailboxPatternMatcherAny(patterns)
	if !ok {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox pattern is invalid\r\n")
		return false, err
	}
	for _, subscription := range subscriptions {
		displayName := imapMailboxWireName(subscription.Name)
		if subscription.Exists {
			displayName = imapMailboxWireName(imapMailboxDisplayName(subscription.Mailbox))
		}
		if !matcher(displayName) {
			parentName := imapLSubParentNameAny(displayName, patterns)
			if parentName == "" {
				continue
			}
			key := strings.ToLower(parentName)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if _, err := writer.WriteString("* " + command + ` (\Noselect) "/" ` + imapQuotedString(imapEncodeMailboxName(parentName)) + "\r\n"); err != nil {
				return false, err
			}
			continue
		}
		key := strings.ToLower(displayName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		attributes := []string{`\Noselect`}
		if subscription.Exists {
			attributes = imapMailboxListAttributes(subscription.Mailbox, children[subscription.Mailbox.ID])
		}
		if listOptions.subscribedReturn {
			attributes = append(attributes, `\Subscribed`)
		}
		if _, err := writer.WriteString("* " + command + " " + imapFlagList(attributes) + ` "/" ` + imapQuotedString(imapEncodeMailboxName(displayName)) + "\r\n"); err != nil {
			return false, err
		}
		if subscription.Exists && len(listOptions.statusItems) > 0 {
			if err := writeIMAPStatusLine(writer, imapQuotedString(imapEncodeMailboxName(displayName)), imapStatusData(subscription.Mailbox, listOptions.statusItems)); err != nil {
				return false, err
			}
		}
	}
	_, err = writer.WriteString(tag + " OK " + command + " completed\r\n")
	return false, err
}

func (s *Server) subscribedMailboxWireNames(ctx context.Context, state *imapConnState) (map[string]struct{}, error) {
	subscriptions, err := s.options.Backend.ListSubscribedMailboxes(ctx, ListMailboxesRequest{UserID: state.session.UserID})
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{}, len(subscriptions))
	for _, subscription := range subscriptions {
		displayName := imapMailboxWireName(subscription.Name)
		if subscription.Exists {
			displayName = imapMailboxWireName(imapMailboxDisplayName(subscription.Mailbox))
		}
		if displayName == "" {
			continue
		}
		names[strings.ToLower(displayName)] = struct{}{}
	}
	return names, nil
}

func imapLSubParentNameAny(name string, patterns []string) string {
	for _, pattern := range patterns {
		matcher, ok := imapMailboxPatternMatcher(pattern)
		if !ok {
			continue
		}
		if parentName := imapLSubParentName(name, pattern, matcher); parentName != "" {
			return parentName
		}
	}
	return ""
}

func imapLSubParentName(name string, pattern string, matcher func(string) bool) string {
	if !strings.Contains(pattern, "%") || !strings.Contains(name, "/") {
		return ""
	}
	parts := strings.Split(name, "/")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[:i], "/")
		if matcher(parent) {
			return parent
		}
	}
	return ""
}

type imapListOptions struct {
	fields           []string
	specialUseOnly   bool
	subscribedOnly   bool
	subscribedReturn bool
	statusItems      []string
}

func imapListCommandOptions(fields []string, subscribed bool) (imapListOptions, string, bool) {
	if subscribed {
		if len(fields) > 0 && strings.HasPrefix(strings.TrimSpace(fields[0]), "(") {
			return imapListOptions{}, "LSUB does not support LIST extension options", false
		}
		if len(fields) > 2 && strings.EqualFold(fields[2], "RETURN") {
			return imapListOptions{}, "LSUB does not support LIST extension options", false
		}
		return imapListOptions{fields: fields}, "", true
	}
	options := imapListOptions{}
	if len(fields) > 0 && strings.HasPrefix(strings.TrimSpace(fields[0]), "(") {
		if !strings.HasPrefix(fields[0], "(") {
			return imapListOptions{}, "", false
		}
		optionFields, rest, ok := imapConsumeParenthesizedFields(fields)
		if !ok {
			return imapListOptions{}, "", false
		}
		tokens, ok := imapSearchReturnOptionTokens(optionFields)
		if !ok || len(tokens) == 0 {
			return imapListOptions{}, "", false
		}
		for _, token := range tokens {
			switch strings.ToUpper(token) {
			case "SPECIAL-USE":
				options.specialUseOnly = true
			case "SUBSCRIBED":
				options.subscribedOnly = true
			default:
				return imapListOptions{}, "", false
			}
		}
		fields = rest
	}
	if len(fields) < 2 {
		return imapListOptions{}, "", false
	}
	options.fields = fields[:2]
	rest := fields[2:]
	if strings.HasPrefix(strings.TrimSpace(fields[1]), "(") {
		if !strings.HasPrefix(fields[1], "(") {
			return imapListOptions{}, "", false
		}
		patternFields, patternRest, ok := imapConsumeParenthesizedFields(fields[1:])
		if !ok {
			return imapListOptions{}, "", false
		}
		options.fields = append([]string{fields[0]}, patternFields...)
		rest = patternRest
	}
	if len(rest) == 0 {
		return options, "", true
	}
	if len(rest) < 2 || !strings.EqualFold(rest[0], "RETURN") {
		return imapListOptions{}, "", false
	}
	if !imapListReturnOptionsParenthesized(rest[1:]) {
		return imapListOptions{}, "LIST requires parenthesized return options", false
	}
	if !imapListStatusReturnItemsParenthesized(rest[1:]) {
		return imapListOptions{}, "LIST requires parenthesized status item list", false
	}
	tokens := imapFetchNormalizedTokens(rest[1:])
	if len(tokens) == 0 {
		return imapListOptions{}, "", false
	}
	statusReturnSeen := false
	for i := 0; i < len(tokens); {
		switch strings.ToUpper(tokens[i]) {
		case "CHILDREN":
			i++
		case "SPECIAL-USE":
			i++
		case "SUBSCRIBED":
			options.subscribedReturn = true
			i++
		case "STATUS":
			if statusReturnSeen {
				return imapListOptions{}, "LIST status return option is duplicated", false
			}
			statusReturnSeen = true
			i++
			start := i
			for i < len(tokens) && !strings.EqualFold(tokens[i], "CHILDREN") && !strings.EqualFold(tokens[i], "SPECIAL-USE") && !strings.EqualFold(tokens[i], "SUBSCRIBED") {
				i++
			}
			if start == i {
				return imapListOptions{}, "LIST requires status data items", false
			}
			statusItems, statusErr, ok := imapStatusItemsFromTokens(tokens[start:i])
			if !ok {
				return imapListOptions{}, strings.Replace(statusErr, "STATUS item", "LIST status item", 1), false
			}
			options.statusItems = statusItems
		default:
			return imapListOptions{}, "", false
		}
	}
	return options, "", true
}

func imapListReturnOptionsParenthesized(fields []string) bool {
	value := strings.Join(fields, " ")
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return false
	}
	if strings.HasPrefix(value, "((") {
		return false
	}
	depth := 0
	for _, r := range value {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func imapListStatusReturnItemsParenthesized(fields []string) bool {
	joined := strings.TrimSpace(strings.Join(fields, " "))
	upper := strings.ToUpper(joined)
	offset := 0
	for {
		index := strings.Index(upper[offset:], "STATUS")
		if index < 0 {
			return true
		}
		index += offset
		end := index + len("STATUS")
		if !imapTokenBoundary(upper, index, end) {
			offset = end
			continue
		}
		rest := strings.TrimLeft(joined[end:], " \t")
		return strings.HasPrefix(rest, "(")
	}
}

func imapTokenBoundary(value string, start int, end int) bool {
	if start > 0 {
		prev := value[start-1]
		if ('A' <= prev && prev <= 'Z') || ('0' <= prev && prev <= '9') || prev == '-' {
			return false
		}
	}
	if end < len(value) {
		next := value[end]
		if ('A' <= next && next <= 'Z') || ('0' <= next && next <= '9') || next == '-' {
			return false
		}
	}
	return true
}

func imapMailboxWireName(value string) string {
	value = strings.ToValidUTF8(value, "")
	var b strings.Builder
	lastSanitizedSpace := false
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			if !lastSanitizedSpace {
				b.WriteRune(' ')
				lastSanitizedSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastSanitizedSpace = false
	}
	return strings.TrimSpace(b.String())
}

func imapEncodeMailboxName(value string) string {
	value = strings.ToValidUTF8(value, "")
	var b strings.Builder
	var shifted []uint16
	flushShifted := func() {
		if len(shifted) == 0 {
			return
		}
		raw := make([]byte, 0, len(shifted)*2)
		for _, unit := range shifted {
			raw = append(raw, byte(unit>>8), byte(unit))
		}
		encoded := base64.RawStdEncoding.EncodeToString(raw)
		encoded = strings.ReplaceAll(encoded, "/", ",")
		b.WriteByte('&')
		b.WriteString(encoded)
		b.WriteByte('-')
		shifted = shifted[:0]
	}
	for _, r := range value {
		if r >= 0x20 && r <= 0x7e && r != '&' {
			flushShifted()
			b.WriteRune(r)
			continue
		}
		if r == '&' {
			flushShifted()
			b.WriteString("&-")
			continue
		}
		shifted = append(shifted, utf16.Encode([]rune{r})...)
	}
	flushShifted()
	return b.String()
}

func imapDecodeMailboxName(value string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(value); {
		if value[i] == '&' {
			end := strings.IndexByte(value[i+1:], '-')
			if end < 0 {
				return "", false
			}
			end += i + 1
			encoded := value[i+1 : end]
			if encoded == "" {
				b.WriteByte('&')
				i = end + 1
				continue
			}
			decoded, ok := imapDecodeMailboxBase64(encoded)
			if !ok {
				return "", false
			}
			b.WriteString(decoded)
			i = end + 1
			continue
		}
		if value[i] >= 0x80 || value[i] < 0x20 || value[i] == 0x7f {
			return "", false
		}
		b.WriteByte(value[i])
		i++
	}
	decoded := b.String()
	if strings.Contains(value, "&") && imapEncodeMailboxName(decoded) != value {
		return "", false
	}
	return decoded, true
}

func imapDecodeRequiredMailboxName(value string) (string, bool, bool) {
	name, ok := imapDecodeMailboxName(value)
	if !ok {
		return "", false, false
	}
	return name, true, name != ""
}

func imapDecodeMailboxBase64(value string) (string, bool) {
	if strings.ContainsAny(value, "&-") || len(value)%4 == 1 {
		return "", false
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.ReplaceAll(value, ",", "/"))
	if err != nil || len(raw) == 0 || len(raw)%2 != 0 {
		return "", false
	}
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i < len(raw); i += 2 {
		units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
	}
	var runes []rune
	for i := 0; i < len(units); i++ {
		unit := units[i]
		switch {
		case 0xd800 <= unit && unit <= 0xdbff:
			if i+1 >= len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
				return "", false
			}
			runes = append(runes, utf16.DecodeRune(rune(unit), rune(units[i+1])))
			i++
		case 0xdc00 <= unit && unit <= 0xdfff:
			return "", false
		default:
			runes = append(runes, rune(unit))
		}
	}
	for _, r := range runes {
		if r >= 0x20 && r <= 0x7e {
			return "", false
		}
		if r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	return string(runes), true
}

func imapListPattern(reference string, pattern string) (string, bool) {
	var ok bool
	reference, ok = imapDecodeMailboxName(reference)
	if !ok {
		return "", false
	}
	pattern, ok = imapDecodeMailboxName(pattern)
	if !ok {
		return "", false
	}
	if strings.HasPrefix(pattern, "/") {
		return strings.TrimPrefix(pattern, "/"), true
	}
	reference = strings.TrimPrefix(reference, "/")
	if reference == "" || pattern == "" {
		return pattern, true
	}
	return strings.TrimRight(reference, "/") + "/" + pattern, true
}

func imapListPatterns(fields []string) ([]string, bool) {
	if len(fields) < 2 {
		return nil, false
	}
	reference := fields[0]
	if len(fields) == 2 && !strings.HasPrefix(strings.TrimSpace(fields[1]), "(") {
		pattern, ok := imapListPattern(reference, fields[1])
		if !ok {
			return nil, false
		}
		return []string{pattern}, true
	}
	if len(fields) < 2 || !strings.HasPrefix(fields[1], "(") {
		return nil, false
	}
	raw := strings.Join(fields[1:], " ")
	if !strings.HasPrefix(raw, "(") || !strings.HasSuffix(raw, ")") || strings.HasPrefix(raw, "((") {
		return nil, false
	}
	patternFields := imapParenthesizedMailboxPatternFields(fields[1:])
	patterns := make([]string, 0, len(patternFields))
	seen := make(map[string]struct{}, len(patternFields))
	for _, patternField := range patternFields {
		if patternField == "" {
			return nil, false
		}
		pattern, ok := imapListPattern(reference, patternField)
		if !ok {
			return nil, false
		}
		key := strings.ToLower(pattern)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		patterns = append(patterns, pattern)
	}
	return patterns, len(patterns) > 0
}

func imapParenthesizedMailboxPatternFields(fields []string) []string {
	raw := strings.TrimSpace(strings.Join(fields, " "))
	if !strings.HasPrefix(raw, "(") || !strings.HasSuffix(raw, ")") || strings.HasPrefix(raw, "((") {
		return nil
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "("), ")"))
	if inner == "" {
		return nil
	}
	patterns, err := parseIMAPFields(inner)
	if err != nil || len(patterns) == 0 {
		return nil
	}
	for _, pattern := range patterns {
		if pattern == "" || strings.ContainsAny(pattern, "()") {
			return nil
		}
	}
	return patterns
}

func imapMailboxMatchesPattern(name string, pattern string) bool {
	matcher, ok := imapMailboxPatternMatcher(pattern)
	return ok && matcher(name)
}

func imapMailboxPatternMatcherAny(patterns []string) (func(string) bool, bool) {
	matchers := make([]func(string) bool, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		matcher, ok := imapMailboxPatternMatcher(pattern)
		if !ok {
			return nil, false
		}
		matchers = append(matchers, matcher)
	}
	return func(name string) bool {
		for _, matcher := range matchers {
			if matcher(name) {
				return true
			}
		}
		return false
	}, true
}

func imapMailboxPatternListContainsRoot(patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			return true
		}
	}
	return false
}

func imapMailboxPatternMatcher(pattern string) (func(string) bool, bool) {
	if pattern == "" {
		return func(name string) bool { return name == "" }, true
	}
	var b strings.Builder
	b.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteString(".*")
		case '%':
			b.WriteString(`[^/]*`)
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	compiled, err := regexp.Compile(b.String())
	if err != nil {
		return nil, false
	}
	return compiled.MatchString, true
}

