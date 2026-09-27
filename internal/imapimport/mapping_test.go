package imapimport

import (
	"reflect"
	"testing"

	"github.com/gogomail/gogomail/internal/imapgw"
)

func TestParseMappingStrategy(t *testing.T) {
	cases := map[string]struct {
		want    MappingStrategy
		wantErr bool
	}{
		"prefix":      {want: MappingPrefix},
		"MERGE":       {want: MappingMerge},
		"flat-labels": {want: MappingFlatLabels},
		"":            {want: MappingPrefix},
		"bogus":       {wantErr: true},
	}
	for in, tc := range cases {
		got, err := ParseMappingStrategy(in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseMappingStrategy(%q) expected error", in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseMappingStrategy(%q) unexpected error: %v", in, err)
		}
		if got != tc.want {
			t.Errorf("ParseMappingStrategy(%q) = %q, want %q", in, got, tc.want)
		}
	}
}

func TestMapFolderPrefix(t *testing.T) {
	cfg := MappingConfig{Strategy: MappingPrefix, Separator: "/", Prefix: "Imported"}

	cases := []struct {
		source   string
		wantDest string
		wantSkip bool
	}{
		{"INBOX", "INBOX", false},
		{"inbox", "INBOX", false},
		{"Work/Projects", "Imported/Work/Projects", false},
		{"Sent", "Imported/Sent", false},
		{"[Gmail]/All Mail", "", true},
	}
	for _, tc := range cases {
		got := MapFolder(cfg, tc.source)
		if got.Skip != tc.wantSkip {
			t.Errorf("MapFolder(prefix, %q).Skip = %v, want %v", tc.source, got.Skip, tc.wantSkip)
		}
		if !tc.wantSkip && got.Destination != tc.wantDest {
			t.Errorf("MapFolder(prefix, %q) = %q, want %q", tc.source, got.Destination, tc.wantDest)
		}
		if got.Label != "" {
			t.Errorf("MapFolder(prefix, %q) unexpected label %q", tc.source, got.Label)
		}
	}
}

func TestMapFolderPrefixDotSeparator(t *testing.T) {
	cfg := MappingConfig{Strategy: MappingPrefix, Separator: "."}
	got := MapFolder(cfg, "Work.Projects.2024")
	if got.Destination != "Work/Projects/2024" {
		t.Errorf("dot-separator mapping = %q, want Work/Projects/2024", got.Destination)
	}
}

func TestMapFolderMerge(t *testing.T) {
	cfg := MappingConfig{Strategy: MappingMerge, MergeTarget: "Archive"}
	for _, src := range []string{"Work/Projects", "Sent", "Random"} {
		got := MapFolder(cfg, src)
		if got.Destination != "Archive" {
			t.Errorf("MapFolder(merge, %q) = %q, want Archive", src, got.Destination)
		}
	}
	// INBOX stays INBOX even under merge.
	if got := MapFolder(cfg, "INBOX"); got.Destination != "INBOX" {
		t.Errorf("merge INBOX = %q, want INBOX", got.Destination)
	}
}

func TestMapFolderMergeDefaultsToInbox(t *testing.T) {
	cfg := MappingConfig{Strategy: MappingMerge}
	if got := MapFolder(cfg, "Sent"); got.Destination != "INBOX" {
		t.Errorf("merge default = %q, want INBOX", got.Destination)
	}
}

func TestMapFolderFlatLabels(t *testing.T) {
	cfg := MappingConfig{Strategy: MappingFlatLabels, Separator: "/"}
	got := MapFolder(cfg, "Work/Projects/Q1")
	if got.Destination != "Q1" {
		t.Errorf("flat-labels dest = %q, want Q1", got.Destination)
	}
	if got.Label != "Work/Projects/Q1" {
		t.Errorf("flat-labels label = %q, want Work/Projects/Q1", got.Label)
	}
}

func TestTranslateFlags(t *testing.T) {
	flags := TranslateFlags([]string{`\Seen`, `\Flagged`, `\Answered`, `\Recent`, `$Forwarded`, "CustomKeyword"})
	if !flags.Read {
		t.Error("\\Seen should map to Read")
	}
	if !flags.Starred {
		t.Error("\\Flagged should map to Starred")
	}
	if !flags.Answered {
		t.Error("\\Answered should map to Answered")
	}
	if !flags.Forwarded {
		t.Error("$Forwarded should map to Forwarded")
	}
	// \Recent must be dropped (session-scoped, not persistable).
	// CustomKeyword should survive as a keyword.
	found := false
	for _, k := range flags.Keywords {
		if k == "CustomKeyword" {
			found = true
		}
		if k == `\Recent` {
			t.Error("\\Recent must not become a keyword")
		}
	}
	if !found {
		t.Errorf("CustomKeyword missing from keywords: %v", flags.Keywords)
	}
}

func TestTranslateFlagsEmpty(t *testing.T) {
	flags := TranslateFlags(nil)
	want := imapgw.MessageFlags{}
	if !reflect.DeepEqual(flags, want) {
		t.Errorf("TranslateFlags(nil) = %+v, want zero value", flags)
	}
}

func TestTranslateFlagsExtraKeywords(t *testing.T) {
	flags := TranslateFlags([]string{`\Seen`}, "Label_Work")
	found := false
	for _, k := range flags.Keywords {
		if k == "Label_Work" {
			found = true
		}
	}
	if !found {
		t.Errorf("extra keyword Label_Work missing: %v", flags.Keywords)
	}
}

func TestIdempotencyKey(t *testing.T) {
	cases := map[string]string{
		"<abc@example.com>":   "abc@example.com",
		"  <x@y>  ":           "x@y",
		"noangle@example.com": "noangle@example.com",
		"":                    "",
	}
	for in, want := range cases {
		if got := IdempotencyKey(in); got != want {
			t.Errorf("IdempotencyKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeriveIdempotencyKey(t *testing.T) {
	// With a Message-ID present, it wins.
	if got := DeriveIdempotencyKey("<id@host>", []byte("body")); got != "id@host" {
		t.Errorf("DeriveIdempotencyKey with id = %q, want id@host", got)
	}
	// Without a Message-ID, a content hash is produced.
	got := DeriveIdempotencyKey("", []byte("hello"))
	if len(got) < len("sha256:") || got[:7] != "sha256:" {
		t.Errorf("DeriveIdempotencyKey without id = %q, want sha256: prefix", got)
	}
	// Same content → same key.
	if again := DeriveIdempotencyKey("", []byte("hello")); again != got {
		t.Errorf("content key not stable: %q vs %q", got, again)
	}
}

func TestMessageIDFromRaw(t *testing.T) {
	raw := []byte("Subject: Hi\r\nMessage-ID: <abc@example.com>\r\nFrom: a@b\r\n\r\nbody")
	if got := messageIDFromRaw(raw); got != "<abc@example.com>" {
		t.Errorf("messageIDFromRaw = %q, want <abc@example.com>", got)
	}
	// Folded header.
	folded := []byte("Message-ID:\r\n <folded@example.com>\r\n\r\nbody")
	if got := messageIDFromRaw(folded); got != "<folded@example.com>" {
		t.Errorf("messageIDFromRaw folded = %q, want <folded@example.com>", got)
	}
	// No Message-ID.
	if got := messageIDFromRaw([]byte("Subject: x\r\n\r\nbody")); got != "" {
		t.Errorf("messageIDFromRaw none = %q, want empty", got)
	}
}
