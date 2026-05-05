package caldavgw

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBuildMultiStatusXMLRendersPropStatusEnvelope(t *testing.T) {
	t.Parallel()

	body, err := BuildMultiStatusXML([]MultiStatusResponse{{
		Href: "/caldav/principals/user-1/",
		PropStats: []PropStatus{
			{
				StatusCode: http.StatusOK,
				Properties: []PropertyResult{
					{Name: PropDisplayName, Value: PropertyValue{Text: "User One"}, Found: true},
					{Name: PropCalendarHomeSet, Value: PropertyValue{Hrefs: []string{"/caldav/calendars/user-1/"}}, Found: true},
					{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection, ResourceTypePrincipal}}, Found: true},
				},
			},
			{
				StatusCode: http.StatusNotFound,
				Properties: []PropertyResult{
					{Name: XMLName{Space: CalDAVNamespace, Local: "unknown-prop"}, Found: false},
				},
			},
		},
	}})
	if err != nil {
		t.Fatalf("BuildMultiStatusXML returned error: %v", err)
	}
	assertParseableXML(t, body)
	text := string(body)
	for _, want := range []string{
		"<D:multistatus",
		"<D:response>",
		"<D:href>/caldav/principals/user-1/</D:href>",
		"<D:status>HTTP/1.1 200 OK</D:status>",
		"<D:status>HTTP/1.1 404 Not Found</D:status>",
		"<C:calendar-home-set><D:href>/caldav/calendars/user-1/</D:href></C:calendar-home-set>",
		"<D:resourcetype><D:collection></D:collection><D:principal></D:principal></D:resourcetype>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("multistatus XML missing %q:\n%s", want, text)
		}
	}
}

func TestBuildSyncCollectionXMLRendersRootSyncToken(t *testing.T) {
	t.Parallel()

	body, err := BuildSyncCollectionXML([]MultiStatusResponse{{
		Href: "/caldav/calendars/user-1/work/event.ics",
		PropStats: []PropStatus{{
			StatusCode: http.StatusOK,
			Properties: []PropertyResult{
				{Name: PropGetETag, Value: PropertyValue{Text: `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`}, Found: true},
			},
		}},
	}}, "sync-123")
	if err != nil {
		t.Fatalf("BuildSyncCollectionXML returned error: %v", err)
	}
	assertParseableXML(t, body)
	text := string(body)
	if !strings.Contains(text, "<D:sync-token>sync-123</D:sync-token>") {
		t.Fatalf("sync-token missing:\n%s", text)
	}
	if !strings.Contains(text, "<D:href>/caldav/calendars/user-1/work/event.ics</D:href>") {
		t.Fatalf("response href missing:\n%s", text)
	}
}

func TestBuildMultiStatusXMLRendersResponseStatus(t *testing.T) {
	t.Parallel()

	body, err := BuildSyncCollectionXML([]MultiStatusResponse{{
		Href:   "/caldav/calendars/user-1/work/missing.ics",
		Status: http.StatusNotFound,
	}}, "sync-123")
	if err != nil {
		t.Fatalf("BuildSyncCollectionXML returned error: %v", err)
	}
	assertParseableXML(t, body)
	text := string(body)
	if !strings.Contains(text, "<D:status>HTTP/1.1 404 Not Found</D:status>") {
		t.Fatalf("response status missing:\n%s", text)
	}
}

func TestSelectPropfindPropertiesSeparatesFoundAndMissing(t *testing.T) {
	t.Parallel()

	available := []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: "Work"}, Found: true},
		{Name: PropSyncToken, Value: PropertyValue{Text: "sync-1"}, Found: true},
	}
	stats := SelectPropfindProperties(PropfindRequest{
		Kind: PropfindProp,
		Properties: []XMLName{
			PropDisplayName,
			{Space: CalDAVNamespace, Local: "missing"},
			PropSyncToken,
		},
	}, available)
	if len(stats) != 2 {
		t.Fatalf("stats = %+v, want 2 propstats", stats)
	}
	if stats[0].StatusCode != http.StatusOK || len(stats[0].Properties) != 2 {
		t.Fatalf("ok propstat = %+v", stats[0])
	}
	if stats[1].StatusCode != http.StatusNotFound || len(stats[1].Properties) != 1 {
		t.Fatalf("missing propstat = %+v", stats[1])
	}
}

func TestSelectPropfindPropertiesSupportsPropnameAndAllpropInclude(t *testing.T) {
	t.Parallel()

	available := []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: "Work"}, Found: true},
		{Name: PropCalendarColor, Value: PropertyValue{Text: "#AABBCC"}, Found: true},
	}
	propname := SelectPropfindProperties(PropfindRequest{Kind: PropfindPropName}, available)
	if len(propname) != 1 || len(propname[0].Properties) != 2 {
		t.Fatalf("propname stats = %+v", propname)
	}
	if propname[0].Properties[0].Value.Text != "" {
		t.Fatalf("propname should render empty property values: %+v", propname[0].Properties[0])
	}

	allprop := SelectPropfindProperties(PropfindRequest{
		Kind:    PropfindAllProp,
		Include: []XMLName{PropCalendarColor},
	}, available)
	if len(allprop) != 1 || len(allprop[0].Properties) != 2 {
		t.Fatalf("allprop stats = %+v", allprop)
	}
}

func TestCalendarCollectionPropertiesExposeCalDAVDiscovery(t *testing.T) {
	t.Parallel()

	props, err := CalendarCollectionProperties("user-1", Calendar{
		ID:          "work",
		UserID:      "user-1",
		Name:        "Work",
		Color:       "#AABBCC",
		Description: "Team calendar",
		SyncToken:   "sync-123",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("CalendarCollectionProperties returned error: %v", err)
	}
	body, err := BuildMultiStatusXML([]MultiStatusResponse{{
		Href:      "/caldav/calendars/user-1/work/",
		PropStats: SelectPropfindProperties(PropfindRequest{Kind: PropfindAllProp}, props),
	}})
	if err != nil {
		t.Fatalf("BuildMultiStatusXML returned error: %v", err)
	}
	assertParseableXML(t, body)
	text := string(body)
	for _, want := range []string{
		"<C:calendar></C:calendar>",
		"<C:supported-calendar-component-set>",
		"<C:comp name=\"VEVENT\"></C:comp>",
		"<C:supported-calendar-data><C:calendar-data content-type=\"text/calendar\" version=\"2.0\"></C:calendar-data></C:supported-calendar-data>",
		"<C:max-resource-size>10485760</C:max-resource-size>",
		"<D:sync-token>sync-123</D:sync-token>",
		"<D:supported-report-set>",
		"<C:calendar-query></C:calendar-query>",
		"<C:calendar-multiget></C:calendar-multiget>",
		"<C:free-busy-query></C:free-busy-query>",
		"<D:sync-collection></D:sync-collection>",
		"<CS:calendar-color>#AABBCC</CS:calendar-color>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("calendar collection XML missing %q:\n%s", want, text)
		}
	}
}

func TestSupportedCalendarReportsMatchesImplementedReportHandlers(t *testing.T) {
	t.Parallel()

	reports := SupportedCalendarReports()
	want := []XMLName{
		{Space: CalDAVNamespace, Local: "calendar-query"},
		{Space: CalDAVNamespace, Local: "calendar-multiget"},
		{Space: CalDAVNamespace, Local: "free-busy-query"},
		{Space: DAVNamespace, Local: "sync-collection"},
	}
	if len(reports) != len(want) {
		t.Fatalf("reports = %+v, want %+v", reports, want)
	}
	for i := range want {
		if reports[i] != want[i] {
			t.Fatalf("reports[%d] = %+v, want %+v", i, reports[i], want[i])
		}
	}
}

func TestPrincipalPropertiesExposeDiscoveryChain(t *testing.T) {
	t.Parallel()

	props := PrincipalProperties(Principal{
		UserID:           "user-1",
		DisplayName:      "User One",
		CalendarHomePath: "/caldav/calendars/user-1/",
		PrincipalPath:    "/caldav/principals/user-1/",
	})
	stats := SelectPropfindProperties(PropfindRequest{
		Kind: PropfindProp,
		Properties: []XMLName{
			PropCurrentUserPrincipal,
			PropPrincipalCollectionSet,
			PropPrincipalURL,
			PropCalendarHomeSet,
		},
	}, props)
	body, err := BuildMultiStatusXML([]MultiStatusResponse{{Href: "/caldav/principals/user-1/", PropStats: stats}})
	if err != nil {
		t.Fatalf("BuildMultiStatusXML returned error: %v", err)
	}
	assertParseableXML(t, body)
	text := string(body)
	for _, want := range []string{
		"<D:current-user-principal><D:href>/caldav/principals/user-1/</D:href></D:current-user-principal>",
		"<D:principal-collection-set><D:href>/caldav/principals/</D:href></D:principal-collection-set>",
		"<D:principal-URL><D:href>/caldav/principals/user-1/</D:href></D:principal-URL>",
		"<C:calendar-home-set><D:href>/caldav/calendars/user-1/</D:href></C:calendar-home-set>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("principal XML missing %q:\n%s", want, text)
		}
	}
}

func TestCalendarObjectPropertiesExposeObjectDiscovery(t *testing.T) {
	t.Parallel()

	props, err := CalendarObjectProperties("user-1", CalendarObject{
		ID:         "object-1",
		UserID:     "user-1",
		CalendarID: "work",
		ObjectName: "event.ics",
		ETag:       `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
		Size:       128,
	})
	if err != nil {
		t.Fatalf("CalendarObjectProperties returned error: %v", err)
	}
	body, err := BuildMultiStatusXML([]MultiStatusResponse{{
		Href:      "/caldav/calendars/user-1/work/event.ics",
		PropStats: SelectPropfindProperties(PropfindRequest{Kind: PropfindAllProp}, props),
	}})
	if err != nil {
		t.Fatalf("BuildMultiStatusXML returned error: %v", err)
	}
	assertParseableXML(t, body)
	text := string(body)
	for _, want := range []string{
		"<D:getetag>&#34;0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&#34;</D:getetag>",
		"<D:getcontenttype>text/calendar; charset=utf-8</D:getcontenttype>",
		"<D:getcontentlength>128</D:getcontentlength>",
		"<D:resourcetype></D:resourcetype>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("object XML missing %q:\n%s", want, text)
		}
	}
}

func TestBuildMultiStatusXMLRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := BuildMultiStatusXML([]MultiStatusResponse{{PropStats: []PropStatus{{StatusCode: http.StatusOK}}}}); err == nil {
		t.Fatal("BuildMultiStatusXML accepted empty href")
	}
	if _, err := BuildMultiStatusXML([]MultiStatusResponse{{
		Href:      "/x",
		PropStats: []PropStatus{{Properties: []PropertyResult{{Name: XMLName{Space: "urn:unknown", Local: "prop"}}}}},
	}}); err == nil {
		t.Fatal("BuildMultiStatusXML accepted unsupported namespace")
	}
}

func assertParseableXML(t *testing.T, body []byte) {
	t.Helper()

	dec := xml.NewDecoder(bytes.NewReader(body))
	for {
		if _, err := dec.Token(); err != nil {
			if err == io.EOF {
				return
			}
			t.Fatalf("XML is not parseable: %v\n%s", err, string(body))
		}
	}
}
