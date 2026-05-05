package imapgw

import "strings"

const (
	FlagSeen     = `\Seen`
	FlagFlagged  = `\Flagged`
	FlagAnswered = `\Answered`
	FlagDraft    = `\Draft`
	FlagDeleted  = `\Deleted`
)

type MessageFlags struct {
	Read      bool
	Starred   bool
	Answered  bool
	Forwarded bool
	Draft     bool
	Deleted   bool
	Status    string
}

func (f MessageFlags) IMAPFlags() []string {
	return MapMessageFlags(f)
}

func MapMessageFlags(flags MessageFlags) []string {
	imapFlags := make([]string, 0, 4)
	if flags.Read {
		imapFlags = append(imapFlags, FlagSeen)
	}
	if flags.Starred {
		imapFlags = append(imapFlags, FlagFlagged)
	}
	if flags.Answered {
		imapFlags = append(imapFlags, FlagAnswered)
	}
	if flags.Draft || strings.EqualFold(strings.TrimSpace(flags.Status), "draft") {
		imapFlags = append(imapFlags, FlagDraft)
	}
	if flags.Deleted {
		imapFlags = append(imapFlags, FlagDeleted)
	}
	return imapFlags
}

func ApplyIMAPFlag(flags MessageFlags, imapFlag string, value bool) (MessageFlags, bool) {
	switch CanonicalIMAPFlag(imapFlag) {
	case FlagSeen:
		flags.Read = value
	case FlagFlagged:
		flags.Starred = value
	case FlagAnswered:
		flags.Answered = value
	case FlagDraft:
		flags.Draft = value
		if value {
			flags.Status = "draft"
		} else if strings.EqualFold(strings.TrimSpace(flags.Status), "draft") {
			flags.Status = ""
		}
	case FlagDeleted:
		flags.Deleted = value
	default:
		return flags, false
	}
	return flags, true
}

func MailFlagForIMAPFlag(imapFlag string) (string, bool) {
	switch CanonicalIMAPFlag(imapFlag) {
	case FlagSeen:
		return "read", true
	case FlagFlagged:
		return "starred", true
	case FlagAnswered:
		return "answered", true
	default:
		return "", false
	}
}

func CanonicalIMAPFlag(flag string) string {
	switch strings.ToLower(strings.TrimSpace(flag)) {
	case `\seen`:
		return FlagSeen
	case `\flagged`:
		return FlagFlagged
	case `\answered`:
		return FlagAnswered
	case `\draft`:
		return FlagDraft
	case `\deleted`:
		return FlagDeleted
	default:
		return strings.TrimSpace(flag)
	}
}
