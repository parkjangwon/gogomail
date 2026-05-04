package imapgw

import "strings"

const MailboxHierarchyDelimiter = "/"

func MailboxDisplayName(mailbox Mailbox) string {
	name := strings.TrimSpace(mailbox.Name)
	if name != "" {
		return name
	}
	path := strings.Trim(strings.TrimSpace(mailbox.FullPath), MailboxHierarchyDelimiter)
	if path == "" {
		return ""
	}
	parts := strings.Split(path, MailboxHierarchyDelimiter)
	return parts[len(parts)-1]
}

func MailboxPath(mailbox Mailbox) string {
	path := strings.TrimSpace(mailbox.FullPath)
	if path != "" {
		return strings.Trim(path, MailboxHierarchyDelimiter)
	}
	return strings.Trim(strings.TrimSpace(mailbox.Name), MailboxHierarchyDelimiter)
}

func IsSelectableMailbox(mailbox Mailbox) bool {
	return MailboxDisplayName(mailbox) != ""
}

func IsSystemMailbox(mailbox Mailbox, systemType string) bool {
	return strings.EqualFold(strings.TrimSpace(mailbox.SystemType), strings.TrimSpace(systemType))
}
