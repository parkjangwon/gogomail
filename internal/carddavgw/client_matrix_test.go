package carddavgw

import (
	"encoding/base64"
	"strings"
	"testing"
)

// This file exercises vCard 3.0 / 4.0 parsing and round-trip fidelity against
// the concrete forms emitted by the major native CardDAV clients:
//
//   - Apple Contacts (macOS / iOS)
//   - DAVx5 (Android)
//   - Mozilla Thunderbird / TbSync
//   - GNOME Contacts / Evolution
//
// The store persists a contact by extracting the inline PHOTO and
// CATEGORIES/GROUP into dedicated columns, storing the remaining vCard, and
// re-merging them on read. storeRoundTrip reproduces that pipeline so the tests
// can assert what a client would receive back after a PUT + GET without needing
// a database.
func storeRoundTrip(vcard []byte) []byte {
	clean, mediaType, data, _ := extractPhotoFromVCard(vcard)
	clean, cats, group, _ := extractCategoriesAndGroupFromVCard(clean)
	out := mergePhotoIntoVCard(clean, data, mediaType)
	out = mergeCategoriesAndGroupIntoVCard(out, cats, group)
	return out
}

func mustValidVCard(t *testing.T, vcard []byte) VCardMetadata {
	t.Helper()
	meta, err := ValidateVCardObject(vcard)
	if err != nil {
		t.Fatalf("vCard failed validation: %v\n---\n%s", err, string(vcard))
	}
	return meta
}

// contentLine unfolds the stored vCard and returns the first content line whose
// property name matches, along with its parsed parameters/value.
func contentLine(t *testing.T, vcard []byte, property string) (vCardContentLine, bool) {
	t.Helper()
	lines, err := unfoldVCardLines(string(vcard))
	if err != nil {
		t.Fatalf("unfold failed: %v", err)
	}
	for _, line := range lines {
		parsed, err := parseVCardContentLineParts(line)
		if err != nil {
			continue
		}
		if parsed.Name == strings.ToUpper(property) {
			return parsed, true
		}
	}
	return vCardContentLine{}, false
}

// --- vCard version matrix: 3.0 vs 4.0 property parsing ----------------------

func TestVCardMatrix_Version30And40_CoreProperties(t *testing.T) {
	t.Parallel()

	cards := map[string]string{
		"3.0": "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:matrix-30\r\nFN:Jane Roe\r\n" +
			"N:Roe;Jane;;;\r\n" +
			"ORG:Example, Inc.;Engineering\r\n" +
			"TITLE:Staff Engineer\r\n" +
			"ADR;TYPE=WORK:;;1 Example St;Seoul;;03187;KR\r\n" +
			"TEL;TYPE=WORK,VOICE:+82-2-555-0100\r\n" +
			"EMAIL;TYPE=INTERNET,WORK:jane@example.com\r\n" +
			"END:VCARD\r\n",
		"4.0": "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:urn:uuid:matrix-40\r\nFN:Jane Roe\r\n" +
			"N:Roe;Jane;;;\r\n" +
			"ORG:Example\\, Inc.;Engineering\r\n" +
			"TITLE:Staff Engineer\r\n" +
			"ADR;TYPE=work:;;1 Example St;Seoul;;03187;KR\r\n" +
			"TEL;TYPE=\"work,voice\";VALUE=uri:tel:+82-2-555-0100\r\n" +
			"EMAIL;TYPE=work:jane@example.com\r\n" +
			"END:VCARD\r\n",
	}
	for version, body := range cards {
		body := body
		version := version
		t.Run(version, func(t *testing.T) {
			t.Parallel()
			meta := mustValidVCard(t, []byte(body))
			if meta.Version != version {
				t.Fatalf("version = %q, want %q", meta.Version, version)
			}
			out := storeRoundTrip([]byte(body))
			mustValidVCard(t, out)
			for _, prop := range []string{"N", "ORG", "TITLE", "ADR", "TEL", "EMAIL"} {
				if _, ok := contentLine(t, out, prop); !ok {
					t.Fatalf("property %s lost in round-trip:\n%s", prop, string(out))
				}
			}
		})
	}
}

// --- Apple Contacts quirks --------------------------------------------------

// Apple sends grouped item properties (item1.EMAIL + item1.X-ABLabel) and
// repeated TYPE parameters in vCard 3.0. Both must parse and round-trip.
func TestAppleContacts_ItemGroupingAndRepeatedType(t *testing.T) {
	t.Parallel()

	body := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:apple-group\r\nFN:Tim Apple\r\n" +
		"item1.EMAIL;TYPE=INTERNET;TYPE=pref:tim@example.com\r\n" +
		"item1.X-ABLabel:_$!<Work>!$_\r\n" +
		"TEL;TYPE=CELL;TYPE=VOICE:+1-555-0100\r\n" +
		"END:VCARD\r\n"
	mustValidVCard(t, []byte(body))

	email, ok := contentLine(t, []byte(body), "EMAIL")
	if !ok {
		t.Fatal("EMAIL not found")
	}
	types := email.Params["TYPE"]
	if len(types) != 2 || types[0] != "INTERNET" || types[1] != "pref" {
		t.Fatalf("repeated TYPE params = %v, want [INTERNET pref]", types)
	}

	out := storeRoundTrip([]byte(body))
	if !strings.Contains(string(out), "item1.X-ABLabel:_$!<Work>!$_") {
		t.Fatalf("Apple item grouping label lost:\n%s", string(out))
	}
}

// Apple/iOS sometimes quotes a comma-bearing TYPE value: TYPE="HOME,VOICE".
// The comma inside quotes must be treated as literal, not a value separator.
func TestAppleContacts_QuotedTypeParameter(t *testing.T) {
	t.Parallel()

	line, err := parseVCardContentLineParts(`TEL;TYPE="HOME,VOICE":+1-555-0100`)
	if err != nil {
		t.Fatalf("parse quoted TYPE: %v", err)
	}
	if got := line.Params["TYPE"]; len(got) != 1 || got[0] != "HOME,VOICE" {
		t.Fatalf("quoted TYPE = %v, want [HOME,VOICE]", got)
	}
	// Unquoted comma form must split into two values.
	line2, err := parseVCardContentLineParts(`TEL;TYPE=HOME,VOICE:+1-555-0100`)
	if err != nil {
		t.Fatalf("parse unquoted TYPE: %v", err)
	}
	if got := line2.Params["TYPE"]; len(got) != 2 || got[0] != "HOME" || got[1] != "VOICE" {
		t.Fatalf("unquoted TYPE = %v, want [HOME VOICE]", got)
	}
}

// Apple Contacts (3.0), DAVx5, and GNOME emit inline photos as
// PHOTO;ENCODING=b;TYPE=JPEG:<base64>. Regression for a defect where this form
// was stripped from the stored vCard yet never captured — silently dropping the
// contact photo on every PUT.
func TestAppleContacts_InlinePhoto30RoundTrips(t *testing.T) {
	t.Parallel()

	raw := []byte("\xff\xd8\xff\xe0\x00\x10JFIFfakejpegbody")
	img := base64.StdEncoding.EncodeToString(raw)
	for _, encoding := range []string{"b", "B", "BASE64"} {
		encoding := encoding
		t.Run("ENCODING="+encoding, func(t *testing.T) {
			t.Parallel()
			body := []byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:apple-photo\r\nFN:Photo User\r\n" +
				"PHOTO;ENCODING=" + encoding + ";TYPE=JPEG:" + img + "\r\nEND:VCARD\r\n")

			clean, mediaType, data, err := extractPhotoFromVCard(body)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if mediaType != "image/jpeg" {
				t.Fatalf("mediaType = %q, want image/jpeg", mediaType)
			}
			if string(data) != string(raw) {
				t.Fatalf("photo bytes not preserved: got %d bytes", len(data))
			}
			if strings.Contains(string(clean), "PHOTO") {
				t.Fatalf("PHOTO not removed from stored body:\n%s", string(clean))
			}
			out := mergePhotoIntoVCard(clean, data, mediaType)
			mustValidVCard(t, out)
			if !strings.Contains(string(out), "PHOTO;ENCODING=b;TYPE=JPEG:") {
				t.Fatalf("merged PHOTO not in interoperable form:\n%s", string(out))
			}
		})
	}
}

// --- DAVx5 (Android) quirks -------------------------------------------------

// DAVx5 emits vCard 4.0 with PHOTO as a data: URI. Regression for a defect
// where the media type kept the ";base64" suffix (image/jpeg;base64).
func TestDAVx5_Photo40DataURIRoundTrips(t *testing.T) {
	t.Parallel()

	raw := []byte("\x89PNG\r\n\x1a\nfakepngbody")
	img := base64.StdEncoding.EncodeToString(raw)
	body := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:davx5-photo\r\nFN:DAVx5 User\r\n" +
		"PHOTO:data:image/png;base64," + img + "\r\nEND:VCARD\r\n")

	clean, mediaType, data, err := extractPhotoFromVCard(body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if mediaType != "image/png" {
		t.Fatalf("mediaType = %q, want image/png (no base64 suffix)", mediaType)
	}
	if string(data) != string(raw) {
		t.Fatalf("photo bytes not preserved")
	}
	out := mergePhotoIntoVCard(clean, data, mediaType)
	mustValidVCard(t, out)
	if !strings.Contains(string(out), "TYPE=PNG:") {
		t.Fatalf("merged PHOTO subtype wrong:\n%s", string(out))
	}
}

// DAVx5 relies on parameter and property ordering being preserved. The stored
// card must keep non-extracted properties in their original order.
func TestDAVx5_PropertyOrderingPreserved(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:davx5-order\r\nFN:Order User\r\n" +
		"N:User;Order;;;\r\n" +
		"NICKNAME:ordr\r\n" +
		"TEL;TYPE=cell;PREF=1:+1-555-0100\r\n" +
		"EMAIL;TYPE=home;PREF=1:order@example.com\r\n" +
		"URL:https://example.com\r\n" +
		"END:VCARD\r\n")
	out := storeRoundTrip(body)
	mustValidVCard(t, out)

	wantOrder := []string{"VERSION", "UID", "FN", "N", "NICKNAME", "TEL", "EMAIL", "URL"}
	lines, _ := unfoldVCardLines(string(out))
	var gotOrder []string
	for _, line := range lines {
		parsed, err := parseVCardContentLineParts(line)
		if err != nil {
			continue
		}
		switch parsed.Name {
		case "BEGIN", "END":
			continue
		}
		gotOrder = append(gotOrder, parsed.Name)
	}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("property order = %v, want %v", gotOrder, wantOrder)
	}
}

// --- Thunderbird quirks -----------------------------------------------------

// Thunderbird historically emits vCard 3.0 with QUOTED-PRINTABLE encoded
// non-ASCII values. The server must accept and preserve these without mangling.
func TestThunderbird_QuotedPrintable30Accepted(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:tb-qp\r\n" +
		"FN;CHARSET=UTF-8;ENCODING=QUOTED-PRINTABLE:=EC=95=88=EB=85=95\r\n" +
		"NOTE;ENCODING=QUOTED-PRINTABLE:line1=0D=0Aline2\r\n" +
		"END:VCARD\r\n")
	mustValidVCard(t, []byte(body))
	out := storeRoundTrip(body)
	mustValidVCard(t, out)
	if !strings.Contains(string(out), "=EC=95=88=EB=85=95") {
		t.Fatalf("quoted-printable value mangled:\n%s", string(out))
	}
}

// Thunderbird's vCard 4.0 support is limited; it still round-trips basic cards.
func TestThunderbird_Vcard40Basic(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:tb-40\r\nFN:TB User\r\n" +
		"EMAIL:tb@example.com\r\nEND:VCARD\r\n")
	meta := mustValidVCard(t, body)
	if meta.Version != "4.0" {
		t.Fatalf("version = %q", meta.Version)
	}
	out := storeRoundTrip(body)
	mustValidVCard(t, out)
}

// --- GNOME Contacts / Evolution quirks --------------------------------------

// GNOME/Evolution fold long lines aggressively. Unfolding must reassemble the
// exact original value with no spurious characters.
func TestGNOME_LineFoldingUnfoldsExactly(t *testing.T) {
	t.Parallel()

	body := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:gnome-fold\r\nFN:GNOME User\r\n" +
		"NOTE:This is a long note that was folded\r\n  across three\r\n  physical lines.\r\n" +
		"END:VCARD\r\n"
	lines, err := unfoldVCardLines(body)
	if err != nil {
		t.Fatalf("unfold: %v", err)
	}
	var note string
	for _, line := range lines {
		if strings.HasPrefix(line, "NOTE:") {
			note = strings.TrimPrefix(line, "NOTE:")
		}
	}
	want := "This is a long note that was folded across three physical lines."
	if note != want {
		t.Fatalf("unfolded NOTE = %q, want %q", note, want)
	}
}

// GNOME emits CATEGORIES; these are extracted to a column and must re-merge.
func TestGNOME_CategoriesRoundTrip(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:gnome-cat\r\nFN:Cat User\r\n" +
		"CATEGORIES:Friends,Family,VIP\r\nEND:VCARD\r\n")
	clean, cats, _, err := extractCategoriesAndGroupFromVCard(body)
	if err != nil {
		t.Fatalf("extract categories: %v", err)
	}
	if len(cats) != 3 || cats[0] != "Friends" || cats[2] != "VIP" {
		t.Fatalf("categories = %v", cats)
	}
	out := mergeCategoriesAndGroupIntoVCard(clean, cats, "")
	mustValidVCard(t, out)
	if !strings.Contains(string(out), "CATEGORIES:Friends,Family,VIP") {
		t.Fatalf("categories not restored:\n%s", string(out))
	}
}

// --- X- property preservation (all clients) ---------------------------------

// Unknown X- properties must round-trip losslessly through the store pipeline.
func TestXProperties_PreservedAcrossRoundTrip(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:xprops\r\nFN:X User\r\n" +
		"X-CUSTOM-FIELD:custom value\r\n" +
		"X-PHONETIC-FIRST-NAME:Jane\r\n" +
		"X-SOCIALPROFILE;TYPE=twitter:https://twitter.com/example\r\n" +
		"END:VCARD\r\n")
	out := storeRoundTrip(body)
	mustValidVCard(t, out)
	for _, want := range []string{
		"X-CUSTOM-FIELD:custom value",
		"X-PHONETIC-FIRST-NAME:Jane",
		"X-SOCIALPROFILE;TYPE=twitter:https://twitter.com/example",
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("X- property lost: %q\n%s", want, string(out))
		}
	}
}

// --- Photo reference preservation (vCard 4.0) -------------------------------

// A PHOTO that is an external URI reference (not inline base64) must be left in
// the vCard body, not extracted and lost. Regression for a defect where any
// PHOTO line was stripped even when no data could be captured.
func TestPhotoURIReference_Preserved(t *testing.T) {
	t.Parallel()

	body := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:photo-uri\r\nFN:URI User\r\n" +
		"PHOTO;MEDIATYPE=image/jpeg:https://example.com/photo.jpg\r\nEND:VCARD\r\n")
	clean, mediaType, data, err := extractPhotoFromVCard(body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(data) != 0 || mediaType != "" {
		t.Fatalf("reference photo wrongly extracted: mediaType=%q dataLen=%d", mediaType, len(data))
	}
	if !strings.Contains(string(clean), "PHOTO;MEDIATYPE=image/jpeg:https://example.com/photo.jpg") {
		t.Fatalf("reference PHOTO removed from body:\n%s", string(clean))
	}
	out := storeRoundTrip(body)
	mustValidVCard(t, out)
}

// A PHOTO whose inline payload exceeds MaxPhotoBytes must be left in place
// rather than silently dropped.
func TestPhotoOversizeInline_LeftInPlace(t *testing.T) {
	t.Parallel()

	big := base64.StdEncoding.EncodeToString(make([]byte, MaxPhotoBytes+1))
	body := []byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:photo-big\r\nFN:Big Photo\r\n" +
		"PHOTO;ENCODING=b;TYPE=JPEG:" + big + "\r\nEND:VCARD\r\n")
	clean, mediaType, data, err := extractPhotoFromVCard(body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(data) != 0 || mediaType != "" {
		t.Fatalf("oversize photo should not be captured: mediaType=%q dataLen=%d", mediaType, len(data))
	}
	if !strings.Contains(string(clean), "PHOTO;ENCODING=b;TYPE=JPEG:") {
		t.Fatalf("oversize PHOTO wrongly dropped from body")
	}
}

// --- Photo fold width -------------------------------------------------------

// Re-merged inline photos must be folded so no physical line exceeds the
// content-line octet cap; long base64 payloads previously used a single fold.
func TestPhoto_MergedValueIsFolded(t *testing.T) {
	t.Parallel()

	raw := make([]byte, 600)
	for i := range raw {
		raw[i] = byte(i)
	}
	merged := mergePhotoIntoVCard(
		[]byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:fold\r\nFN:Fold User\r\nEND:VCARD\r\n"),
		raw, "image/jpeg")
	mustValidVCard(t, merged)
	for _, physical := range strings.Split(string(merged), "\r\n") {
		if len(physical) > 75 {
			t.Fatalf("physical line exceeds 75 octets (%d): %q", len(physical), physical)
		}
	}
}
