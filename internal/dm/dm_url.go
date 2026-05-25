package dm

import (
	"net/url"
	"regexp"
	"strings"
)

const maxMessageURLs = 32

var urlCandidatePattern = regexp.MustCompile(`https?://[^\s<>"']+`)

func ExtractMessageURLs(body string) []string {
	matches := urlCandidatePattern.FindAllString(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, candidate := range matches {
		candidate = strings.TrimRight(candidate, ".,;:!?)]}")
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			continue
		}
		normalized := parsed.String()
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
		if len(out) >= maxMessageURLs {
			break
		}
	}
	return out
}
