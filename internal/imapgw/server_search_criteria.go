package imapgw

import "strings"

type imapSearchReturnSpec struct {
	extended bool
	save     bool
	options  []string
}

func imapSearchReturnOptions(fields []string) (imapSearchReturnSpec, []string, bool) {
	if len(fields) == 0 || !strings.EqualFold(fields[0], "RETURN") {
		return imapSearchReturnSpec{}, fields, true
	}
	if len(fields) < 2 {
		return imapSearchReturnSpec{}, nil, false
	}
	optionFields, rest, ok := imapConsumeParenthesizedFields(fields[1:])
	if !ok {
		return imapSearchReturnSpec{}, nil, false
	}
	tokens, ok := imapSearchReturnOptionTokens(optionFields)
	if !ok {
		return imapSearchReturnSpec{}, nil, false
	}
	if len(tokens) == 0 {
		tokens = []string{"ALL"}
	}
	seen := make(map[string]struct{}, len(tokens))
	options := make([]string, 0, len(tokens))
	for _, token := range tokens {
		option := strings.ToUpper(token)
		switch option {
		case "MIN", "MAX", "ALL", "COUNT", "SAVE":
		default:
			return imapSearchReturnSpec{}, nil, false
		}
		if _, ok := seen[option]; ok {
			return imapSearchReturnSpec{}, nil, false
		}
		seen[option] = struct{}{}
		if option == "SAVE" {
			continue
		}
		options = append(options, option)
	}
	_, save := seen["SAVE"]
	return imapSearchReturnSpec{extended: true, save: save, options: options}, rest, true
}

func imapSearchSaveReturnOption(fields []string) (bool, []string, bool) {
	if len(fields) == 0 || !strings.EqualFold(fields[0], "RETURN") {
		return false, fields, true
	}
	if len(fields) < 2 {
		return false, nil, false
	}
	optionFields, rest, ok := imapConsumeParenthesizedFields(fields[1:])
	if !ok {
		return false, nil, false
	}
	tokens, ok := imapSearchReturnOptionTokens(optionFields)
	if !ok {
		return false, nil, false
	}
	if len(tokens) != 1 || !strings.EqualFold(tokens[0], "SAVE") {
		return false, nil, false
	}
	return true, rest, true
}

func imapSearchReturnOptionTokens(fields []string) ([]string, bool) {
	inner, ok := imapStatusItemListInner(fields)
	if !ok {
		return nil, false
	}
	return imapParenthesizedAtomListTokens(inner)
}

func imapSearchFieldsContainCriteria(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	if strings.EqualFold(fields[0], "CHARSET") {
		return len(fields) >= 3
	}
	return true
}

func imapSearchCriteriaSyntaxError(fields []string) (string, bool) {
	criteria, charsetOK := imapSearchCriteria(fields)
	if !charsetOK {
		return "", false
	}
	if !imapSearchCriteriaSyntaxValid(criteria) {
		return "SEARCH criteria are unsupported", true
	}
	return "", false
}

func imapSearchCriteriaSyntaxValid(criteria []string) bool {
	for i := 0; i < len(criteria); {
		consumed, ok := imapSearchCriterionSyntaxConsumed(criteria[i:])
		if !ok || consumed <= 0 {
			return false
		}
		i += consumed
	}
	return true
}

func imapSearchCriterionSyntaxConsumed(criteria []string) (int, bool) {
	if len(criteria) == 0 {
		return 0, false
	}
	criterion := strings.ToUpper(criteria[0])
	switch criterion {
	case "(":
		i := 1
		seenSearchKey := false
		for i < len(criteria) {
			if criteria[i] == ")" {
				break
			}
			consumed, ok := imapSearchCriterionSyntaxConsumed(criteria[i:])
			if !ok {
				return 0, false
			}
			seenSearchKey = true
			i += consumed
		}
		if i >= len(criteria) || criteria[i] != ")" || !seenSearchKey {
			return 0, false
		}
		return i + 1, true
	case "ALL", "SEEN", "UNSEEN", "FLAGGED", "UNFLAGGED", "ANSWERED", "UNANSWERED", "DRAFT", "UNDRAFT", "DELETED", "UNDELETED", "RECENT", "OLD", "NEW":
		return 1, true
	case "NOT":
		consumed, ok := imapSearchCriterionSyntaxConsumed(criteria[1:])
		if !ok {
			return 0, false
		}
		return consumed + 1, true
	case "OR":
		leftConsumed, ok := imapSearchCriterionSyntaxConsumed(criteria[1:])
		if !ok {
			return 0, false
		}
		rightConsumed, ok := imapSearchCriterionSyntaxConsumed(criteria[1+leftConsumed:])
		if !ok {
			return 0, false
		}
		return 1 + leftConsumed + rightConsumed, true
	case "UID":
		if len(criteria) < 2 || !imapUIDSetSyntaxValid(criteria[1]) {
			return 0, false
		}
		return 2, true
	case "SINCE", "BEFORE", "ON", "SENTSINCE", "SENTBEFORE", "SENTON":
		if len(criteria) < 2 {
			return 0, false
		}
		_, ok := parseIMAPSearchDate(criteria[1])
		return 2, ok
	case "LARGER", "SMALLER":
		if len(criteria) < 2 {
			return 0, false
		}
		_, ok := parseIMAPSearchSize(criteria[1])
		return 2, ok
	case "MODSEQ":
		_, consumed, ok := parseIMAPSearchModSeq(criteria)
		return consumed, ok
	case "KEYWORD", "UNKEYWORD":
		if len(criteria) < 2 || !imapSearchKeywordValid(criteria[1]) {
			return 0, false
		}
		return 2, true
	case "FROM", "TO", "CC", "BCC", "SUBJECT", "BODY", "TEXT":
		if len(criteria) < 2 {
			return 0, false
		}
		_, ok := imapSearchStringArgument(criteria[1])
		return 2, ok
	case "HEADER":
		if len(criteria) < 3 {
			return 0, false
		}
		fieldName, ok := imapSearchStringArgument(criteria[1])
		if !ok || !imapSearchHeaderFieldNameValid(fieldName) {
			return 0, false
		}
		if _, ok := imapSearchStringArgument(criteria[2]); !ok {
			return 0, false
		}
		return 3, true
	default:
		if imapSearchCriterionLooksLikeSequenceSet(criteria[0]) {
			return 1, imapSequenceSetSyntaxValid(criteria[0])
		}
		return 0, false
	}
}

func imapSearchCriterionLooksLikeSequenceSet(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if value == "*" || value == "$" || strings.ContainsAny(value, ":,") {
		return true
	}
	first := value[0]
	return first == '+' || first == '-' || ('0' <= first && first <= '9')
}

func imapConsumeParenthesizedFields(fields []string) ([]string, []string, bool) {
	if len(fields) == 0 {
		return nil, nil, false
	}
	depth := 0
	for i, field := range fields {
		if i == 0 && !strings.HasPrefix(strings.TrimSpace(field), "(") {
			return nil, nil, false
		}
		depth += strings.Count(field, "(")
		depth -= strings.Count(field, ")")
		if depth == 0 {
			return fields[:i+1], fields[i+1:], true
		}
		if depth < 0 {
			return nil, nil, false
		}
	}
	return nil, nil, false
}

func imapSearchCriteria(criteria []string) ([]string, bool) {
	if len(criteria) >= 2 && strings.EqualFold(criteria[0], "CHARSET") {
		charset, ok := imapSupportedCharset(criteria[1])
		if !ok {
			return nil, false
		}
		switch charset {
		case "US-ASCII", "UTF-8":
			return imapNormalizeSearchCriteria(criteria[2:]), true
		}
	}
	return imapNormalizeSearchCriteria(criteria), true
}

func imapSupportedCharset(value string) (string, bool) {
	if strings.Contains(value, `"`) {
		return "", false
	}
	if strings.TrimSpace(value) != value {
		return "", false
	}
	charset := strings.ToUpper(value)
	switch charset {
	case "US-ASCII", "UTF-8":
		return charset, true
	default:
		return "", false
	}
}

func imapNormalizeSearchCriteria(criteria []string) []string {
	normalized := make([]string, 0, len(criteria))
	for _, token := range criteria {
		if token == "" {
			normalized = append(normalized, imapEmptySearchStringToken)
			continue
		}
		for strings.HasPrefix(token, "(") {
			normalized = append(normalized, "(")
			token = token[1:]
		}
		trailingGroups := 0
		for strings.HasSuffix(token, ")") {
			trailingGroups++
			token = strings.TrimSuffix(token, ")")
		}
		if token != "" {
			normalized = append(normalized, token)
		}
		for ; trailingGroups > 0; trailingGroups-- {
			normalized = append(normalized, ")")
		}
	}
	return normalized
}
