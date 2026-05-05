package carddavgw

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
		Href: "/carddav/principals/user-1/",
		PropStats: []PropStatus{
			{
				StatusCode: http.StatusOK,
				Properties: []PropertyResult{
					{Name: PropDisplayName, Value: PropertyValue{Text: "User One"}, Found: true},
					{Name: PropAddressBookHomeSet, Value: PropertyValue{Hrefs: []string{"/carddav/addressbooks/user-1/"}}, Found: true},
					{Name: PropResourceType, Value: PropertyValue{ResourceTypes: []XMLName{ResourceTypeCollection, ResourceTypePrincipal}}, Found: true},
				},
			},
			{
				StatusCode: http.StatusNotFound,
				Properties: []PropertyResult{
					{Name: XMLName{Space: CardDAVNamespace, Local: "unknown-prop"}, Found: false},
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
		"<D:href>/carddav/principals/user-1/</D:href>",
		"<D:status>HTTP/1.1 200 OK</D:status>",
		"<D:status>HTTP/1.1 404 Not Found</D:status>",
		"<C:addressbook-home-set><D:href>/carddav/addressbooks/user-1/</D:href></C:addressbook-home-set>",
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
		Href: "/carddav/addressbooks/user-1/personal/contact-1.vcf",
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
	if !strings.Contains(text, "<D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>") {
		t.Fatalf("response href missing:\n%s", text)
	}
}

func TestBuildMultiStatusXMLRendersResponseStatus(t *testing.T) {
	t.Parallel()

	body, err := BuildSyncCollectionXML([]MultiStatusResponse{{
		Href:   "/carddav/addressbooks/user-1/personal/missing.vcf",
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

func TestSelectReportPropertiesSeparatesFoundAndMissing(t *testing.T) {
	t.Parallel()

	available := []PropertyResult{
		{Name: PropGetETag, Value: PropertyValue{Text: `"abc"`}, Found: true},
		{Name: PropAddressData, Value: PropertyValue{Text: "BEGIN:VCARD\r\nEND:VCARD\r\n"}, Found: true},
	}
	stats := SelectReportProperties(ReportRequest{
		Properties: []XMLName{
			PropGetETag,
			{Space: CardDAVNamespace, Local: "missing"},
			PropAddressData,
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

func TestContactObjectDataPropertyProjectsRequestedProperties(t *testing.T) {
	t.Parallel()

	prop, err := ContactObjectDataPropertyWithProperties([]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEMAIL:one@example.com\r\nEND:VCARD\r\n"), []string{"FN"})
	if err != nil {
		t.Fatalf("ContactObjectDataPropertyWithProperties returned error: %v", err)
	}
	if !prop.Found {
		t.Fatal("projected address-data not found")
	}
	if !strings.Contains(prop.Value.Text, "BEGIN:VCARD\r\n") || !strings.Contains(prop.Value.Text, "VERSION:4.0\r\n") || !strings.Contains(prop.Value.Text, "FN:Contact One\r\n") || !strings.Contains(prop.Value.Text, "END:VCARD\r\n") {
		t.Fatalf("projected vcard missing required/requested lines:\n%s", prop.Value.Text)
	}
	if strings.Contains(prop.Value.Text, "EMAIL:") || strings.Contains(prop.Value.Text, "UID:") {
		t.Fatalf("projected vcard included unrequested data:\n%s", prop.Value.Text)
	}
}

func TestContactObjectDataPropertyRejectsProjectionFailures(t *testing.T) {
	t.Parallel()

	if _, err := ContactObjectDataPropertyWithProperties([]byte("BEGIN:VCARD\r\nBROKEN\r\nEND:VCARD\r\n"), []string{"FN"}); err == nil {
		t.Fatal("ContactObjectDataPropertyWithProperties error = nil, want projection failure")
	}
}

func TestBuildMultiStatusXMLRendersAddressDataTypeAttributes(t *testing.T) {
	t.Parallel()

	body, err := BuildMultiStatusXML([]MultiStatusResponse{{
		Href: "/carddav/addressbooks/user-1/personal/contact-1.vcf",
		PropStats: []PropStatus{{
			StatusCode: http.StatusOK,
			Properties: []PropertyResult{
				ContactObjectDataProperty([]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n")),
			},
		}},
	}})
	if err != nil {
		t.Fatalf("BuildMultiStatusXML returned error: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, `<C:address-data content-type="text/vcard" version="4.0">BEGIN:VCARD`) {
		t.Fatalf("address-data type attributes missing:\n%s", text)
	}
}

func TestSelectPropfindPropertiesSupportsPropfindModes(t *testing.T) {
	t.Parallel()

	available := []PropertyResult{
		{Name: PropDisplayName, Value: PropertyValue{Text: "Personal"}, Found: true},
		{Name: PropGetETag, Value: PropertyValue{Text: `"abc"`}, Found: true},
	}
	prop := SelectPropfindProperties(PropfindRequest{
		Kind: PropfindProp,
		Properties: []XMLName{
			PropGetETag,
			{Space: CardDAVNamespace, Local: "missing"},
		},
	}, available)
	if len(prop) != 2 || prop[0].StatusCode != http.StatusOK || prop[1].StatusCode != http.StatusNotFound {
		t.Fatalf("prop stats = %+v", prop)
	}

	propname := SelectPropfindProperties(PropfindRequest{Kind: PropfindPropName}, available)
	if len(propname) != 1 || len(propname[0].Properties) != 2 {
		t.Fatalf("propname stats = %+v", propname)
	}

	allprop := SelectPropfindProperties(PropfindRequest{
		Kind:    PropfindAllProp,
		Include: []XMLName{PropGetETag},
	}, available)
	if len(allprop) != 1 || len(allprop[0].Properties) != 2 {
		t.Fatalf("allprop stats = %+v", allprop)
	}
}

func TestAddressBookHomePropertiesUsePrincipalAsCurrentUser(t *testing.T) {
	t.Parallel()

	props, err := AddressBookHomeProperties("user-1")
	if err != nil {
		t.Fatalf("AddressBookHomeProperties returned error: %v", err)
	}
	body, err := BuildMultiStatusXML([]MultiStatusResponse{{
		Href:      "/carddav/addressbooks/user-1/",
		PropStats: []PropStatus{{StatusCode: http.StatusOK, Properties: props}},
	}})
	if err != nil {
		t.Fatalf("BuildMultiStatusXML returned error: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "<D:current-user-principal><D:href>/carddav/principals/user-1/</D:href></D:current-user-principal>") {
		t.Fatalf("current-user-principal should point to principal href:\n%s", text)
	}
	if !strings.Contains(text, "<D:current-user-privilege-set><D:privilege><D:read></D:read></D:privilege></D:current-user-privilege-set>") {
		t.Fatalf("current-user-privilege-set should expose read privilege:\n%s", text)
	}
}

func TestAddressBookCollectionPropertiesExposeCardDAVDiscovery(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 6, 1, 2, 3, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	props, err := AddressBookCollectionProperties("user-1", AddressBook{
		ID:          "personal",
		UserID:      "user-1",
		Name:        "Personal",
		Description: "People",
		SyncToken:   "sync-123",
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	})
	if err != nil {
		t.Fatalf("AddressBookCollectionProperties returned error: %v", err)
	}
	body, err := BuildMultiStatusXML([]MultiStatusResponse{{
		Href:      "/carddav/addressbooks/user-1/personal/",
		PropStats: []PropStatus{{StatusCode: http.StatusOK, Properties: props}},
	}})
	if err != nil {
		t.Fatalf("BuildMultiStatusXML returned error: %v", err)
	}
	assertParseableXML(t, body)
	text := string(body)
	for _, want := range []string{
		"<C:addressbook></C:addressbook>",
		"<C:supported-address-data><C:address-data content-type=\"text/vcard\" version=\"4.0\"></C:address-data></C:supported-address-data>",
		"<C:max-resource-size>5242880</C:max-resource-size>",
		"<D:sync-token>sync-123</D:sync-token>",
		"<D:owner><D:href>/carddav/principals/user-1/</D:href></D:owner>",
		"<D:current-user-privilege-set><D:privilege><D:read></D:read></D:privilege></D:current-user-privilege-set>",
		"<D:creationdate>2026-05-06T01:02:03Z</D:creationdate>",
		"<D:getlastmodified>Wed, 06 May 2026 04:05:06 GMT</D:getlastmodified>",
		"<D:supported-report-set>",
		"<C:addressbook-query></C:addressbook-query>",
		"<C:addressbook-multiget></C:addressbook-multiget>",
		"<D:sync-collection></D:sync-collection>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("addressbook collection XML missing %q:\n%s", want, text)
		}
	}
}

func TestContactObjectPropertiesExposeObjectMetadata(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 6, 1, 2, 3, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 6, 4, 5, 6, 0, time.UTC)
	props, err := ContactObjectProperties("user-1", ContactObject{
		AddressBookID: "personal",
		ObjectName:    "contact-1.vcf",
		ETag:          `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
		Size:          64,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	})
	if err != nil {
		t.Fatalf("ContactObjectProperties returned error: %v", err)
	}
	body, err := BuildMultiStatusXML([]MultiStatusResponse{{
		Href:      "/carddav/addressbooks/user-1/personal/contact-1.vcf",
		PropStats: []PropStatus{{StatusCode: http.StatusOK, Properties: props}},
	}})
	if err != nil {
		t.Fatalf("BuildMultiStatusXML returned error: %v", err)
	}
	assertParseableXML(t, body)
	text := string(body)
	for _, want := range []string{
		"<D:getetag>&#34;0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&#34;</D:getetag>",
		"<D:getcontenttype>text/vcard; charset=utf-8</D:getcontenttype>",
		"<D:getcontentlength>64</D:getcontentlength>",
		"<D:current-user-privilege-set><D:privilege><D:read></D:read></D:privilege><D:privilege><D:write-content></D:write-content></D:privilege></D:current-user-privilege-set>",
		"<D:resourcetype></D:resourcetype>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("contact object XML missing %q:\n%s", want, text)
		}
	}
}

func TestSupportedAddressBookReportsMatchesParsedReports(t *testing.T) {
	t.Parallel()

	reports := SupportedAddressBookReports()
	want := []XMLName{
		{Space: CardDAVNamespace, Local: "addressbook-query"},
		{Space: CardDAVNamespace, Local: "addressbook-multiget"},
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

func assertParseableXML(t *testing.T, body []byte) {
	t.Helper()

	dec := xml.NewDecoder(bytes.NewReader(body))
	for {
		if _, err := dec.Token(); err != nil {
			if err == io.EOF {
				return
			}
			t.Fatalf("XML is not parseable: %v\n%s", err, body)
		}
	}
}
