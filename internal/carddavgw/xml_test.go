package carddavgw

import (
	"strings"
	"testing"
)

func TestParseDepth(t *testing.T) {
	t.Parallel()

	got, err := ParseDepth("", DepthZero)
	if err != nil {
		t.Fatalf("ParseDepth returned error: %v", err)
	}
	if got != DepthZero {
		t.Fatalf("depth = %q, want %q", got, DepthZero)
	}
	if got, err := ParseDepth("1", DepthZero); err != nil || got != DepthOne {
		t.Fatalf("ParseDepth(1) = %q, %v", got, err)
	}
	if got, err := ParseDepth("infinity", DepthZero); err != nil || got != DepthInfinity {
		t.Fatalf("ParseDepth(infinity) = %q, %v", got, err)
	}
	for _, value := range []string{"2", "1\n", ""} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseDepth(value, ""); err == nil {
				t.Fatalf("ParseDepth(%q) error = nil, want rejection", value)
			}
		})
	}
}

func TestParsePropfind(t *testing.T) {
	t.Parallel()

	req, err := ParsePropfind(strings.NewReader(" \n\t "))
	if err != nil {
		t.Fatalf("ParsePropfind empty returned error: %v", err)
	}
	if req.Kind != PropfindAllProp {
		t.Fatalf("empty kind = %q, want %q", req.Kind, PropfindAllProp)
	}

	const body = `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:prop><D:getetag/><C:address-data/></D:prop></D:propfind>`
	req, err = ParsePropfind(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParsePropfind prop returned error: %v", err)
	}
	want := []XMLName{{Space: DAVNamespace, Local: "getetag"}, {Space: CardDAVNamespace, Local: "address-data"}}
	if req.Kind != PropfindProp || len(req.Properties) != len(want) {
		t.Fatalf("request = %+v, want kind %q props %+v", req, PropfindProp, want)
	}
	for i := range want {
		if req.Properties[i] != want[i] {
			t.Fatalf("property %d = %+v, want %+v", i, req.Properties[i], want[i])
		}
	}
}

func TestParsePropfindRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"wrong root":            `<D:sync-collection xmlns:D="DAV:"/>`,
		"multiple modes":        `<D:propfind xmlns:D="DAV:"><D:allprop/><D:propname/></D:propfind>`,
		"empty prop":            `<D:propfind xmlns:D="DAV:"><D:prop/></D:propfind>`,
		"include before mode":   `<D:propfind xmlns:D="DAV:"><D:include><D:getetag/></D:include></D:propfind>`,
		"unsupported element":   `<D:propfind xmlns:D="DAV:"><D:foo/></D:propfind>`,
		"multiple roots":        `<D:propfind xmlns:D="DAV:"/><D:propfind xmlns:D="DAV:"/>`,
		"malformed":             `<D:propfind xmlns:D="DAV:"><D:allprop></D:propfind>`,
		"unsupported directive": `<!DOCTYPE propfind><D:propfind xmlns:D="DAV:"/>`,
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

func TestParseReportRecognizesCardDAVAndSyncReports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want ReportKind
	}{
		{
			name: "addressbook query",
			body: `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:prop-filter name="FN"/></C:filter></C:addressbook-query>`,
			want: ReportAddressBookQuery,
		},
		{
			name: "addressbook multiget",
			body: `<C:addressbook-multiget xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:"><D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href></C:addressbook-multiget>`,
			want: ReportAddressBookMulti,
		},
		{
			name: "sync collection",
			body: `<D:sync-collection xmlns:D="DAV:"><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`,
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

	const body = `<D:sync-collection xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:sync-token>sync-abc</D:sync-token>
  <D:sync-level>1</D:sync-level>
  <D:limit><D:nresults>25</D:nresults></D:limit>
  <D:prop><D:getetag/><C:address-data><C:prop name="FN"/><C:prop name="EMAIL"/></C:address-data></D:prop>
</D:sync-collection>`
	req, err := ParseReport(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if req.SyncToken != "sync-abc" || req.SyncLevel != "1" || req.Limit != 25 {
		t.Fatalf("sync metadata = %+v", req)
	}
	want := []XMLName{
		{Space: DAVNamespace, Local: "getetag"},
		{Space: CardDAVNamespace, Local: "address-data"},
	}
	if len(req.Properties) != len(want) {
		t.Fatalf("properties = %+v, want %+v", req.Properties, want)
	}
	for i := range want {
		if req.Properties[i] != want[i] {
			t.Fatalf("property %d = %+v, want %+v", i, req.Properties[i], want[i])
		}
	}
	if got := req.AddressDataProperties; len(got) != 2 || got[0] != "FN" || got[1] != "EMAIL" {
		t.Fatalf("address-data properties = %+v", got)
	}
}

func TestParseReportCollectsAddressBookQueryTextMatch(t *testing.T) {
	t.Parallel()

	const body = `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav">
  <C:filter>
    <C:prop-filter name="FN">
      <C:text-match collation="i;unicode-casemap"> Alice </C:text-match>
    </C:prop-filter>
  </C:filter>
</C:addressbook-query>`
	req, err := ParseReport(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if !req.HasFilter || req.Filter.Test != FilterTestAnyOf || len(req.Filter.PropFilters) != 1 {
		t.Fatalf("filter = %+v", req)
	}
	propFilter := req.Filter.PropFilters[0]
	if propFilter.Name != "FN" || propFilter.Test != FilterTestAnyOf || len(propFilter.TextMatches) != 1 {
		t.Fatalf("prop-filter = %+v", propFilter)
	}
	if propFilter.TextMatches[0].Text != "Alice" || propFilter.TextMatches[0].Collation != TextMatchUnicodeCasemap || propFilter.TextMatches[0].MatchType != TextMatchContains || propFilter.TextMatches[0].Negate {
		t.Fatalf("text-match defaults = %+v", propFilter.TextMatches[0])
	}
}

func TestParseReportCollectsAddressBookQueryTextMatchAttributes(t *testing.T) {
	t.Parallel()

	const body = `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav">
  <C:filter>
    <C:prop-filter name="EMAIL">
      <C:text-match collation="i;unicode-casemap" match-type="ends-with" negate-condition="yes">example.net</C:text-match>
    </C:prop-filter>
  </C:filter>
</C:addressbook-query>`
	req, err := ParseReport(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if len(req.Filter.PropFilters) != 1 {
		t.Fatalf("filter = %+v", req.Filter)
	}
	propFilter := req.Filter.PropFilters[0]
	if propFilter.Name != "EMAIL" || len(propFilter.TextMatches) != 1 {
		t.Fatalf("prop-filter = %+v", propFilter)
	}
	match := propFilter.TextMatches[0]
	if match.Text != "example.net" || match.MatchType != TextMatchEndsWith || match.Collation != TextMatchUnicodeCasemap || !match.Negate {
		t.Fatalf("text-match = %+v", match)
	}
}

func TestParseReportCollectsAddressBookQueryParamFilter(t *testing.T) {
	t.Parallel()

	const body = `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav">
  <C:filter>
    <C:prop-filter name="EMAIL">
      <C:param-filter name="TYPE"><C:text-match match-type="equals">home</C:text-match></C:param-filter>
    </C:prop-filter>
  </C:filter>
</C:addressbook-query>`
	req, err := ParseReport(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if len(req.Filter.PropFilters) != 1 {
		t.Fatalf("filter = %+v", req.Filter)
	}
	propFilter := req.Filter.PropFilters[0]
	if propFilter.Name != "EMAIL" || len(propFilter.ParamFilters) != 1 {
		t.Fatalf("prop-filter = %+v", propFilter)
	}
	paramFilter := propFilter.ParamFilters[0]
	if paramFilter.Name != "TYPE" || !paramFilter.HasTextMatch {
		t.Fatalf("param-filter = %+v", paramFilter)
	}
	if paramFilter.TextMatch.Text != "home" || paramFilter.TextMatch.MatchType != TextMatchEquals {
		t.Fatalf("param text-match = %+v", paramFilter.TextMatch)
	}
}

func TestParseReportCollectsAddressBookQueryFilterTests(t *testing.T) {
	t.Parallel()

	const body = `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav">
  <C:filter test="allof">
    <C:prop-filter name="FN"><C:text-match>Contact</C:text-match></C:prop-filter>
    <C:prop-filter name="EMAIL" test="allof">
      <C:text-match match-type="ends-with">example.com</C:text-match>
      <C:param-filter name="TYPE"><C:text-match match-type="equals">work</C:text-match></C:param-filter>
    </C:prop-filter>
  </C:filter>
</C:addressbook-query>`
	req, err := ParseReport(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseReport returned error: %v", err)
	}
	if req.Filter.Test != FilterTestAllOf || len(req.Filter.PropFilters) != 2 {
		t.Fatalf("filter = %+v", req.Filter)
	}
	if req.Filter.PropFilters[1].Test != FilterTestAllOf || len(req.Filter.PropFilters[1].TextMatches) != 1 || len(req.Filter.PropFilters[1].ParamFilters) != 1 {
		t.Fatalf("second prop-filter = %+v", req.Filter.PropFilters[1])
	}
}

func TestParseReportRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"empty":                  ``,
		"wrong root":             `<D:propfind xmlns:D="DAV:"/>`,
		"query missing filter":   `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"/>`,
		"multiget missing href":  `<C:addressbook-multiget xmlns:C="urn:ietf:params:xml:ns:carddav"/>`,
		"sync missing level":     `<D:sync-collection xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></D:sync-collection>`,
		"sync unsupported level": `<D:sync-collection xmlns:D="DAV:"><D:sync-level>infinity</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`,
		"sync missing prop":      `<D:sync-collection xmlns:D="DAV:"><D:sync-level>1</D:sync-level></D:sync-collection>`,
		"limit too high":         `<D:sync-collection xmlns:D="DAV:"><D:sync-level>1</D:sync-level><D:limit><D:nresults>1001</D:nresults></D:limit><D:prop><D:getetag/></D:prop></D:sync-collection>`,
		"text match line break":  `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:text-match>A&#x0A;B</C:text-match></C:filter></C:addressbook-query>`,
		"bad match type":         `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:prop-filter name="FN"><C:text-match match-type="wildcard">A</C:text-match></C:prop-filter></C:filter></C:addressbook-query>`,
		"bad negate condition":   `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:prop-filter name="FN"><C:text-match negate-condition="maybe">A</C:text-match></C:prop-filter></C:filter></C:addressbook-query>`,
		"unsupported collation":  `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:prop-filter name="FN"><C:text-match collation="i;octet">A</C:text-match></C:prop-filter></C:filter></C:addressbook-query>`,
		"bad filter test":        `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter test="maybe"><C:prop-filter name="FN"/></C:filter></C:addressbook-query>`,
		"bad prop filter test":   `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:prop-filter name="FN" test="maybe"/></C:filter></C:addressbook-query>`,
		"prop filter no name":    `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:prop-filter><C:text-match>A</C:text-match></C:prop-filter></C:filter></C:addressbook-query>`,
		"bad prop filter name":   `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:prop-filter name="bad name"><C:text-match>A</C:text-match></C:prop-filter></C:filter></C:addressbook-query>`,
		"bad address-data prop":  `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:"><C:filter/><D:prop><C:address-data><C:prop name="bad name"/></C:address-data></D:prop></C:addressbook-query>`,
		"param filter no parent": `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:param-filter name="TYPE"><C:text-match>home</C:text-match></C:param-filter></C:filter></C:addressbook-query>`,
		"param filter no name":   `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:prop-filter name="EMAIL"><C:param-filter><C:text-match>home</C:text-match></C:param-filter></C:prop-filter></C:filter></C:addressbook-query>`,
		"param filter mixed":     `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter><C:prop-filter name="EMAIL"><C:param-filter name="TYPE"><C:is-not-defined/><C:text-match>home</C:text-match></C:param-filter></C:prop-filter></C:filter></C:addressbook-query>`,
		"malformed xml":          `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter></C:addressbook-query>`,
		"multiple roots":         `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter/></C:addressbook-query><C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"/>`,
		"xml directive":          `<!DOCTYPE report><C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter/></C:addressbook-query>`,
		"too much nesting":       `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav"><C:filter>` + strings.Repeat("<C:x>", MaxWebDAVXMLDepth+1) + strings.Repeat("</C:x>", MaxWebDAVXMLDepth+1) + `</C:filter></C:addressbook-query>`,
		"href nested element":    `<C:addressbook-multiget xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:"><D:href><D:x/></D:href></C:addressbook-multiget>`,
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

func TestParseReportRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	if _, err := ParseReport(strings.NewReader(strings.Repeat("x", MaxWebDAVXMLBodyBytes+1))); err == nil {
		t.Fatal("ParseReport accepted oversized body")
	}
}
