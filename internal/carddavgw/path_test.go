package carddavgw

import "testing"

func TestParseResourcePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want ResourcePath
	}{
		{path: "/.well-known/carddav", want: ResourcePath{Kind: ResourceWellKnown}},
		{path: "/carddav/", want: ResourcePath{Kind: ResourceRoot}},
		{path: "/carddav/principals/", want: ResourcePath{Kind: ResourcePrincipalCollection}},
		{path: "/carddav/principals/user-1/", want: ResourcePath{Kind: ResourcePrincipal, UserID: "user-1"}},
		{path: "/carddav/addressbooks/user-1/", want: ResourcePath{Kind: ResourceAddressBookHome, UserID: "user-1"}},
		{path: "/carddav/addressbooks/user-1/personal/", want: ResourcePath{Kind: ResourceAddressBookCollection, UserID: "user-1", AddressBookID: "personal"}},
		{path: "/carddav/addressbooks/user-1/personal/contact-1.vcf", want: ResourcePath{Kind: ResourceContactObject, UserID: "user-1", AddressBookID: "personal", ObjectName: "contact-1.vcf"}},
		{path: "/carddav/addressbooks/user%201/personal/contact%201.vcf", want: ResourcePath{Kind: ResourceContactObject, UserID: "user 1", AddressBookID: "personal", ObjectName: "contact 1.vcf"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			got, err := ParseResourcePath(tc.path)
			if err != nil {
				t.Fatalf("ParseResourcePath(%q) returned error: %v", tc.path, err)
			}
			if got != tc.want {
				t.Fatalf("ParseResourcePath(%q) = %+v, want %+v", tc.path, got, tc.want)
			}
		})
	}
}

func TestParseResourcePathRejectsUnsafeOrUnsupportedPaths(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"/carddav//principals/user-1",
		"/carddav/principals/../user-1",
		"/carddav/principals/user\n1",
		"/carddav/principals/user%2F1",
		"/carddav/principals/user%252F1",
		"/carddav/addressbooks/user%5C1/personal/contact-1.vcf",
		"/carddav/addressbooks/user%255C1/personal/contact-1.vcf",
		"/carddav/addressbooks/user-1/personal/contact-1.txt",
		"/carddav/addressbooks/user-1/personal%2Fcontact-1.vcf",
		"/carddav/addressbooks/user-1/personal%252Fcontact-1.vcf",
		"/carddav/addressbooks/user-1/personal/contact%5C1.vcf",
		"/carddav/addressbooks/user-1/personal/contact%255C1.vcf",
		"/carddav/addressbooks/user-1/personal/folder/contact-1.vcf",
		"/caldav/addressbooks/user-1/personal/contact-1.vcf",
	}
	for _, value := range tests {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseResourcePath(value); err == nil {
				t.Fatalf("ParseResourcePath(%q) error = nil, want rejection", value)
			}
		})
	}
}

func TestParseResourceHrefAcceptsAbsoluteURIPath(t *testing.T) {
	t.Parallel()

	got, err := ParseResourceHref("https://contacts.example.test/carddav/addressbooks/user-1/personal/contact-1.vcf")
	if err != nil {
		t.Fatalf("ParseResourceHref returned error: %v", err)
	}
	want := ResourcePath{Kind: ResourceContactObject, UserID: "user-1", AddressBookID: "personal", ObjectName: "contact-1.vcf"}
	if got != want {
		t.Fatalf("ParseResourceHref = %+v, want %+v", got, want)
	}
}

func TestParseResourceHrefRejectsUnsafeAbsoluteURI(t *testing.T) {
	t.Parallel()

	tests := []string{
		"mailto:user@example.com",
		"https://user@example.test/carddav/addressbooks/user-1/personal/contact-1.vcf",
		"https://contacts.example.test/carddav/addressbooks/user-1/personal/contact-1.vcf?download=1",
		"https://contacts.example.test/carddav/addressbooks/user-1/personal/contact-1.vcf#frag",
		"https://contacts.example.test/carddav/addressbooks/user-1/personal%2Fcontact-1.vcf",
		"https://contacts.example.test/carddav/addressbooks/user-1/personal%252Fcontact-1.vcf",
		"https://contacts.example.test/carddav/addressbooks/user%5C1/personal/contact-1.vcf",
		"https://contacts.example.test/carddav/addressbooks/user%255C1/personal/contact-1.vcf",
	}
	for _, value := range tests {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseResourceHref(value); err == nil {
				t.Fatalf("ParseResourceHref(%q) error = nil, want rejection", value)
			}
		})
	}
}

func TestPathBuildersEscapeSegments(t *testing.T) {
	t.Parallel()

	principal, err := PrincipalPath("user 1")
	if err != nil {
		t.Fatalf("PrincipalPath returned error: %v", err)
	}
	if principal != "/carddav/principals/user%201/" {
		t.Fatalf("principal path = %q", principal)
	}
	object, err := ContactObjectPath("user 1", "personal", "contact 1.vcf")
	if err != nil {
		t.Fatalf("ContactObjectPath returned error: %v", err)
	}
	if object != "/carddav/addressbooks/user%201/personal/contact%201.vcf" {
		t.Fatalf("object path = %q", object)
	}
}
