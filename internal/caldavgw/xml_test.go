package caldavgw

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseDepth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value    string
		fallback Depth
		want     Depth
	}{
		{value: "", fallback: DepthOne, want: DepthOne},
		{value: "0", want: DepthZero},
		{value: "1", want: DepthOne},
		{value: "infinity", want: DepthInfinity},
		{value: " Infinity ", want: DepthInfinity},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.value, func(t *testing.T) {
			t.Parallel()

			got, err := ParseDepth(tc.value, tc.fallback)
			if err != nil {
				t.Fatalf("ParseDepth(%q) returned error: %v", tc.value, err)
			}
			if got != tc.want {
				t.Fatalf("ParseDepth(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestParseDepthRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "2", "0,1", "1\nX-Other: bad"} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseDepth(value, ""); err == nil {
				t.Fatalf("ParseDepth(%q) error = nil, want rejection", value)
			}
		})
	}
}

func TestParsePropfindEmptyBodyDefaultsToAllProp(t *testing.T) {
	t.Parallel()

	req, err := ParsePropfind(strings.NewReader(" \n\t "))
	if err != nil {
		t.Fatalf("ParsePropfind returned error: %v", err)
	}
	if req.Kind != PropfindAllProp {
		t.Fatalf("kind = %q, want %q", req.Kind, PropfindAllProp)
	}
}

func TestParsePropfindPropRequest(t *testing.T) {
	t.Parallel()

	const body = `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:displayname/>
    <C:calendar-home-set/>
    <D:resourcetype><D:collection/></D:resourcetype>
  </D:prop>
</D:propfind>`
	req, err := ParsePropfind(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParsePropfind returned error: %v", err)
	}
	if req.Kind != PropfindProp {
		t.Fatalf("kind = %q, want %q", req.Kind, PropfindProp)
	}
	want := []XMLName{
		{Space: DAVNamespace, Local: "displayname"},
		{Space: CalDAVNamespace, Local: "calendar-home-set"},
		{Space: DAVNamespace, Local: "resourcetype"},
	}
	if len(req.Properties) != len(want) {
		t.Fatalf("properties = %+v, want %+v", req.Properties, want)
	}
	for i := range want {
		if req.Properties[i] != want[i] {
			t.Fatalf("property %d = %+v, want %+v", i, req.Properties[i], want[i])
		}
	}
}

func TestParsePropfindAllPropInclude(t *testing.T) {
	t.Parallel()

	const body = `<propfind xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <allprop/>
  <include>
    <C:calendar-color/>
    <C:supported-calendar-component-set/>
  </include>
</propfind>`
	req, err := ParsePropfind(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParsePropfind returned error: %v", err)
	}
	if req.Kind != PropfindAllProp {
		t.Fatalf("kind = %q, want %q", req.Kind, PropfindAllProp)
	}
	if len(req.Include) != 2 {
		t.Fatalf("include = %+v, want 2 properties", req.Include)
	}
	if req.Include[0] != (XMLName{Space: CalDAVNamespace, Local: "calendar-color"}) {
		t.Fatalf("first include = %+v", req.Include[0])
	}
}

func TestParsePropfindRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"wrong root":       `<D:foo xmlns:D="DAV:"><D:allprop/></D:foo>`,
		"wrong namespace":  `<propfind><allprop/></propfind>`,
		"duplicate modes":  `<D:propfind xmlns:D="DAV:"><D:allprop/><D:propname/></D:propfind>`,
		"include no mode":  `<D:propfind xmlns:D="DAV:"><D:include><D:getetag/></D:include></D:propfind>`,
		"empty prop":       `<D:propfind xmlns:D="DAV:"><D:prop/></D:propfind>`,
		"malformed xml":    `<D:propfind xmlns:D="DAV:"><D:allprop></D:propfind>`,
		"multiple roots":   `<D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind><D:propfind xmlns:D="DAV:"/>`,
		"xml directive":    `<!DOCTYPE propfind><D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`,
		"too much nesting": `<D:propfind xmlns:D="DAV:"><D:allprop>` + strings.Repeat("<D:x>", MaxWebDAVXMLDepth+1) + strings.Repeat("</D:x>", MaxWebDAVXMLDepth+1) + `</D:allprop></D:propfind>`,
	}
	for name, body := range tests {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParsePropfind(strings.NewReader(body)); err == nil {
				t.Fatal("ParsePropfind error = nil, want rejection")
			}
		})
	}
}

func TestParsePropfindRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	if _, err := ParsePropfind(strings.NewReader(strings.Repeat("x", MaxWebDAVXMLBodyBytes+1))); err == nil {
		t.Fatal("ParsePropfind accepted oversized body")
	}
}

func TestParseMKCalendarCollectsCreationProperties(t *testing.T) {
	t.Parallel()

	const body = `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:set>
    <D:prop>
      <D:displayname> Team Calendar </D:displayname>
      <C:calendar-description> Milestones </C:calendar-description>
      <CS:calendar-color> #aabbcc </CS:calendar-color>
    </D:prop>
  </D:set>
</C:mkcalendar>`
	req, err := ParseMKCalendar(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseMKCalendar returned error: %v", err)
	}
	if req.DisplayName != "Team Calendar" || req.Description != "Milestones" || req.Color != "#aabbcc" {
		t.Fatalf("request = %+v", req)
	}
	wantProperties := []XMLName{PropDisplayName, PropCalendarDescription, PropCalendarColor}
	if !reflect.DeepEqual(req.Properties, wantProperties) {
		t.Fatalf("properties = %+v, want %+v", req.Properties, wantProperties)
	}
}

func TestParseMKCalendarCollectsUnsupportedProperties(t *testing.T) {
	t.Parallel()

	const body = `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:set>
    <D:prop>
      <D:displayname> Team Calendar </D:displayname>
      <C:unknown>unsupported</C:unknown>
    </D:prop>
  </D:set>
</C:mkcalendar>`
	req, err := ParseMKCalendar(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseMKCalendar returned error: %v", err)
	}
	if len(req.Unsupported) != 1 || req.Unsupported[0] != (XMLName{Space: CalDAVNamespace, Local: "unknown"}) {
		t.Fatalf("unsupported = %+v", req.Unsupported)
	}
}

func TestParseMKCalendarAllowsEmptyBody(t *testing.T) {
	t.Parallel()

	req, err := ParseMKCalendar(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseMKCalendar returned error: %v", err)
	}
	if !reflect.DeepEqual(req, MKCalendarRequest{}) {
		t.Fatalf("request = %+v, want empty", req)
	}
}

func TestParseMKCalendarRejectsWhitespaceOnlyBody(t *testing.T) {
	t.Parallel()

	if _, err := ParseMKCalendar(strings.NewReader(" \r\n\t ")); err == nil {
		t.Fatal("ParseMKCalendar error = nil, want rejection")
	}
}

func TestParseMKCalendarRejectsNonEmptyBodyWithoutSet(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"empty mkcalendar": `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav"/>`,
		"unknown child":    `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav"><C:unknown/></C:mkcalendar>`,
		"unknown set child": `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:set><C:unknown/></D:set>
</C:mkcalendar>`,
	}
	for name, body := range tests {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseMKCalendar(strings.NewReader(body)); err == nil {
				t.Fatal("ParseMKCalendar error = nil, want rejection")
			}
		})
	}
}

func TestParseMKCalendarRejectsWrongRoot(t *testing.T) {
	t.Parallel()

	if _, err := ParseMKCalendar(strings.NewReader(`<D:propfind xmlns:D="DAV:"/>`)); err == nil {
		t.Fatal("ParseMKCalendar accepted wrong root")
	}
}

func TestParseProppatchCollectsCalendarCollectionProperties(t *testing.T) {
	t.Parallel()

	const body = `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/">
  <D:set>
    <D:prop>
      <D:displayname> Product </D:displayname>
      <C:calendar-description> Launch dates </C:calendar-description>
      <CS:calendar-color> #112233 </CS:calendar-color>
    </D:prop>
  </D:set>
</D:propertyupdate>`
	req, err := ParseProppatch(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseProppatch returned error: %v", err)
	}
	if req.Name == nil || *req.Name != "Product" {
		t.Fatalf("name = %v", req.Name)
	}
	if req.Description == nil || *req.Description != "Launch dates" {
		t.Fatalf("description = %v", req.Description)
	}
	if req.Color == nil || *req.Color != "#112233" {
		t.Fatalf("color = %v", req.Color)
	}
	if len(req.Properties) != 3 {
		t.Fatalf("properties = %+v", req.Properties)
	}
}

func TestParseProppatchCollectsUnsupportedAndProtectedProperties(t *testing.T) {
	t.Parallel()

	const body = `<D:propertyupdate xmlns:D="DAV:" xmlns:E="urn:example:test">
  <D:set><D:prop><D:displayname>Product</D:displayname><E:unsupported>value</E:unsupported></D:prop></D:set>
  <D:remove><D:prop><D:displayname/></D:prop></D:remove>
</D:propertyupdate>`
	req, err := ParseProppatch(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseProppatch returned error: %v", err)
	}
	if req.Name == nil || *req.Name != "Product" {
		t.Fatalf("name = %v", req.Name)
	}
	if len(req.Unsupported) != 1 || req.Unsupported[0] != (XMLName{Space: "urn:example:test", Local: "unsupported"}) {
		t.Fatalf("unsupported = %+v", req.Unsupported)
	}
	if len(req.Protected) != 1 || req.Protected[0] != PropDisplayName {
		t.Fatalf("protected = %+v", req.Protected)
	}
}

func TestParseProppatchCollectsMultipleInstructions(t *testing.T) {
	t.Parallel()

	const body = `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/">
  <D:set>
    <D:prop><D:displayname>Product</D:displayname></D:prop>
  </D:set>
  <D:set>
    <D:prop><CS:calendar-color>#445566</CS:calendar-color></D:prop>
  </D:set>
  <D:remove>
    <D:prop><C:calendar-description/></D:prop>
  </D:remove>
</D:propertyupdate>`
	req, err := ParseProppatch(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseProppatch returned error: %v", err)
	}
	if req.Name == nil || *req.Name != "Product" {
		t.Fatalf("name = %v", req.Name)
	}
	if req.Color == nil || *req.Color != "#445566" {
		t.Fatalf("color = %v", req.Color)
	}
	if req.Description == nil || *req.Description != "" {
		t.Fatalf("description = %v", req.Description)
	}
	if len(req.Properties) != 3 {
		t.Fatalf("properties = %+v", req.Properties)
	}
}

func TestParseProppatchRejectsMultiplePropChildrenPerInstruction(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"set": `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set>
    <D:prop><D:displayname>Product</D:displayname></D:prop>
    <D:prop><C:calendar-description>Launch dates</C:calendar-description></D:prop>
  </D:set>
</D:propertyupdate>`,
		"remove": `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:remove>
    <D:prop><C:calendar-description/></D:prop>
    <D:prop><C:calendar-timezone/></D:prop>
  </D:remove>
</D:propertyupdate>`,
	}
	for name, body := range tests {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseProppatch(strings.NewReader(body)); err == nil {
				t.Fatal("ParseProppatch error = nil, want duplicate DAV:prop rejection")
			}
		})
	}
}

func TestParseProppatchRejectsTooManyPropertiesAcrossPropBlocks(t *testing.T) {
	t.Parallel()

	var body strings.Builder
	body.WriteString(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:E="urn:example:test">`)
	body.WriteString(`<D:set><D:prop>`)
	for i := 0; i < MaxWebDAVProperties-2; i++ {
		body.WriteString(`<E:unsupported/>`)
	}
	body.WriteString(`</D:prop></D:set>`)
	body.WriteString(`<D:set><D:prop><D:displayname>Product</D:displayname><C:calendar-description>Launch dates</C:calendar-description></D:prop></D:set>`)
	body.WriteString(`<D:remove><D:prop><D:displayname/></D:prop></D:remove>`)
	body.WriteString(`</D:propertyupdate>`)

	if _, err := ParseProppatch(strings.NewReader(body.String())); err == nil {
		t.Fatal("ParseProppatch error = nil, want rejection for aggregate property count")
	}
}

func TestParseProppatchRemovesOptionalCalendarProperties(t *testing.T) {
	t.Parallel()

	const body = `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:A="http://apple.com/ns/ical/">
  <D:remove>
    <D:prop>
      <C:calendar-description/>
      <A:calendar-color/>
    </D:prop>
  </D:remove>
</D:propertyupdate>`
	req, err := ParseProppatch(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseProppatch returned error: %v", err)
	}
	if req.Description == nil || *req.Description != "" {
		t.Fatalf("description = %v", req.Description)
	}
	if req.Color == nil || *req.Color != "" {
		t.Fatalf("color = %v", req.Color)
	}
}

func TestParseProppatchAllowsEmptyUnsupportedRemoveProperty(t *testing.T) {
	t.Parallel()

	const body = `<D:propertyupdate xmlns:D="DAV:" xmlns:E="urn:example:test">
  <D:remove>
    <D:prop>
      <E:unsupported/>
    </D:prop>
  </D:remove>
</D:propertyupdate>`
	req, err := ParseProppatch(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseProppatch returned error: %v", err)
	}
	if len(req.Unsupported) != 1 || req.Unsupported[0] != (XMLName{Space: "urn:example:test", Local: "unsupported"}) {
		t.Fatalf("unsupported = %+v", req.Unsupported)
	}
}

func TestParseProppatchRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"empty body":                       ``,
		"wrong root":                       `<D:propfind xmlns:D="DAV:"/>`,
		"nested supported text":            `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop><D:displayname><D:x/></D:displayname></D:prop></D:set></D:propertyupdate>`,
		"remove supported property text":   `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:remove><D:prop><C:calendar-description>Launch dates</C:calendar-description></D:prop></D:remove></D:propertyupdate>`,
		"remove supported property child":  `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:remove><D:prop><C:calendar-description><C:x/></C:calendar-description></D:prop></D:remove></D:propertyupdate>`,
		"remove unsupported property text": `<D:propertyupdate xmlns:D="DAV:" xmlns:E="urn:example:test"><D:remove><D:prop><E:unsupported>value</E:unsupported></D:prop></D:remove></D:propertyupdate>`,
		"empty self closing set with valid remove": `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set/>
  <D:remove><D:prop><C:calendar-description/></D:prop></D:remove>
</D:propertyupdate>`,
		"empty set with valid remove": `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set></D:set>
  <D:remove><D:prop><C:calendar-description/></D:prop></D:remove>
</D:propertyupdate>`,
		"empty self closing remove with valid set": `<D:propertyupdate xmlns:D="DAV:">
  <D:remove/>
  <D:set><D:prop><D:displayname>Product</D:displayname></D:prop></D:set>
</D:propertyupdate>`,
		"empty remove with valid set": `<D:propertyupdate xmlns:D="DAV:">
  <D:remove></D:remove>
  <D:set><D:prop><D:displayname>Product</D:displayname></D:prop></D:set>
</D:propertyupdate>`,
		"unsupported child shape": `<D:propertyupdate xmlns:D="DAV:"><D:patch/></D:propertyupdate>`,
		"unsupported set child": `<D:propertyupdate xmlns:D="DAV:" xmlns:E="urn:example:test">
  <D:set><D:prop><D:displayname>Product</D:displayname></D:prop><E:unsupported/></D:set>
</D:propertyupdate>`,
		"unsupported remove child": `<D:propertyupdate xmlns:D="DAV:" xmlns:E="urn:example:test">
  <D:remove><D:prop><D:displayname/></D:prop><E:unsupported/></D:remove>
</D:propertyupdate>`,
	}
	for name, body := range tests {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseProppatch(strings.NewReader(body)); err == nil {
				t.Fatal("ParseProppatch error = nil, want rejection")
			}
		})
	}
}

func TestParseReportRecognizesCalDAVAndSyncReports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want ReportKind
	}{
		{
			name: "calendar-query",
			body: `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><D:getetag/><C:calendar-data/></D:prop><C:filter><C:comp-filter name="VCALENDAR"/></C:filter></C:calendar-query>`,
			want: ReportCalendarQuery,
		},
		{
			name: "calendar-multiget",
			body: `<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><D:getetag/></D:prop><D:href>/caldav/calendars/u/work/e.ics</D:href></C:calendar-multiget>`,
			want: ReportCalendarMulti,
		},
		{
			name: "free-busy-query",
			body: `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"><C:time-range start="20260506T000000Z" end="20260507T000000Z"/></C:free-busy-query>`,
			want: ReportFreeBusyQuery,
		},
		{
			name: "sync-collection",
			body: `<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`,
			want: ReportSyncCollection,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := ParseReport(strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("ParseReport returned error: %v", err)
			}
			if req.Kind != tc.want {
				t.Fatalf("kind = %q, want %q", req.Kind, tc.want)
			}
		})
	}
}

func TestParseReportCollectsPropertiesHrefsAndSyncToken(t *testing.T) {
	t.Parallel()

	const body = `<D:sync-collection xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token> sync-123 </D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:limit><D:nresults>25</D:nresults></D:limit>
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <D:href> /caldav/calendars/user/work/event.ics </D:href>
</D:sync-collection>`
	req, err := ParseReport(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if req.SyncToken != "sync-123" || !req.HasSyncToken {
		t.Fatalf("sync token = %q has = %v", req.SyncToken, req.HasSyncToken)
	}
	if len(req.Hrefs) != 1 || req.Hrefs[0] != "/caldav/calendars/user/work/event.ics" {
		t.Fatalf("hrefs = %+v", req.Hrefs)
	}
	if req.SyncLevel != "1" {
		t.Fatalf("sync level = %q", req.SyncLevel)
	}
	if req.Limit != 25 {
		t.Fatalf("limit = %d", req.Limit)
	}
	if len(req.Properties) != 2 {
		t.Fatalf("properties = %+v, want 2", req.Properties)
	}
}

func TestParseReportCollectsCalendarDataProjection(t *testing.T) {
	t.Parallel()

	const body = `<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop>
    <D:getetag/>
    <C:calendar-data>
      <C:comp name="VCALENDAR">
        <C:prop name="VERSION"/>
        <C:prop name="PRODID"/>
        <C:comp name="VEVENT">
          <C:prop name="UID"/>
          <C:prop name="DTSTART"/>
          <C:prop name="SUMMARY"/>
        </C:comp>
      </C:comp>
    </C:calendar-data>
  </D:prop>
  <D:href>/caldav/calendars/user/work/event.ics</D:href>
</C:calendar-multiget>`
	req, err := ParseReport(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if !req.CalendarData.Requested || !req.CalendarData.HasProjection {
		t.Fatalf("calendar-data projection = %+v", req.CalendarData)
	}
	if !req.CalendarData.CalendarProperties["VERSION"] || !req.CalendarData.CalendarProperties["PRODID"] {
		t.Fatalf("calendar properties = %+v", req.CalendarData.CalendarProperties)
	}
	if req.CalendarData.Component != ComponentVEVENT {
		t.Fatalf("component = %q, want %q", req.CalendarData.Component, ComponentVEVENT)
	}
	if !req.CalendarData.ComponentProperties["UID"] || !req.CalendarData.ComponentProperties["DTSTART"] || !req.CalendarData.ComponentProperties["SUMMARY"] {
		t.Fatalf("component properties = %+v", req.CalendarData.ComponentProperties)
	}
}

func TestParseReportRejectsUnsupportedCalendarDataMediaAttributes(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"content_type": `<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><C:calendar-data content-type="application/calendar+json"/></D:prop>
  <D:href>/caldav/calendars/user/work/event.ics</D:href>
</C:calendar-multiget>`,
		"version": `<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><C:calendar-data content-type="text/calendar" version="1.0"/></D:prop>
  <D:href>/caldav/calendars/user/work/event.ics</D:href>
</C:calendar-multiget>`,
	}
	for name, body := range tests {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseReport(strings.NewReader(body)); err == nil {
				t.Fatal("ParseReport error = nil, want unsupported calendar-data media rejection")
			}
		})
	}
}

func TestParseReportCollectsCalendarQueryTimeRange(t *testing.T) {
	t.Parallel()

	const body = `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/><C:calendar-data/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="20260506T000000Z" end="20260507T000000Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`
	req, err := ParseReport(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if !req.HasFilter {
		t.Fatal("HasFilter = false, want true")
	}
	if req.TimeRange == nil {
		t.Fatal("TimeRange = nil")
	}
	if req.Component != ComponentVEVENT {
		t.Fatalf("component = %q, want %q", req.Component, ComponentVEVENT)
	}
	if got := req.TimeRange.Start.Format("20060102T150405Z"); got != "20260506T000000Z" {
		t.Fatalf("start = %s", got)
	}
	if got := req.TimeRange.End.Format("20060102T150405Z"); got != "20260507T000000Z" {
		t.Fatalf("end = %s", got)
	}
}

func TestParseReportCollectsCalendarQueryComponentFilter(t *testing.T) {
	t.Parallel()

	const body = `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VTODO"/>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`
	req, err := ParseReport(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if req.Component != ComponentVTODO {
		t.Fatalf("component = %q, want %q", req.Component, ComponentVTODO)
	}
	if req.TimeRange != nil {
		t.Fatalf("time range = %+v, want nil", req.TimeRange)
	}
}

func TestParseReportRejectsUnsupportedCalendarQueryFilter(t *testing.T) {
	t.Parallel()

	const body = `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:prop-filter name="SUMMARY"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`
	_, err := ParseReport(strings.NewReader(body))
	var unsupported UnsupportedCalendarFilterError
	if !errors.As(err, &unsupported) {
		t.Fatalf("ParseReport error = %v, want UnsupportedCalendarFilterError", err)
	}
	if unsupported.Element.Local != "prop-filter" {
		t.Fatalf("unsupported element = %+v", unsupported.Element)
	}
}

func TestParseReportRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"empty body":       ``,
		"unknown root":     `<D:expand-property xmlns:D="DAV:"/>`,
		"wrong namespace":  `<calendar-query/>`,
		"nested href":      `<D:sync-collection xmlns:D="DAV:"><D:href><D:x/></D:href></D:sync-collection>`,
		"too many hrefs":   `<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">` + strings.Repeat("<D:href>/x.ics</D:href>", MaxWebDAVHrefs+1) + `</C:calendar-multiget>`,
		"multiget no href": `<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></C:calendar-multiget>`,
		"query no filter":  `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></C:calendar-query>`,
		"query comp filter no name": `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/></D:prop>
  <C:filter><C:comp-filter/></C:filter>
</C:calendar-query>`,
		"query top comp filter not vcalendar": `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/></D:prop>
  <C:filter><C:comp-filter name="VEVENT"/></C:filter>
</C:calendar-query>`,
		"query direct time range filter": `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/></D:prop>
  <C:filter><C:time-range start="20260506T000000Z" end="20260507T000000Z"/></C:filter>
</C:calendar-query>`,
		"query direct time range plus comp filter": `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/></D:prop>
  <C:filter>
    <C:time-range start="20260506T000000Z" end="20260507T000000Z"/>
    <C:comp-filter name="VCALENDAR"/>
  </C:filter>
</C:calendar-query>`,
		"query duplicate comp time range": `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/></D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="20260506T000000Z" end="20260507T000000Z"/>
        <C:time-range start="20260508T000000Z" end="20260509T000000Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
		"query duplicate top comp filter": `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:">
  <D:prop><D:getetag/></D:prop>
  <C:filter><C:comp-filter name="VCALENDAR"/><C:comp-filter name="VCALENDAR"/></C:filter>
</C:calendar-query>`,
		"free busy no range": `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"/>`,
		"free busy duplicate range": `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav">
  <C:time-range start="20260506T000000Z" end="20260507T000000Z"/>
  <C:time-range start="20260508T000000Z" end="20260509T000000Z"/>
</C:free-busy-query>`,
		"sync no token":  `<D:sync-collection xmlns:D="DAV:"><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`,
		"sync no level":  `<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:prop><D:getetag/></D:prop></D:sync-collection>`,
		"sync bad level": `<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>infinity</D:sync-level></D:sync-collection>`,
		"sync no prop":   `<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level></D:sync-collection>`,
		"duplicate sync-token": `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token>sync-1</D:sync-token>
  <D:sync-token/>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`,
		"duplicate sync-level": `<D:sync-collection xmlns:D="DAV:">
  <D:sync-token/>
  <D:sync-level>1</D:sync-level>
  <D:sync-level>1</D:sync-level>
  <D:prop><D:getetag/></D:prop>
</D:sync-collection>`,
		"bad range order": `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"><C:time-range start="20260507T000000Z" end="20260506T000000Z"/></C:free-busy-query>`,
		"bad range utc":   `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"><C:time-range start="20260506T000000" end="20260507T000000Z"/></C:free-busy-query>`,
		"bad limit":       `<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level><D:limit><D:nresults>0</D:nresults></D:limit></D:sync-collection>`,
		"duplicate limit": `<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level><D:limit><D:nresults>25</D:nresults></D:limit><D:limit><D:nresults>50</D:nresults></D:limit><D:prop><D:getetag/></D:prop></D:sync-collection>`,
		"duplicate nresults": `<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level><D:limit>
  <D:nresults>25</D:nresults>
  <D:nresults>50</D:nresults>
</D:limit><D:prop><D:getetag/></D:prop></D:sync-collection>`,
	}
	for name, body := range tests {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseReport(strings.NewReader(body)); err == nil {
				t.Fatal("ParseReport error = nil, want rejection")
			}
		})
	}
}
