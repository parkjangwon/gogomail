package carddavgw

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	PropDisplayName            = XMLName{Space: DAVNamespace, Local: "displayname"}
	PropResourceType           = XMLName{Space: DAVNamespace, Local: "resourcetype"}
	PropCurrentUserPrincipal   = XMLName{Space: DAVNamespace, Local: "current-user-principal"}
	PropCurrentUserPrivileges  = XMLName{Space: DAVNamespace, Local: "current-user-privilege-set"}
	PropPrincipalCollectionSet = XMLName{Space: DAVNamespace, Local: "principal-collection-set"}
	PropPrincipalURL           = XMLName{Space: DAVNamespace, Local: "principal-URL"}
	PropOwner                  = XMLName{Space: DAVNamespace, Local: "owner"}
	PropCreationDate           = XMLName{Space: DAVNamespace, Local: "creationdate"}
	PropGetLastModified        = XMLName{Space: DAVNamespace, Local: "getlastmodified"}
	PropGetETag                = XMLName{Space: DAVNamespace, Local: "getetag"}
	PropGetContentType         = XMLName{Space: DAVNamespace, Local: "getcontenttype"}
	PropGetContentLength       = XMLName{Space: DAVNamespace, Local: "getcontentlength"}
	PropSyncToken              = XMLName{Space: DAVNamespace, Local: "sync-token"}
	PropSupportedReportSet     = XMLName{Space: DAVNamespace, Local: "supported-report-set"}
	PropAddressBookHomeSet     = XMLName{Space: CardDAVNamespace, Local: "addressbook-home-set"}
	PropAddressData            = XMLName{Space: CardDAVNamespace, Local: "address-data"}
	PropAddressBookDescription = XMLName{Space: CardDAVNamespace, Local: "addressbook-description"}
	PropSupportedAddressData   = XMLName{Space: CardDAVNamespace, Local: "supported-address-data"}
	PropSupportedCollationSet  = XMLName{Space: CardDAVNamespace, Local: "supported-collation-set"}
	PropMaxResourceSize        = XMLName{Space: CardDAVNamespace, Local: "max-resource-size"}
	PropGetCTag                = XMLName{Space: CalendarServerNamespace, Local: "getctag"}
	PropCalendarUserType       = XMLName{Space: CardDAVNamespace, Local: "calendar-user-type"}
	PropResourceID             = XMLName{Space: DAVNamespace, Local: "resource-id"}
)

var (
	ResourceTypeCollection   = XMLName{Space: DAVNamespace, Local: "collection"}
	ResourceTypePrincipal    = XMLName{Space: DAVNamespace, Local: "principal"}
	ResourceTypeAddressBook  = XMLName{Space: CardDAVNamespace, Local: "addressbook"}
	PrivilegeRead            = XMLName{Space: DAVNamespace, Local: "read"}
	PrivilegeBind            = XMLName{Space: DAVNamespace, Local: "bind"}
	PrivilegeUnbind          = XMLName{Space: DAVNamespace, Local: "unbind"}
	PrivilegeWriteContent    = XMLName{Space: DAVNamespace, Local: "write-content"}
	PrivilegeWriteProperties = XMLName{Space: DAVNamespace, Local: "write-properties"}
)

type PropertyValue struct {
	Text               string
	AddressDataVersion string
	Hrefs              []string
	ResourceTypes      []XMLName
	Privileges         []XMLName
	Reports            []XMLName
	AddressDataTypes   []AddressDataType
	Collations         []string
}

type PropertyResult struct {
	Name            XMLName
	Value           PropertyValue
	Found           bool
	OmitFromAllProp bool
}

type AddressDataType struct {
	ContentType string
	Version     string
}

type MultiStatusResponse struct {
	Href      string
	Status    int
	PropStats []PropStatus
}

type PropStatus struct {
	StatusCode int
	Properties []PropertyResult
}

func ContactObjectDataProperty(body []byte) PropertyResult {
	prop, _ := ContactObjectDataPropertyWithProperties(body, nil)
	return prop
}

func ContactObjectDataPropertyWithProperties(body []byte, properties []string) (PropertyResult, error) {
	text := string(body)
	version, err := vCardBodyVersion(text)
	if err != nil {
		return PropertyResult{}, err
	}
	if len(properties) > 0 {
		projected, err := projectVCardProperties(text, properties)
		if err != nil {
			return PropertyResult{}, err
		}
		text = projected
	}
	return PropertyResult{
		Name:  PropAddressData,
		Value: PropertyValue{Text: text, AddressDataVersion: version},
		Found: len(body) > 0,
	}, nil
}

func vCardBodyVersion(raw string) (string, error) {
	lines, err := unfoldVCardLines(raw)
	if err != nil {
		return "", err
	}
	for _, line := range lines {
		parsed, err := parseVCardContentLineParts(line)
		if err != nil {
			return "", err
		}
		if parsed.Name == "VERSION" {
			version := strings.TrimSpace(parsed.Value)
			if version == "3.0" || version == "4.0" {
				return version, nil
			}
			return "", fmt.Errorf("vcard VERSION must be 3.0 or 4.0")
		}
	}
	return "", fmt.Errorf("vcard VERSION is required")
}

func projectVCardProperties(raw string, properties []string) (string, error) {
	wanted := make(map[string]struct{}, len(properties))
	for _, property := range properties {
		name, err := normalizeVCardName(property, "address-data property")
		if err != nil {
			return "", err
		}
		wanted[name] = struct{}{}
	}
	lines, err := unfoldVCardLines(raw)
	if err != nil {
		return "", err
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		parsed, err := parseVCardContentLineParts(line)
		if err != nil {
			return "", err
		}
		switch parsed.Name {
		case "BEGIN", "VERSION", "END":
			out = append(out, line)
		default:
			if _, ok := wanted[parsed.Name]; ok {
				out = append(out, line)
			}
		}
	}
	if len(out) == 0 {
		return "", fmt.Errorf("projected vcard is empty")
	}
	return strings.Join(out, "\r\n") + "\r\n", nil
}

func PrincipalProperties(principal Principal) []PropertyResult {
	return []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: principal.DisplayName}, Found: true},
		{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection, ResourceTypePrincipal}}, Found: true},
		{Name: PropCurrentUserPrincipal, Value: PropertyValue{Hrefs: []string{principal.PrincipalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: readOnlyPrivileges()}, Found: true},
		{Name: PropPrincipalCollectionSet, Value: PropertyValue{Hrefs: []string{PrincipalsPrefix + "/"}}, Found: true},
		{Name: PropPrincipalURL, Value: PropertyValue{Hrefs: []string{principal.PrincipalPath}}, Found: true},
		{Name: PropOwner, Value: PropertyValue{Hrefs: []string{principal.PrincipalPath}}, Found: true},
		{Name: PropAddressBookHomeSet, Value: PropertyValue{Hrefs: []string{principal.AddressBookHomePath}}, Found: true},
		{Name: PropCalendarUserType, Value: PropertyValue{Text: principal.CalendarUserType}, Found: principal.CalendarUserType != ""},
		{Name: PropResourceID, Value: PropertyValue{Hrefs: []string{principal.ResourceID}}, Found: principal.ResourceID != ""},
	}
}

func PrincipalCollectionProperties(principal Principal) []PropertyResult {
	return []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: "Principals"}, Found: true},
		{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection}}, Found: true},
		{Name: PropCurrentUserPrincipal, Value: PropertyValue{Hrefs: []string{principal.PrincipalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: readOnlyPrivileges()}, Found: true},
		{Name: PropPrincipalCollectionSet, Value: PropertyValue{Hrefs: []string{PrincipalsPrefix + "/"}}, Found: true},
	}
}

func AddressBookHomeProperties(userID string) ([]PropertyResult, error) {
	if _, err := AddressBookHomePath(userID); err != nil {
		return nil, err
	}
	principalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	return []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: "Address Books"}, Found: true},
		{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection}}, Found: true},
		{Name: PropCurrentUserPrincipal, Value: PropertyValue{Hrefs: []string{principalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: addressBookHomePrivileges()}, Found: true},
		{Name: PropOwner, Value: PropertyValue{Hrefs: []string{principalPath}}, Found: true},
	}, nil
}

func AddressBookCollectionProperties(userID string, book AddressBook, includeSyncCollection bool) ([]PropertyResult, error) {
	if _, err := AddressBookCollectionPath(userID, book.ID); err != nil {
		return nil, err
	}
	principalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	etag, err := AddressBookCollectionETag(userID, book)
	if err != nil {
		return nil, err
	}
	return []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: book.Name}, Found: true},
		{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection, ResourceTypeAddressBook}}, Found: true},
		{Name: PropGetETag, Value: PropertyValue{Text: etag}, Found: true},
		{Name: PropAddressBookDescription, Value: PropertyValue{Text: book.Description}, Found: true},
		webDAVTimeProperty(PropCreationDate, book.CreatedAt, formatWebDAVCreationDate),
		webDAVTimeProperty(PropGetLastModified, book.UpdatedAt, formatHTTPDate),
		{Name: PropOwner, Value: PropertyValue{Hrefs: []string{principalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: addressBookCollectionPrivileges()}, Found: true},
		{
			Name: PropSupportedAddressData,
			Value: PropertyValue{AddressDataTypes: []AddressDataType{
				{ContentType: "text/vcard", Version: "4.0"},
				{ContentType: "text/vcard", Version: "3.0"},
			}},
			Found:           true,
			OmitFromAllProp: true,
		},
		{Name: PropSupportedCollationSet, Value: PropertyValue{Collations: SupportedTextMatchCollations()}, Found: true, OmitFromAllProp: true},
		{Name: PropMaxResourceSize, Value: PropertyValue{Text: strconv.Itoa(MaxContactObjectBytes)}, Found: true},
		{Name: PropSyncToken, Value: PropertyValue{Text: book.SyncToken}, Found: true},
		{Name: PropGetCTag, Value: PropertyValue{Text: book.SyncToken}, Found: true},
		{Name: PropSupportedReportSet, Value: PropertyValue{Reports: SupportedAddressBookReports(includeSyncCollection)}, Found: true},
	}, nil
}

func SupportedAddressBookReports(includeSyncCollection bool) []XMLName {
	reports := []XMLName{
		{Space: CardDAVNamespace, Local: string(ReportAddressBookQuery)},
		{Space: CardDAVNamespace, Local: string(ReportAddressBookMulti)},
		{Space: CardDAVNamespace, Local: string(ReportPrincipalPropertySearch)},
	}
	if includeSyncCollection {
		reports = append(reports, XMLName{Space: DAVNamespace, Local: string(ReportSyncCollection)})
	}
	return reports
}

func ContactObjectProperties(userID string, object ContactObject) ([]PropertyResult, error) {
	if _, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName); err != nil {
		return nil, err
	}
	principalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	return []PropertyResult{
		{Name: PropGetETag, Value: PropertyValue{Text: object.ETag}, Found: true},
		{Name: PropGetContentType, Value: PropertyValue{Text: "text/vcard; charset=utf-8"}, Found: true},
		{Name: PropGetContentLength, Value: PropertyValue{Text: strconv.FormatInt(object.Size, 10)}, Found: true},
		webDAVTimeProperty(PropCreationDate, object.CreatedAt, formatWebDAVCreationDate),
		webDAVTimeProperty(PropGetLastModified, object.UpdatedAt, formatHTTPDate),
		{Name: PropOwner, Value: PropertyValue{Hrefs: []string{principalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: writableObjectPrivileges()}, Found: true},
		{Name: PropResourceType, Found: true},
	}, nil
}

func readOnlyPrivileges() []XMLName {
	return []XMLName{PrivilegeRead}
}

func addressBookHomePrivileges() []XMLName {
	return []XMLName{PrivilegeRead, PrivilegeBind, PrivilegeUnbind}
}

func addressBookCollectionPrivileges() []XMLName {
	return []XMLName{PrivilegeRead, PrivilegeBind, PrivilegeUnbind, PrivilegeWriteProperties}
}

func writableObjectPrivileges() []XMLName {
	return []XMLName{PrivilegeRead, PrivilegeWriteContent}
}

func SelectPropfindProperties(req PropfindRequest, available []PropertyResult) []PropStatus {
	byName := make(map[XMLName]PropertyResult, len(available))
	var all []PropertyResult
	var propNames []PropertyResult
	for _, prop := range available {
		if !prop.Found {
			continue
		}
		byName[prop.Name] = prop
		propNames = append(propNames, PropertyResult{Name: prop.Name, Found: true})
		if !prop.OmitFromAllProp {
			all = append(all, prop)
		}
	}
	sortPropertyResults(all)
	sortPropertyResults(propNames)

	switch req.Kind {
	case PropfindPropName:
		return []PropStatus{{StatusCode: http.StatusOK, Properties: propNames}}
	case PropfindProp:
		var found, missing []PropertyResult
		for _, name := range req.Properties {
			if prop, ok := byName[name]; ok {
				found = append(found, prop)
			} else {
				missing = append(missing, PropertyResult{Name: name, Found: false})
			}
		}
		sortPropertyResults(found)
		sortPropertyResults(missing)
		return propStatsForFoundMissing(found, missing)
	default:
		selected := append([]PropertyResult(nil), all...)
		for _, name := range req.Include {
			if prop, ok := byName[name]; ok && !containsProperty(selected, name) {
				selected = append(selected, prop)
			}
		}
		sortPropertyResults(selected)
		return []PropStatus{{StatusCode: http.StatusOK, Properties: selected}}
	}
}

func SelectReportProperties(req ReportRequest, available []PropertyResult) []PropStatus {
	return selectNamedProperties(req.Properties, available)
}

func BuildMultiStatusXML(responses []MultiStatusResponse) ([]byte, error) {
	return buildMultiStatusXML(responses, "")
}

func BuildSyncCollectionXML(responses []MultiStatusResponse, syncToken string) ([]byte, error) {
	syncToken = strings.TrimSpace(syncToken)
	if syncToken == "" {
		return nil, fmt.Errorf("sync-token is required")
	}
	return buildMultiStatusXML(responses, syncToken)
}

func BuildSyncCollectionTruncatedXML() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	root := xml.StartElement{
		Name: xml.Name{Local: "D:multistatus"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "xmlns:D"}, Value: DAVNamespace},
			{Name: xml.Name{Local: "xmlns:C"}, Value: CardDAVNamespace},
			{Name: xml.Name{Local: "xmlns:CS"}, Value: CalendarServerNamespace},
		},
	}
	if err := enc.EncodeToken(root); err != nil {
		return nil, err
	}
	if err := encodeTextElement(enc, "D:number-of-matches", "0"); err != nil {
		return nil, err
	}
	if err := enc.EncodeToken(root.End()); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildMultiStatusXML(responses []MultiStatusResponse, syncToken string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	root := xml.StartElement{
		Name: xml.Name{Local: "D:multistatus"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "xmlns:D"}, Value: DAVNamespace},
			{Name: xml.Name{Local: "xmlns:C"}, Value: CardDAVNamespace},
			{Name: xml.Name{Local: "xmlns:CS"}, Value: CalendarServerNamespace},
		},
	}
	if err := enc.EncodeToken(root); err != nil {
		return nil, err
	}
	for _, response := range responses {
		if err := encodeResponse(enc, response); err != nil {
			return nil, err
		}
	}
	if syncToken != "" {
		if err := encodeTextElement(enc, "D:sync-token", syncToken); err != nil {
			return nil, err
		}
	}
	if err := enc.EncodeToken(root.End()); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func webDAVTimeProperty(name XMLName, value time.Time, format func(time.Time) string) PropertyResult {
	if value.IsZero() {
		return PropertyResult{Name: name}
	}
	return PropertyResult{Name: name, Value: PropertyValue{Text: format(value)}, Found: true}
}

func formatWebDAVCreationDate(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func formatHTTPDate(value time.Time) string {
	return value.UTC().Format(http.TimeFormat)
}

func selectNamedProperties(names []XMLName, available []PropertyResult) []PropStatus {
	byName := make(map[XMLName]PropertyResult, len(available))
	for _, prop := range available {
		if prop.Found {
			byName[prop.Name] = prop
		}
	}
	var found, missing []PropertyResult
	for _, name := range names {
		if prop, ok := byName[name]; ok {
			found = append(found, prop)
		} else {
			missing = append(missing, PropertyResult{Name: name, Found: false})
		}
	}
	sortPropertyResults(found)
	sortPropertyResults(missing)
	return propStatsForFoundMissing(found, missing)
}

func encodeResponse(enc *xml.Encoder, response MultiStatusResponse) error {
	if strings.TrimSpace(response.Href) == "" {
		return fmt.Errorf("multistatus response href is required")
	}
	start := xml.StartElement{Name: xml.Name{Local: "D:response"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := encodeTextElement(enc, "D:href", response.Href); err != nil {
		return err
	}
	if response.Status != 0 {
		if err := encodeTextElement(enc, "D:status", "HTTP/1.1 "+strconv.Itoa(response.Status)+" "+http.StatusText(response.Status)); err != nil {
			return err
		}
		return enc.EncodeToken(start.End())
	}
	for _, propstat := range response.PropStats {
		if err := encodePropStatus(enc, propstat); err != nil {
			return err
		}
	}
	return enc.EncodeToken(start.End())
}

func encodePropStatus(enc *xml.Encoder, propstat PropStatus) error {
	if propstat.StatusCode == 0 {
		propstat.StatusCode = http.StatusOK
	}
	start := xml.StartElement{Name: xml.Name{Local: "D:propstat"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	propStart := xml.StartElement{Name: xml.Name{Local: "D:prop"}}
	if err := enc.EncodeToken(propStart); err != nil {
		return err
	}
	for _, prop := range propstat.Properties {
		if err := encodeProperty(enc, prop); err != nil {
			return err
		}
	}
	if err := enc.EncodeToken(propStart.End()); err != nil {
		return err
	}
	if err := encodeTextElement(enc, "D:status", "HTTP/1.1 "+strconv.Itoa(propstat.StatusCode)+" "+http.StatusText(propstat.StatusCode)); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func encodeProperty(enc *xml.Encoder, prop PropertyResult) error {
	name, err := prefixedName(prop.Name)
	if err != nil {
		return err
	}
	start := xml.StartElement{Name: xml.Name{Local: name}}
	if prop.Name == PropAddressData {
		version := strings.TrimSpace(prop.Value.AddressDataVersion)
		if version == "" {
			version = "4.0"
		}
		start.Attr = append(start.Attr,
			xml.Attr{Name: xml.Name{Local: "content-type"}, Value: "text/vcard"},
			xml.Attr{Name: xml.Name{Local: "version"}, Value: version},
		)
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	switch {
	case len(prop.Value.Hrefs) > 0:
		for _, href := range prop.Value.Hrefs {
			if err := encodeTextElement(enc, "D:href", href); err != nil {
				return err
			}
		}
	case len(prop.Value.ResourceTypes) > 0:
		for _, resourceType := range prop.Value.ResourceTypes {
			resourceName, err := prefixedName(resourceType)
			if err != nil {
				return err
			}
			if err := encodeEmptyElement(enc, resourceName); err != nil {
				return err
			}
		}
	case len(prop.Value.Privileges) > 0:
		for _, privilege := range prop.Value.Privileges {
			if err := encodeCurrentUserPrivilege(enc, privilege); err != nil {
				return err
			}
		}
	case len(prop.Value.Reports) > 0:
		for _, report := range prop.Value.Reports {
			if err := encodeSupportedReport(enc, report); err != nil {
				return err
			}
		}
	case len(prop.Value.AddressDataTypes) > 0:
		for _, dataType := range prop.Value.AddressDataTypes {
			if err := encodeAddressDataType(enc, dataType); err != nil {
				return err
			}
		}
	case len(prop.Value.Collations) > 0:
		for _, collation := range prop.Value.Collations {
			if err := encodeTextElement(enc, "C:supported-collation", collation); err != nil {
				return err
			}
		}
	case prop.Value.Text != "":
		if err := enc.EncodeToken(xml.CharData([]byte(prop.Value.Text))); err != nil {
			return err
		}
	}
	return enc.EncodeToken(start.End())
}

func encodeCurrentUserPrivilege(enc *xml.Encoder, privilege XMLName) error {
	privilegeName, err := prefixedName(privilege)
	if err != nil {
		return err
	}
	start := xml.StartElement{Name: xml.Name{Local: "D:privilege"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := encodeEmptyElement(enc, privilegeName); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func encodeSupportedReport(enc *xml.Encoder, report XMLName) error {
	reportName, err := prefixedName(report)
	if err != nil {
		return err
	}
	supportedStart := xml.StartElement{Name: xml.Name{Local: "D:supported-report"}}
	if err := enc.EncodeToken(supportedStart); err != nil {
		return err
	}
	reportStart := xml.StartElement{Name: xml.Name{Local: "D:report"}}
	if err := enc.EncodeToken(reportStart); err != nil {
		return err
	}
	if err := encodeEmptyElement(enc, reportName); err != nil {
		return err
	}
	if err := enc.EncodeToken(reportStart.End()); err != nil {
		return err
	}
	return enc.EncodeToken(supportedStart.End())
}

func encodeAddressDataType(enc *xml.Encoder, dataType AddressDataType) error {
	contentType := strings.TrimSpace(dataType.ContentType)
	if contentType == "" {
		return fmt.Errorf("address data content type is required")
	}
	version := strings.TrimSpace(dataType.Version)
	if version == "" {
		version = "4.0"
	}
	start := xml.StartElement{
		Name: xml.Name{Local: "C:address-data"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "content-type"}, Value: contentType},
			{Name: xml.Name{Local: "version"}, Value: version},
		},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func encodeEmptyElement(enc *xml.Encoder, name string) error {
	start := xml.StartElement{Name: xml.Name{Local: name}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func encodeTextElement(enc *xml.Encoder, name string, value string) error {
	start := xml.StartElement{Name: xml.Name{Local: name}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.CharData([]byte(value))); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func propStatsForFoundMissing(found []PropertyResult, missing []PropertyResult) []PropStatus {
	stats := make([]PropStatus, 0, 2)
	if len(found) > 0 {
		stats = append(stats, PropStatus{StatusCode: http.StatusOK, Properties: found})
	}
	if len(missing) > 0 {
		stats = append(stats, PropStatus{StatusCode: http.StatusNotFound, Properties: missing})
	}
	return stats
}

func containsProperty(properties []PropertyResult, name XMLName) bool {
	for _, prop := range properties {
		if prop.Name == name {
			return true
		}
	}
	return false
}

func sortPropertyResults(properties []PropertyResult) {
	sort.Slice(properties, func(i, j int) bool {
		if properties[i].Name.Space != properties[j].Name.Space {
			return properties[i].Name.Space < properties[j].Name.Space
		}
		return properties[i].Name.Local < properties[j].Name.Local
	})
}

func prefixedName(name XMLName) (string, error) {
	switch name.Space {
	case DAVNamespace:
		return "D:" + name.Local, nil
	case CardDAVNamespace:
		return "C:" + name.Local, nil
	case CalendarServerNamespace:
		return "CS:" + name.Local, nil
	default:
		return "", fmt.Errorf("unsupported XML namespace %q for %s", name.Space, name.Local)
	}
}
