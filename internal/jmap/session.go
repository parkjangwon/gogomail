package jmap

import "fmt"

// JMAP capability URIs (RFC 8620 §2, RFC 8621 §2).
const (
	CapabilityCore             = "urn:ietf:params:jmap:core"
	CapabilityMail             = "urn:ietf:params:jmap:mail"
	CapabilitySubmission       = "urn:ietf:params:jmap:submission"
	CapabilityVacationResponse = "urn:ietf:params:jmap:vacationresponse"
)

// BuildSession constructs a Session for the given user+account.
// Called from the handler to build the response to GET /.well-known/jmap.
// apiBase should be the public base URL of the server (e.g. "https://mail.example.com").
func BuildSession(username, accountID, apiBase string) *Session {
	coreCaps := map[string]interface{}{
		// RFC 8620 §2: maxSizeUpload, maxConcurrentUpload, maxSizeRequest,
		// maxConcurrentRequests, maxCallsInRequest, maxObjectsInGet,
		// maxObjectsInSet, collationAlgorithms are required fields.
		"maxSizeUpload":          50_000_000,
		"maxConcurrentUpload":    4,
		"maxSizeRequest":         10_000_000,
		"maxConcurrentRequests":  4,
		"maxCallsInRequest":      16,
		"maxObjectsInGet":        500,
		"maxObjectsInSet":        500,
		"collationAlgorithms":    []string{"i;ascii-numeric", "i;ascii-casemap", "i;unicode-casemap"},
	}

	mailCaps := map[string]interface{}{
		// RFC 8621 §2: required fields.
		"maxMailboxesPerEmail":        nil,
		"maxMailboxDepth":             nil,
		"maxSizeMailboxName":          200,
		"maxDescendantMailboxes":      nil,
		"emailQuerySortOptions":       []string{"receivedAt", "from", "to", "subject", "size"},
		"mayCreateTopLevelMailbox":    true,
	}

	accountCaps := map[string]interface{}{
		CapabilityCore:             map[string]interface{}{},
		CapabilityMail:             mailCaps,
		CapabilitySubmission:       map[string]interface{}{},
		CapabilityVacationResponse: map[string]interface{}{},
	}

	return &Session{
		Capabilities: map[string]interface{}{
			CapabilityCore:             coreCaps,
			CapabilityMail:             map[string]interface{}{},
			CapabilitySubmission:       map[string]interface{}{},
			CapabilityVacationResponse: map[string]interface{}{},
		},
		Accounts: map[string]Account{
			accountID: {
				Name:                username,
				IsPersonal:          true,
				IsReadOnly:          false,
				AccountCapabilities: accountCaps,
			},
		},
		PrimaryAccounts: map[string]string{
			CapabilityCore:             accountID,
			CapabilityMail:             accountID,
			CapabilitySubmission:       accountID,
			CapabilityVacationResponse: accountID,
		},
		Username:       username,
		APIUrl:         fmt.Sprintf("%s/jmap/api", apiBase),
		DownloadUrl:    fmt.Sprintf("%s/jmap/download/{accountId}/{blobId}/{name}?accept={type}", apiBase),
		UploadUrl:      fmt.Sprintf("%s/jmap/upload/{accountId}/", apiBase),
		EventSourceUrl: fmt.Sprintf("%s/jmap/eventsource/?types={types}&closeafter={closeafter}&ping={ping}", apiBase),
		State:          "state-v1",
	}
}
