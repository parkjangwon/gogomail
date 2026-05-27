package httpapi

// sanitizeCSVCell defends against CSV formula injection (also called CSV injection).
// Spreadsheet applications such as Excel and LibreOffice Calc interpret cell values
// that begin with '=', '+', '-', or '@' as formulas, even inside a quoted CSV field.
// A malicious user who sets their display name to =HYPERLINK("http://evil.com","click")
// can cause arbitrary formula execution when an admin exports and opens the file.
//
// Mitigation: prefix any dangerous cell with a tab character (\t).  Most spreadsheet
// applications treat a leading tab as plain text rather than a formula prefix.
// The tab is invisible in the rendered cell but prevents formula evaluation.
func sanitizeCSVCell(s string) string {
	if len(s) == 0 {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "\t" + s
	}
	return s
}
