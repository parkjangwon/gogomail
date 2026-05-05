package carddavgw

import "time"

const (
	RFCWebDAV           = "RFC 4918"
	RFCCardDAV          = "RFC 6352"
	RFCVCard            = "RFC 6350"
	RFCCardDAVDiscovery = "RFC 6764"
)

const (
	MethodOptions   = "OPTIONS"
	MethodPropfind  = "PROPFIND"
	MethodProppatch = "PROPPATCH"
	MethodReport    = "REPORT"
	MethodMkcol     = "MKCOL"
	MethodGet       = "GET"
	MethodHead      = "HEAD"
	MethodPut       = "PUT"
	MethodDelete    = "DELETE"
)

const (
	DAVClass1         = "1"
	DAVClass3         = "3"
	DAVAddressBook    = "addressbook"
	DAVSyncCollection = "sync-collection"
)

type ResourceKind string

const (
	ResourceUnknown               ResourceKind = "unknown"
	ResourceWellKnown             ResourceKind = "well_known"
	ResourceRoot                  ResourceKind = "root"
	ResourcePrincipal             ResourceKind = "principal"
	ResourceAddressBookHome       ResourceKind = "addressbook_home"
	ResourceAddressBookCollection ResourceKind = "addressbook_collection"
	ResourceContactObject         ResourceKind = "contact_object"
)

type ResourcePath struct {
	Kind          ResourceKind
	UserID        string
	AddressBookID string
	ObjectName    string
}

type Principal struct {
	UserID              string
	DisplayName         string
	PrincipalPath       string
	AddressBookHomePath string
}

type AddressBook struct {
	ID          string
	UserID      string
	Name        string
	Description string
	SyncToken   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ContactObject struct {
	ID            string
	UserID        string
	AddressBookID string
	ObjectName    string
	UID           string
	ETag          string
	Size          int64
	VCard         []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AddressBookChange struct {
	ID            int64
	UserID        string
	AddressBookID string
	ObjectName    string
	ETag          string
	Action        string
	SyncToken     string
	ChangedAt     time.Time
}

func Standards() []string {
	return []string{
		RFCWebDAV,
		RFCCardDAV,
		RFCVCard,
		RFCCardDAVDiscovery,
	}
}

func AdvertisedDAVTokens(includeSync bool) []string {
	tokens := []string{DAVClass1, DAVClass3, DAVAddressBook}
	if includeSync {
		tokens = append(tokens, DAVSyncCollection)
	}
	return tokens
}
