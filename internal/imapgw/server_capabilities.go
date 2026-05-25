package imapgw

import (
	"bufio"
	"strings"
)

func (s *Server) handleNamespace(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 2 {
		_, err := writer.WriteString(tag + " BAD NAMESPACE does not accept arguments\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if _, err := writer.WriteString(`* NAMESPACE (("" "/")) NIL NIL` + "\r\n"); err != nil {
		return false, err
	}
	_, err := writer.WriteString(tag + " OK NAMESPACE completed\r\n")
	return false, err
}

func (s *Server) handleCapability(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 2 {
		_, err := writer.WriteString(tag + " BAD CAPABILITY does not accept arguments\r\n")
		return false, err
	}
	if _, err := writer.WriteString("* CAPABILITY " + strings.Join(s.imapCapabilities(state), " ") + "\r\n"); err != nil {
		return false, err
	}
	_, err := writer.WriteString(tag + " OK CAPABILITY completed\r\n")
	return false, err
}

func (s *Server) handleNoop(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 2 {
		_, err := writer.WriteString(tag + " BAD NOOP does not accept arguments\r\n")
		return false, err
	}
	if err := s.drainMailboxEvents(writer, state); err != nil {
		return false, err
	}
	_, err := writer.WriteString(tag + " OK NOOP completed\r\n")
	return false, err
}

func (s *Server) handleID(writer *bufio.Writer, tag string, trimmedLine string, literals []string, state *imapConnState) (bool, error) {
	if !imapIDArgumentsValidWithLiterals(imapCommandArgumentString(trimmedLine), literals) {
		_, err := writer.WriteString(tag + " BAD ID requires NIL or parameter list\r\n")
		return false, err
	}
	if _, err := writer.WriteString(`* ID ("name" "gogomail")` + "\r\n"); err != nil {
		return false, err
	}
	_, err := writer.WriteString(tag + " OK ID completed\r\n")
	return false, err
}

func (s *Server) imapCapabilities(state *imapConnState) []string {
	capabilities := []string{"IMAP4rev1", "LITERAL+", "IDLE", "ID", "NAMESPACE", "CHILDREN", "UNSELECT", "UIDPLUS", "MOVE", "CONDSTORE", "ENABLE", "SPECIAL-USE", "LIST-EXTENDED", "LIST-STATUS", "ESEARCH", "SEARCHRES", "STATUS=SIZE", "SORT", "THREAD=ORDEREDSUBJECT"}
	if state != nil && state.session == nil && !state.tlsActive && s != nil && s.options.TLSConfig != nil {
		capabilities = append(capabilities, "STARTTLS")
	}
	if state == nil || state.session == nil {
		if s.authAllowed(state) {
			capabilities = append(capabilities, "SASL-IR", "AUTH=PLAIN")
		} else {
			capabilities = append(capabilities, "LOGINDISABLED")
		}
	}
	return capabilities
}

func (s *Server) authenticatedCapabilityCode(state *imapConnState) string {
	return s.capabilityCode(state)
}

func (s *Server) capabilityCode(state *imapConnState) string {
	return "[CAPABILITY " + strings.Join(s.imapCapabilities(state), " ") + "]"
}

func (s *Server) handleEnable(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 3 {
		_, err := writer.WriteString(tag + " BAD ENABLE requires at least one capability\r\n")
		return false, err
	}
	for _, field := range fields[2:] {
		if !imapAtomValid(field) {
			_, err := writer.WriteString(tag + " BAD ENABLE capability is malformed\r\n")
			return false, err
		}
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	wasCondstoreAware := state.condstoreAware
	enableCondstore := false
	enabled := make([]string, 0, len(fields)-2)
	for _, field := range fields[2:] {
		if strings.EqualFold(field, "CONDSTORE") {
			enableCondstore = true
			state.condstoreAware = true
			if !imapStringSliceContainsFold(enabled, "CONDSTORE") {
				enabled = append(enabled, "CONDSTORE")
			}
		}
	}
	if len(enabled) == 0 {
		if _, err := writer.WriteString("* ENABLED\r\n"); err != nil {
			return false, err
		}
	} else if _, err := writer.WriteString("* ENABLED " + strings.Join(enabled, " ") + "\r\n"); err != nil {
		return false, err
	}
	if enableCondstore && !wasCondstoreAware && state.selectedMailbox != "" {
		if state.selectedHighestModSeq > 0 {
			state.selectedNoModSeq = false
			if err := writeIMAPUintLine(writer, "* OK [HIGHESTMODSEQ ", state.selectedHighestModSeq, "] Highest mod-sequence\r\n"); err != nil {
				return false, err
			}
		} else {
			state.selectedNoModSeq = true
			if _, err := writer.WriteString("* OK [NOMODSEQ] No persistent mod-sequences\r\n"); err != nil {
				return false, err
			}
		}
	}
	_, err := writer.WriteString(tag + " OK ENABLE completed\r\n")
	return false, err
}

func imapStringSliceContainsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func (s *Server) authAllowed(state *imapConnState) bool {
	if s == nil {
		return false
	}
	if s.options.AllowInsecureAuth {
		return true
	}
	return state != nil && state.tlsActive
}

func imapIDArgumentsValid(argument string) bool {
	return imapIDArgumentsValidWithLiterals(argument, nil)
}

func imapIDArgumentsValidWithLiterals(argument string, literals []string) bool {
	argument = strings.TrimSpace(argument)
	if argument == "" {
		return len(literals) == 0
	}
	if strings.EqualFold(argument, "NIL") {
		return len(literals) == 0
	}
	if len(argument) < 2 || argument[0] != '(' || argument[len(argument)-1] != ')' {
		return false
	}
	tokens, ok := imapIDListTokens(argument[1:len(argument)-1], literals)
	if !ok || len(tokens)%2 != 0 || len(tokens)/2 > 30 {
		return false
	}
	seenFields := make(map[string]struct{}, len(tokens)/2)
	for i := 0; i < len(tokens); i += 2 {
		field := tokens[i]
		value := tokens[i+1]
		if strings.EqualFold(field, "NIL") || len(field) == 0 || len(field) > 30 || len(value) > 1024 {
			return false
		}
		key := strings.ToLower(field)
		if _, ok := seenFields[key]; ok {
			return false
		}
		seenFields[key] = struct{}{}
	}
	return true
}

func imapIDListTokens(value string, literals []string) ([]string, bool) {
	tokens := make([]string, 0, 8)
	literalIndex := 0
	for i := 0; i < len(value); {
		for i < len(value) && (value[i] == ' ' || value[i] == '\t') {
			i++
		}
		if i >= len(value) {
			break
		}
		if value[i] == '"' {
			token, next, ok := imapParseQuotedToken(value, i)
			if !ok {
				return nil, false
			}
			if next < len(value) && value[next] != ' ' && value[next] != '\t' {
				return nil, false
			}
			tokens = append(tokens, token)
			i = next
			continue
		}
		start := i
		for i < len(value) && value[i] != ' ' && value[i] != '\t' {
			if value[i] == '(' || value[i] == ')' || value[i] < 0x20 || value[i] >= 0x7f {
				return nil, false
			}
			i++
		}
		token := value[start:i]
		if imapLooksLikeLiteral(token) {
			if literalIndex >= len(literals) {
				return nil, false
			}
			tokens = append(tokens, literals[literalIndex])
			literalIndex++
			continue
		}
		if !imapAtomValid(token) {
			return nil, false
		}
		tokens = append(tokens, token)
	}
	if literalIndex != len(literals) {
		return nil, false
	}
	return tokens, true
}

