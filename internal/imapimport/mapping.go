// Package imapimport implements a source-IMAP mailbox importer used to onboard
// tenants migrating from Gmail/Workspace, Outlook/Microsoft 365, Fastmail, or any
// other RFC 3501/9051 IMAP server into a gogomail user mailbox.
//
// The package is split into three layers so the risky/network parts stay small
// and the decision logic stays pure and unit-testable:
//
//   - mapping.go   — pure folder-name mapping, IMAP flag translation, and
//     Message-ID idempotency-key derivation (no I/O).
//   - client.go    — the SourceClient interface plus a minimal, dependency-free
//     IMAP client implemented over crypto/tls + net.
//   - importer.go  — the concurrent, resumable, rate-limited import loop that
//     glues a SourceClient to a MessageSink and ProgressStore.
package imapimport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/gogomail/gogomail/internal/imapgw"
)

// MappingStrategy selects how source IMAP folder hierarchies are projected onto
// gogomail's flat, per-user folder model.
type MappingStrategy string

const (
	// MappingPrefix keeps each source folder as its own destination folder,
	// flattening the hierarchy separator into a single readable name. For
	// example, "Work/Projects" (separator "/") becomes "Work/Projects" as a
	// single folder name, optionally with a configured prefix prepended.
	MappingPrefix MappingStrategy = "prefix"

	// MappingMerge collapses every source folder into a single destination
	// folder (default: INBOX). Useful when the operator only wants the mail,
	// not the original organization.
	MappingMerge MappingStrategy = "merge"

	// MappingFlatLabels flattens each source folder to its leaf name and emits
	// the full source path as a keyword/label on every message so no
	// organizational signal is lost even though folders are flattened.
	MappingFlatLabels MappingStrategy = "flat-labels"
)

// ParseMappingStrategy validates and normalizes a mapping strategy string.
func ParseMappingStrategy(raw string) (MappingStrategy, error) {
	switch MappingStrategy(strings.ToLower(strings.TrimSpace(raw))) {
	case MappingPrefix:
		return MappingPrefix, nil
	case MappingMerge:
		return MappingMerge, nil
	case MappingFlatLabels:
		return MappingFlatLabels, nil
	case "":
		return MappingPrefix, nil
	default:
		return "", fmt.Errorf("unknown mapping strategy %q; valid: prefix, merge, flat-labels", raw)
	}
}

// MappingConfig parameterizes folder mapping.
type MappingConfig struct {
	Strategy MappingStrategy
	// Separator is the source server's hierarchy delimiter (from LIST), e.g.
	// "/" for Gmail/Fastmail or "." for some Dovecot servers. Defaults to "/".
	Separator string
	// Prefix, when set, is prepended to every destination folder name for the
	// prefix strategy (e.g. "Imported"). Ignored for merge.
	Prefix string
	// MergeTarget is the destination folder for the merge strategy. Defaults to
	// "INBOX".
	MergeTarget string
}

func (c MappingConfig) separator() string {
	if strings.TrimSpace(c.Separator) == "" {
		return "/"
	}
	return c.Separator
}

// FolderMapping is the result of projecting one source folder.
type FolderMapping struct {
	// Destination is the gogomail folder name that messages should land in.
	Destination string
	// Label, when non-empty, is added as a keyword on every message from this
	// source folder (flat-labels strategy). Empty for prefix/merge.
	Label string
	// Skip is true when the folder should not be imported at all (e.g. Gmail's
	// virtual "[Gmail]/All Mail" duplicate container is best skipped to avoid
	// importing every message twice).
	Skip bool
}

// wellKnownSkip lists source folder full-paths (lowercased) that are virtual
// containers or duplicate every message; importing them causes duplication.
// The special-use \All mailbox (Gmail "All Mail") is the canonical example.
var wellKnownSkip = map[string]struct{}{
	"[gmail]/all mail":     {},
	"[gmail]/important":    {},
	"[google mail]/all mail": {},
}

// MapFolder projects a single source folder path onto a destination folder name
// (and optional label) according to the configured strategy. The sourcePath is
// the raw mailbox name as returned by the source server's LIST command.
func MapFolder(cfg MappingConfig, sourcePath string) FolderMapping {
	sep := cfg.separator()
	trimmed := strings.Trim(strings.TrimSpace(sourcePath), sep)

	if _, skip := wellKnownSkip[strings.ToLower(trimmed)]; skip {
		return FolderMapping{Skip: true}
	}

	segments := splitNonEmpty(trimmed, sep)
	leaf := ""
	if len(segments) > 0 {
		leaf = segments[len(segments)-1]
	}

	// INBOX is always the inbox regardless of strategy (RFC 3501 §5.1: INBOX is
	// case-insensitive and special).
	if strings.EqualFold(trimmed, "INBOX") {
		return FolderMapping{Destination: "INBOX"}
	}

	switch cfg.Strategy {
	case MappingMerge:
		target := strings.TrimSpace(cfg.MergeTarget)
		if target == "" {
			target = "INBOX"
		}
		return FolderMapping{Destination: target}

	case MappingFlatLabels:
		dest := leaf
		if dest == "" {
			dest = "INBOX"
		}
		label := strings.Join(segments, "/")
		return FolderMapping{Destination: sanitizeFolderName(dest), Label: label}

	default: // MappingPrefix
		joined := strings.Join(segments, "/")
		if joined == "" {
			joined = "INBOX"
		}
		if p := strings.TrimSpace(cfg.Prefix); p != "" && !strings.EqualFold(joined, "INBOX") {
			joined = strings.Trim(p, "/") + "/" + joined
		}
		return FolderMapping{Destination: sanitizeFolderName(joined)}
	}
}

func splitNonEmpty(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// sanitizeFolderName removes characters gogomail's folder model rejects
// (path separators are collapsed to a space, CR/LF stripped) while keeping the
// name human-readable. gogomail stores folders flat, so a mapped name like
// "Work/Projects" is a single literal folder name — we keep the slash as a
// visual separator but strip control characters.
func sanitizeFolderName(name string) string {
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.TrimSpace(name)
	if name == "" {
		return "INBOX"
	}
	return name
}

// TranslateFlags converts a set of source IMAP flags (e.g. "\Seen", "\Flagged",
// "\Answered", "$Forwarded", custom keywords) into gogomail's MessageFlags.
// It builds on imapgw.ApplyIMAPFlag so translation stays consistent with the
// server's own flag handling (RFC 3501 §2.3.2 system flags + keywords).
//
// extraKeywords are appended as message keywords (used to carry the source
// folder label under the flat-labels strategy). "\Recent" is intentionally
// ignored: it is session-scoped and not persistable (RFC 3501 §2.3.2).
func TranslateFlags(sourceFlags []string, extraKeywords ...string) imapgw.MessageFlags {
	var flags imapgw.MessageFlags
	keywords := make([]string, 0, len(sourceFlags)+len(extraKeywords))

	for _, raw := range sourceFlags {
		f := strings.TrimSpace(raw)
		if f == "" || strings.EqualFold(f, `\Recent`) {
			continue
		}
		if updated, ok := imapgw.ApplyIMAPFlag(flags, f, true); ok {
			flags = updated
			continue
		}
		// Not a system flag: treat as a keyword if it is a valid IMAP atom.
		if imapgw.IMAPKeywordFlagValid(f) {
			keywords = append(keywords, f)
		}
	}

	keywords = append(keywords, extraKeywords...)
	flags.Keywords = imapgw.CanonicalIMAPKeywords(keywords)
	return flags
}

// IdempotencyKey derives a stable key for a message so re-runs can skip messages
// that were already imported. When the message carries an RFC 5322 Message-ID
// header it is used verbatim (angle brackets and surrounding whitespace
// stripped, case preserved — Message-IDs are case-sensitive per RFC 5322 §3.6.4).
//
// When no Message-ID is present, callers should fall back to a content hash via
// ContentIdempotencyKey so those messages are still de-duplicated.
func IdempotencyKey(messageID string) string {
	id := strings.TrimSpace(messageID)
	id = strings.TrimPrefix(id, "<")
	id = strings.TrimSuffix(id, ">")
	return strings.TrimSpace(id)
}

// ContentIdempotencyKey returns a fallback idempotency key derived from the raw
// message bytes for messages that lack a usable Message-ID. It is prefixed so it
// never collides with a real Message-ID.
func ContentIdempotencyKey(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// DeriveIdempotencyKey picks the Message-ID when present, otherwise a content
// hash of the raw message.
func DeriveIdempotencyKey(messageID string, raw []byte) string {
	if k := IdempotencyKey(messageID); k != "" {
		return k
	}
	return ContentIdempotencyKey(raw)
}

// sortedKeys is a small helper used by tests and progress serialization to make
// output deterministic.
func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
