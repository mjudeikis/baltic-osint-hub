package api

import "testing"

// A headline starting with "=" is a formula to Excel; the export must not
// hand a downloader a spreadsheet that executes what a publisher wrote.
func TestCSVSafeNeutralisesFormulas(t *testing.T) {
	cases := map[string]string{
		"=HYPERLINK(\"http://x\")": "'=HYPERLINK(\"http://x\")",
		"+1+1":                     "'+1+1",
		"-2":                       "'-2",
		"@SUM(A1)":                 "'@SUM(A1)",
		"\tcmd":                    "'\tcmd",
		"\rcmd":                    "'\rcmd",
		"Plain headline":           "Plain headline",
		"":                         "",
		"https://example.test":     "https://example.test",
	}
	for in, want := range cases {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}
