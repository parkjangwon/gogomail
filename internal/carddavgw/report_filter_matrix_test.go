package carddavgw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// This file covers addressbook-query / addressbook-multiget REPORT filtering
// (including param-filter quirks that native clients rely on) and the
// ETag / If-Match / UID-stability guarantees required for lossless
// 3.0 -> 4.0 -> 3.0 conversions and safe concurrent PUTs.

// --- addressbook-query param-filter -----------------------------------------

// Apple Contacts and DAVx5 issue param-filter queries such as
// EMAIL[TYPE contains "work"] to find a typed address. The filter must match on
// the property's TYPE parameter values.
func TestReportQuery_ParamFilterMatchesTypeParameter(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="EMAIL"><C:param-filter name="TYPE"><C:text-match match-type="equals">work</C:text-match></C:param-filter></C:prop-filter></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	// contact-2 has EMAIL;TYPE=work; contact-1 has TYPE=home.
	if !strings.Contains(text, "contact-2.vcf") {
		t.Fatalf("param-filter TYPE=work did not match contact-2:\n%s", text)
	}
	if strings.Contains(text, "contact-1.vcf") {
		t.Fatalf("param-filter TYPE=work wrongly matched contact-1:\n%s", text)
	}
}

// param-filter is-not-defined must select properties lacking the parameter.
func TestReportQuery_ParamFilterIsNotDefined(t *testing.T) {
	t.Parallel()

	store := testCardDAVDiscoveryStore(t)
	// Add a contact whose EMAIL has no TYPE parameter.
	store.objects = append(store.objects, ContactObject{
		UserID:        "user-1",
		AddressBookID: "personal",
		ObjectName:    "contact-3.vcf",
		UID:           "contact-3",
		VCard:         []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-3\r\nFN:No Type\r\nEMAIL:notype@example.com\r\nEND:VCARD\r\n"),
		ETag:          `"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"`,
		Size:          60,
	})
	handler := NewHandler(store, func(*http.Request) (string, error) { return "user-1", nil })

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="EMAIL"><C:param-filter name="TYPE"><C:is-not-defined/></C:param-filter></C:prop-filter></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	req := httptest.NewRequest(MethodReport, "/carddav/addressbooks/user-1/personal/", strings.NewReader(body))
	req.Header.Set("Depth", string(DepthOne))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "contact-3.vcf") {
		t.Fatalf("is-not-defined param-filter did not match untyped EMAIL:\n%s", text)
	}
	if strings.Contains(text, "contact-1.vcf") || strings.Contains(text, "contact-2.vcf") {
		t.Fatalf("is-not-defined param-filter matched typed EMAILs:\n%s", text)
	}
}

// allof test across multiple prop-filters (Apple sends allof for
// name+organization narrowing).
func TestReportQuery_AllOfTestAcrossPropFilters(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter test="allof">
    <C:prop-filter name="FN"><C:text-match>Contact One</C:text-match></C:prop-filter>
    <C:prop-filter name="EMAIL"><C:text-match>contact-one@example.com</C:text-match></C:prop-filter>
  </C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "contact-1.vcf") {
		t.Fatalf("allof filter did not match contact-1:\n%s", text)
	}
	if strings.Contains(text, "contact-2.vcf") {
		t.Fatalf("allof filter matched contact-2:\n%s", text)
	}
}

// anyof test (the CardDAV default) across prop-filters returns the union.
func TestReportQuery_AnyOfTestAcrossPropFilters(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter test="anyof">
    <C:prop-filter name="FN"><C:text-match>Contact One</C:text-match></C:prop-filter>
    <C:prop-filter name="FN"><C:text-match>Other Person</C:text-match></C:prop-filter>
  </C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "contact-1.vcf") || !strings.Contains(text, "contact-2.vcf") {
		t.Fatalf("anyof filter did not match both contacts:\n%s", text)
	}
}

// A param-filter naming an unsupported parameter must yield a supported-filter
// precondition (403), not a silent empty result.
func TestReportQuery_UnsupportedParamFilterPrecondition(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-query xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <C:filter><C:prop-filter name="EMAIL"><C:param-filter name="BOGUS-PARAM"><C:text-match>x</C:text-match></C:param-filter></C:prop-filter></C:filter>
  <D:prop><D:getetag/></D:prop>
</C:addressbook-query>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<C:supported-filter/>") {
		t.Fatalf("missing supported-filter precondition:\n%s", rec.Body.String())
	}
}

// --- addressbook-multiget ---------------------------------------------------

// multiget with address-data must return the full vCard for requested hrefs.
func TestReportMultiget_ReturnsRequestedCards(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-multiget xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <D:href>/carddav/addressbooks/user-1/personal/contact-2.vcf</D:href>
  <D:prop><D:getetag/><C:address-data/></D:prop>
</C:addressbook-multiget>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "FN:Other Person") {
		t.Fatalf("multiget missing requested card body:\n%s", text)
	}
}

// address-data property projection must always retain the mandatory FN even
// when the client only asks for other properties. Regression: FN was dropped,
// yielding an RFC-6350-invalid card that Apple Contacts rejects.
func TestReportMultiget_ProjectionAlwaysKeepsFN(t *testing.T) {
	t.Parallel()

	body := `<C:addressbook-multiget xmlns:C="urn:ietf:params:xml:ns:carddav" xmlns:D="DAV:">
  <D:href>/carddav/addressbooks/user-1/personal/contact-1.vcf</D:href>
  <D:prop><D:getetag/><C:address-data><C:prop name="EMAIL"/></C:address-data></D:prop>
</C:addressbook-multiget>`
	rec := runCardDAVReport(t, "/carddav/addressbooks/user-1/personal/", DepthOne, body)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if !strings.Contains(text, "FN:Contact One") {
		t.Fatalf("projection dropped mandatory FN:\n%s", text)
	}
	if !strings.Contains(text, "EMAIL;TYPE=home:contact-one@example.com") {
		t.Fatalf("projection missing requested EMAIL:\n%s", text)
	}
	if !strings.Contains(text, "UID:contact-1") {
		t.Fatalf("projection dropped required UID:\n%s", text)
	}
}

// --- ETag / If-Match preconditions ------------------------------------------

func TestPutPrecondition_IfNoneMatchStarRejectsExisting(t *testing.T) {
	t.Parallel()

	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n"
	headers := http.Header{}
	headers.Set("Content-Type", "text/vcard")
	headers.Set("If-None-Match", "*")
	rec := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/contact-1.vcf", body, headers)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412 for If-None-Match:* on existing object", rec.Code)
	}
}

func TestPutPrecondition_IfMatchMismatchRejected(t *testing.T) {
	t.Parallel()

	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n"
	headers := http.Header{}
	headers.Set("Content-Type", "text/vcard")
	headers.Set("If-Match", `"deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"`)
	rec := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/contact-1.vcf", body, headers)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412 for stale If-Match", rec.Code)
	}
}

func TestPutPrecondition_ContentTypeVersionMismatchRejected(t *testing.T) {
	t.Parallel()

	// Body declares 4.0 but Content-Type says version=3.0.
	body := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-9\r\nFN:New\r\nEND:VCARD\r\n"
	headers := http.Header{}
	headers.Set("Content-Type", "text/vcard; version=3.0")
	rec := runCardDAVObjectRequest(t, MethodPut, "/carddav/addressbooks/user-1/personal/contact-9.vcf", body, headers)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for Content-Type/body version mismatch", rec.Code)
	}
}

func TestGetConditional_IfNoneMatchReturnsNotModified(t *testing.T) {
	t.Parallel()

	headers := http.Header{}
	headers.Set("If-None-Match", `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`)
	rec := runCardDAVObjectRequest(t, MethodGet, "/carddav/addressbooks/user-1/personal/contact-1.vcf", "", headers)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304 for matching If-None-Match", rec.Code)
	}
}

// --- UID stability across version conversion --------------------------------

// A client migrating a card 3.0 -> 4.0 -> 3.0 must not lose or duplicate the
// UID, and the store pipeline must not corrupt any fields. The ETag is a pure
// function of the body, so an unchanged body yields an unchanged ETag
// (a stable validator across GETs).
func TestUIDStability_VersionRoundTripPreservesUIDAndFields(t *testing.T) {
	t.Parallel()

	v30 := []byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:stable-uid-1\r\nFN:Round Trip\r\n" +
		"N:Trip;Round;;;\r\nEMAIL;TYPE=WORK:rt@example.com\r\nTEL;TYPE=CELL:+1-555-0100\r\nEND:VCARD\r\n")

	meta30 := mustValidVCard(t, v30)
	if meta30.UID != "stable-uid-1" {
		t.Fatalf("3.0 UID = %q", meta30.UID)
	}

	// Simulate a client converting to 4.0 (same UID, VERSION changed) and back.
	v40 := []byte(strings.Replace(string(v30), "VERSION:3.0", "VERSION:4.0", 1))
	meta40 := mustValidVCard(t, v40)
	if meta40.UID != meta30.UID {
		t.Fatalf("UID changed on 3.0->4.0: %q vs %q", meta40.UID, meta30.UID)
	}

	back30 := []byte(strings.Replace(string(v40), "VERSION:4.0", "VERSION:3.0", 1))
	metaBack := mustValidVCard(t, back30)
	if metaBack.UID != meta30.UID {
		t.Fatalf("UID changed on 4.0->3.0: %q vs %q", metaBack.UID, meta30.UID)
	}

	// Field integrity through the store pipeline at each stage.
	for _, stage := range [][]byte{v30, v40, back30} {
		out := storeRoundTrip(stage)
		if got := mustValidVCard(t, out).UID; got != "stable-uid-1" {
			t.Fatalf("store round-trip changed UID: %q", got)
		}
		for _, prop := range []string{"N", "EMAIL", "TEL"} {
			if _, ok := contentLine(t, out, prop); !ok {
				t.Fatalf("store round-trip dropped %s at a conversion stage:\n%s", prop, string(out))
			}
		}
	}

	// UID must not be duplicated after a round-trip.
	out := storeRoundTrip(v30)
	if strings.Count(string(out), "UID:") != 1 {
		t.Fatalf("UID duplicated after round-trip:\n%s", string(out))
	}
}

// ContactObjectETag is a strong validator: identical bodies produce identical
// ETags; any change produces a different one.
func TestETag_StrongValidatorSemantics(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:etag-1\r\nFN:ETag User\r\nEND:VCARD\r\n")
	e1, err := ContactObjectETag(body)
	if err != nil {
		t.Fatalf("etag: %v", err)
	}
	e2, _ := ContactObjectETag(body)
	if e1 != e2 {
		t.Fatalf("ETag not stable for identical body: %q vs %q", e1, e2)
	}
	changed, _ := ContactObjectETag([]byte(strings.Replace(string(body), "ETag User", "Changed", 1)))
	if changed == e1 {
		t.Fatalf("ETag did not change when body changed")
	}
}
