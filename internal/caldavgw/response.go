package caldavgw

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

const (
	CalendarServerNamespace = "http://calendarserver.org/ns/"
)

var (
	PropDisplayName                   = XMLName{Space: DAVNamespace, Local: "displayname"}
	PropResourceType                  = XMLName{Space: DAVNamespace, Local: "resourcetype"}
	PropCurrentUserPrincipal          = XMLName{Space: DAVNamespace, Local: "current-user-principal"}
	PropCurrentUserPrivileges         = XMLName{Space: DAVNamespace, Local: "current-user-privilege-set"}
	PropPrincipalCollectionSet        = XMLName{Space: DAVNamespace, Local: "principal-collection-set"}
	PropPrincipalURL                  = XMLName{Space: DAVNamespace, Local: "principal-URL"}
	PropOwner                         = XMLName{Space: DAVNamespace, Local: "owner"}
	PropCreationDate                  = XMLName{Space: DAVNamespace, Local: "creationdate"}
	PropGetLastModified               = XMLName{Space: DAVNamespace, Local: "getlastmodified"}
	PropGetETag                       = XMLName{Space: DAVNamespace, Local: "getetag"}
	PropGetContentType                = XMLName{Space: DAVNamespace, Local: "getcontenttype"}
	PropGetContentLength              = XMLName{Space: DAVNamespace, Local: "getcontentlength"}
	PropSyncToken                     = XMLName{Space: DAVNamespace, Local: "sync-token"}
	PropSupportedReportSet            = XMLName{Space: DAVNamespace, Local: "supported-report-set"}
	PropCalendarHomeSet               = XMLName{Space: CalDAVNamespace, Local: "calendar-home-set"}
	PropCalendarUserAddressSet        = XMLName{Space: CalDAVNamespace, Local: "calendar-user-address-set"}
	PropCalendarData                  = XMLName{Space: CalDAVNamespace, Local: "calendar-data"}
	PropCalendarDescription           = XMLName{Space: CalDAVNamespace, Local: "calendar-description"}
	PropCalendarColor                 = XMLName{Space: CalendarServerNamespace, Local: "calendar-color"}
	PropSupportedCalendarComponentSet = XMLName{Space: CalDAVNamespace, Local: "supported-calendar-component-set"}
	PropSupportedCalendarData         = XMLName{Space: CalDAVNamespace, Local: "supported-calendar-data"}
	PropMaxResourceSize               = XMLName{Space: CalDAVNamespace, Local: "max-resource-size"}
	PropScheduleInboxURL              = XMLName{Space: CalDAVNamespace, Local: "schedule-inbox-URL"}
	PropScheduleOutboxURL             = XMLName{Space: CalDAVNamespace, Local: "schedule-outbox-URL"}
	PropCalendarFreeBusySet           = XMLName{Space: CalDAVNamespace, Local: "calendar-free-busy-set"}
)

var (
	ResourceTypeCollection     = XMLName{Space: DAVNamespace, Local: "collection"}
	ResourceTypePrincipal      = XMLName{Space: DAVNamespace, Local: "principal"}
	ResourceTypeCalendar       = XMLName{Space: CalDAVNamespace, Local: "calendar"}
	ResourceTypeScheduleInbox  = XMLName{Space: CalDAVNamespace, Local: "schedule-inbox"}
	ResourceTypeScheduleOutbox = XMLName{Space: CalDAVNamespace, Local: "schedule-outbox"}
	PrivilegeRead              = XMLName{Space: DAVNamespace, Local: "read"}
	PrivilegeBind              = XMLName{Space: DAVNamespace, Local: "bind"}
	PrivilegeUnbind            = XMLName{Space: DAVNamespace, Local: "unbind"}
	PrivilegeWriteContent      = XMLName{Space: DAVNamespace, Local: "write-content"}
	PrivilegeWriteProps        = XMLName{Space: DAVNamespace, Local: "write-properties"}
)

type PropertyValue struct {
	Text               string
	Hrefs              []string
	ResourceTypes      []XMLName
	Privileges         []XMLName
	Reports            []XMLName
	CalendarComponents []string
	CalendarDataTypes  []CalendarDataType
}

func CalendarObjectDataProperty(body []byte, req CalendarDataRequest) (PropertyResult, error) {
	body, err := ProjectCalendarData(body, req)
	if err != nil {
		return PropertyResult{}, err
	}
	return PropertyResult{
		Name:  PropCalendarData,
		Value: PropertyValue{Text: string(body)},
		Found: len(body) > 0,
	}, nil
}

type PropertyResult struct {
	Name  XMLName
	Value PropertyValue
	Found bool
}

type CalendarDataType struct {
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

func PrincipalProperties(principal Principal) []PropertyResult {
	return []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: principal.DisplayName}, Found: true},
		{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection, ResourceTypePrincipal}}, Found: true},
		{Name: PropCurrentUserPrincipal, Value: PropertyValue{Hrefs: []string{principal.PrincipalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: readOnlyPrivileges()}, Found: true},
		{Name: PropPrincipalCollectionSet, Value: PropertyValue{Hrefs: []string{PrincipalsPrefix + "/"}}, Found: true},
		{Name: PropPrincipalURL, Value: PropertyValue{Hrefs: []string{principal.PrincipalPath}}, Found: true},
		{Name: PropOwner, Value: PropertyValue{Hrefs: []string{principal.PrincipalPath}}, Found: true},
		{Name: PropCalendarHomeSet, Value: PropertyValue{Hrefs: []string{principal.CalendarHomePath}}, Found: true},
		{Name: PropCalendarUserAddressSet, Value: PropertyValue{Hrefs: append([]string(nil), principal.CalendarUserAddresses...)}, Found: len(principal.CalendarUserAddresses) > 0},
		{Name: PropScheduleInboxURL, Value: PropertyValue{Hrefs: []string{principal.ScheduleInboxPath}}, Found: principal.ScheduleInboxPath != ""},
		{Name: PropScheduleOutboxURL, Value: PropertyValue{Hrefs: []string{principal.ScheduleOutboxPath}}, Found: principal.ScheduleOutboxPath != ""},
		{Name: PropCalendarFreeBusySet, Value: PropertyValue{Hrefs: []string{principal.CalendarHomePath}}, Found: principal.CalendarHomePath != ""},
	}
}

func ServiceRootProperties(principal Principal) []PropertyResult {
	return []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: "gogomail CalDAV"}, Found: true},
		{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection}}, Found: true},
		{Name: PropCurrentUserPrincipal, Value: PropertyValue{Hrefs: []string{principal.PrincipalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: readOnlyPrivileges()}, Found: true},
		{Name: PropPrincipalCollectionSet, Value: PropertyValue{Hrefs: []string{PrincipalsPrefix + "/"}}, Found: true},
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

func CalendarHomeProperties(userID string) ([]PropertyResult, error) {
	principalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	return []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: "Calendars"}, Found: true},
		{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection}}, Found: true},
		{Name: PropCurrentUserPrincipal, Value: PropertyValue{Hrefs: []string{principalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: calendarHomePrivileges()}, Found: true},
		{Name: PropOwner, Value: PropertyValue{Hrefs: []string{principalPath}}, Found: true},
	}, nil
}

func CalendarCollectionProperties(userID string, calendar Calendar, includeSyncCollection bool) ([]PropertyResult, error) {
	if _, err := CalendarCollectionPath(userID, calendar.ID); err != nil {
		return nil, err
	}
	principalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	etag, err := CalendarCollectionETag(userID, calendar)
	if err != nil {
		return nil, err
	}
	return []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: calendar.Name}, Found: true},
		{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection, ResourceTypeCalendar}}, Found: true},
		{Name: PropGetETag, Value: PropertyValue{Text: etag}, Found: true},
		webDAVTimeProperty(PropCreationDate, calendar.CreatedAt, formatWebDAVCreationDate),
		webDAVTimeProperty(PropGetLastModified, calendar.UpdatedAt, formatHTTPDate),
		{Name: PropOwner, Value: PropertyValue{Hrefs: []string{principalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: calendarCollectionPrivileges()}, Found: true},
		{Name: PropCalendarDescription, Value: PropertyValue{Text: calendar.Description}, Found: true},
		{Name: PropCalendarColor, Value: PropertyValue{Text: calendar.Color}, Found: calendar.Color != ""},
		{Name: PropSupportedCalendarComponentSet, Value: PropertyValue{CalendarComponents: []string{ComponentVEVENT, ComponentVTODO, ComponentVJOURNAL, ComponentVFREEBUSY}}, Found: true},
		{Name: PropSupportedCalendarData, Value: PropertyValue{CalendarDataTypes: []CalendarDataType{{ContentType: "text/calendar", Version: "2.0"}}}, Found: true},
		{Name: PropMaxResourceSize, Value: PropertyValue{Text: strconv.Itoa(MaxCalendarObjectBytes)}, Found: true},
		{Name: PropSyncToken, Value: PropertyValue{Text: calendar.SyncToken}, Found: true},
		{Name: PropSupportedReportSet, Value: PropertyValue{Reports: SupportedCalendarReports(includeSyncCollection)}, Found: true},
	}, nil
}

func SchedulingInboxProperties(userID string) ([]PropertyResult, error) {
	principalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	return []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: "Scheduling Inbox"}, Found: true},
		{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection, ResourceTypeScheduleInbox}}, Found: true},
		{Name: PropGetETag, Value: PropertyValue{Text: ""}, Found: true},
		{Name: PropOwner, Value: PropertyValue{Hrefs: []string{principalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: schedulingInboxPrivileges()}, Found: true},
		{Name: PropSupportedCalendarComponentSet, Value: PropertyValue{CalendarComponents: []string{ComponentVEVENT}}, Found: true},
		{Name: PropSupportedCalendarData, Value: PropertyValue{CalendarDataTypes: []CalendarDataType{{ContentType: "text/calendar", Version: "2.0"}}}, Found: true},
		{Name: PropMaxResourceSize, Value: PropertyValue{Text: strconv.Itoa(MaxCalendarObjectBytes)}, Found: true},
		{Name: PropSupportedReportSet, Value: PropertyValue{Reports: []XMLName{{Space: CalDAVNamespace, Local: string(ReportCalendarMulti)}}}, Found: true},
	}, nil
}

func SchedulingOutboxProperties(userID string) ([]PropertyResult, error) {
	principalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	return []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: "Scheduling Outbox"}, Found: true},
		{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection, ResourceTypeScheduleOutbox}}, Found: true},
		{Name: PropGetETag, Value: PropertyValue{Text: ""}, Found: true},
		{Name: PropOwner, Value: PropertyValue{Hrefs: []string{principalPath}}, Found: true},
		{Name: PropCurrentUserPrivileges, Value: PropertyValue{Privileges: schedulingOutboxPrivileges()}, Found: true},
		{Name: PropSupportedCalendarData, Value: PropertyValue{CalendarDataTypes: []CalendarDataType{{ContentType: "text/calendar", Version: "2.0"}}}, Found: true},
		{Name: PropMaxResourceSize, Value: PropertyValue{Text: strconv.Itoa(MaxCalendarObjectBytes)}, Found: true},
	}, nil
}

func SupportedCalendarReports(includeSyncCollection bool) []XMLName {
	reports := []XMLName{
		{Space: CalDAVNamespace, Local: string(ReportCalendarQuery)},
		{Space: CalDAVNamespace, Local: string(ReportCalendarMulti)},
		{Space: CalDAVNamespace, Local: string(ReportFreeBusyQuery)},
	}
	if includeSyncCollection {
		reports = append(reports, XMLName{Space: DAVNamespace, Local: string(ReportSyncCollection)})
	}
	return reports
}

func CalendarObjectProperties(userID string, object CalendarObject) ([]PropertyResult, error) {
	if _, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName); err != nil {
		return nil, err
	}
	principalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	return []PropertyResult{
		{Name: PropGetETag, Value: PropertyValue{Text: object.ETag}, Found: true},
		{Name: PropGetContentType, Value: PropertyValue{Text: "text/calendar; charset=utf-8"}, Found: true},
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

func calendarHomePrivileges() []XMLName {
	return []XMLName{PrivilegeRead, PrivilegeBind, PrivilegeUnbind}
}

func calendarCollectionPrivileges() []XMLName {
	return []XMLName{PrivilegeRead, PrivilegeBind, PrivilegeUnbind, PrivilegeWriteProps}
}

func writableObjectPrivileges() []XMLName {
	return []XMLName{PrivilegeRead, PrivilegeWriteContent}
}

func schedulingInboxPrivileges() []XMLName {
	return []XMLName{PrivilegeRead, PrivilegeWriteContent}
}

func schedulingOutboxPrivileges() []XMLName {
	return []XMLName{PrivilegeWriteContent}
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

func SelectPropfindProperties(req PropfindRequest, available []PropertyResult) []PropStatus {
	byName := make(map[XMLName]PropertyResult, len(available))
	var all []PropertyResult
	for _, prop := range available {
		if !prop.Found {
			continue
		}
		byName[prop.Name] = prop
		all = append(all, prop)
	}
	sortPropertyResults(all)

	switch req.Kind {
	case PropfindPropName:
		names := make([]PropertyResult, 0, len(all))
		for _, prop := range all {
			names = append(names, PropertyResult{Name: prop.Name, Found: true})
		}
		return []PropStatus{{StatusCode: http.StatusOK, Properties: names}}
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
			{Name: xml.Name{Local: "xmlns:C"}, Value: CalDAVNamespace},
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
			{Name: xml.Name{Local: "xmlns:C"}, Value: CalDAVNamespace},
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
	case len(prop.Value.CalendarComponents) > 0:
		for _, component := range prop.Value.CalendarComponents {
			if err := encodeCalendarComponent(enc, component); err != nil {
				return err
			}
		}
	case len(prop.Value.CalendarDataTypes) > 0:
		for _, dataType := range prop.Value.CalendarDataTypes {
			if err := encodeCalendarDataType(enc, dataType); err != nil {
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

func encodeCalendarComponent(enc *xml.Encoder, component string) error {
	component = strings.TrimSpace(strings.ToUpper(component))
	if component == "" {
		return fmt.Errorf("calendar component name is required")
	}
	start := xml.StartElement{
		Name: xml.Name{Local: "C:comp"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "name"}, Value: component}},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	return enc.EncodeToken(start.End())
}

func encodeCalendarDataType(enc *xml.Encoder, dataType CalendarDataType) error {
	contentType := strings.TrimSpace(dataType.ContentType)
	if contentType == "" {
		return fmt.Errorf("calendar data content type is required")
	}
	version := strings.TrimSpace(dataType.Version)
	if version == "" {
		version = "2.0"
	}
	start := xml.StartElement{
		Name: xml.Name{Local: "C:calendar-data"},
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
	case CalDAVNamespace:
		return "C:" + name.Local, nil
	case CalendarServerNamespace:
		return "CS:" + name.Local, nil
	default:
		return "", fmt.Errorf("unsupported XML namespace %q for %s", name.Space, name.Local)
	}
}
