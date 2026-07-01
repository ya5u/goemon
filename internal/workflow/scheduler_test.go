package workflow

import "testing"

func TestParseValidationVerdict(t *testing.T) {
	cases := []struct {
		name     string
		response string
		wantOK   bool
	}{
		{
			name:     "bare complete",
			response: "COMPLETE",
			wantOK:   true,
		},
		{
			name:     "markdown-wrapped verdict after reasoning",
			response: "The completion criteria are:\n1. metrics collected\nAll conditions are met.\n\n**COMPLETE**",
			wantOK:   true,
		},
		{
			name:     "verdict as a list item",
			response: "- ✅ report generated\n\n**COMPLETE**",
			wantOK:   true,
		},
		{
			name:     "incomplete with reason",
			response: "The report is missing the hostname.\n\nINCOMPLETE: hostname not included",
			wantOK:   false,
		},
		{
			name:     "markdown incomplete",
			response: "Analysis...\n**INCOMPLETE: sources.md not found**",
			wantOK:   false,
		},
		{
			name:     "no verdict token defaults to complete",
			response: "Looks fine to me.",
			wantOK:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := parseValidationVerdict(tc.response)
			if ok != tc.wantOK {
				t.Fatalf("parseValidationVerdict() ok = %v, want %v (reason=%q)", ok, tc.wantOK, reason)
			}
		})
	}
}

func TestParseValidationVerdictReason(t *testing.T) {
	ok, reason := parseValidationVerdict("INCOMPLETE: hostname not included")
	if ok {
		t.Fatal("expected incomplete")
	}
	if reason != "hostname not included" {
		t.Fatalf("reason = %q, want %q", reason, "hostname not included")
	}
}
