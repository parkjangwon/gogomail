package imapimport

import (
	"testing"
	"time"
)

func TestQuoteIMAP(t *testing.T) {
	if got := quoteIMAP(`He said "hi"\bye`); got != `"He said \"hi\"\\bye"` {
		t.Errorf("quoteIMAP = %q", got)
	}
}

func TestTrailingLiteral(t *testing.T) {
	if n, ok := trailingLiteral("* 1 FETCH (BODY[] {12}"); !ok || n != 12 {
		t.Errorf("trailingLiteral = %d,%v want 12,true", n, ok)
	}
	if n, ok := trailingLiteral("BODY[] {34+}"); !ok || n != 34 {
		t.Errorf("non-sync literal = %d,%v want 34,true", n, ok)
	}
	if _, ok := trailingLiteral("no literal here"); ok {
		t.Error("expected no literal")
	}
}

func TestParseListLine(t *testing.T) {
	cases := []struct {
		line      string
		wantName  string
		wantDelim string
		wantOK    bool
	}{
		{`* LIST (\HasNoChildren) "/" "INBOX"`, "INBOX", "/", true},
		{`* LIST (\Noselect \HasChildren) "/" "[Gmail]"`, "[Gmail]", "/", true},
		{`* LIST () "." Work.Projects`, "Work.Projects", ".", true},
		{`* LIST (\All) "/" "[Gmail]/All Mail"`, "[Gmail]/All Mail", "/", true},
		{`a001 OK LIST completed`, "", "", false},
	}
	for _, tc := range cases {
		f, ok := parseListLine(tc.line)
		if ok != tc.wantOK {
			t.Errorf("parseListLine(%q) ok=%v want %v", tc.line, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if f.Name != tc.wantName {
			t.Errorf("parseListLine(%q) name=%q want %q", tc.line, f.Name, tc.wantName)
		}
		if f.Delimiter != tc.wantDelim {
			t.Errorf("parseListLine(%q) delim=%q want %q", tc.line, f.Delimiter, tc.wantDelim)
		}
	}
}

func TestParseListLineNoselectAttribute(t *testing.T) {
	f, ok := parseListLine(`* LIST (\Noselect) "/" "[Gmail]"`)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if f.Selectable() {
		t.Error("\\Noselect folder should not be selectable")
	}
}

func TestParseStatusMessages(t *testing.T) {
	if n, ok := parseStatusMessages(`* STATUS "INBOX" (MESSAGES 42)`); !ok || n != 42 {
		t.Errorf("parseStatusMessages = %d,%v want 42,true", n, ok)
	}
	if _, ok := parseStatusMessages(`* STATUS "INBOX" (UIDNEXT 5)`); ok {
		t.Error("expected no MESSAGES match")
	}
}

func TestParseExists(t *testing.T) {
	if n, ok := parseExists("* 17 EXISTS"); !ok || n != 17 {
		t.Errorf("parseExists = %d,%v want 17,true", n, ok)
	}
	if _, ok := parseExists("* OK [UIDVALIDITY 1]"); ok {
		t.Error("expected no EXISTS match")
	}
}

func TestParseFetchMessage(t *testing.T) {
	body := "Subject: Hi\r\nMessage-ID: <m1@example.com>\r\n\r\nHello"
	line := "* 1 FETCH (UID 5 FLAGS (\\Seen \\Flagged) INTERNALDATE \"01-Jan-2024 10:30:00 +0000\" BODY[] {" +
		itoa(len(body)) + "}\n" + body + ")"
	msg, ok := parseFetchMessage(line)
	if !ok {
		t.Fatal("parseFetchMessage failed")
	}
	if msg.UID != 5 {
		t.Errorf("UID = %d want 5", msg.UID)
	}
	if len(msg.Flags) != 2 || msg.Flags[0] != `\Seen` || msg.Flags[1] != `\Flagged` {
		t.Errorf("flags = %v want [\\Seen \\Flagged]", msg.Flags)
	}
	wantDate := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	if !msg.InternalDate.Equal(wantDate) {
		t.Errorf("internal date = %v want %v", msg.InternalDate, wantDate)
	}
	if string(msg.Raw) != body {
		t.Errorf("raw = %q want %q", msg.Raw, body)
	}
}

func TestExtractInternalDateSpacePadded(t *testing.T) {
	d, ok := extractInternalDate(`... INTERNALDATE " 5-Mar-2023 08:00:00 +0900" ...`)
	if !ok {
		t.Fatal("expected date parse")
	}
	if d.Day() != 5 || d.Month() != time.March {
		t.Errorf("date = %v want 5 March", d)
	}
}

// itoa avoids importing strconv in this test file's literal construction.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
