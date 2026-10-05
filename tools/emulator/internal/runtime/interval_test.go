package runtime

import "testing"

func TestReadIntervalAndValidate(t *testing.T) {
	hdr := func(typ, extra string) string {
		return "{{/*\n  Trigger type: `" + typ + "`\n  Group: `Utility`\n" + extra + "*/}}"
	}
	tests := []struct {
		name    string
		src     string
		wantN   int
		wantOK  bool
		wantErr string
	}{
		{"hourly", hdr("Hourly interval", "  Interval: `12`\n"), 12, true, ""},
		{"minute", hdr("Minute interval", "  Interval: `15`\n"), 15, true, ""},
		{"type is case-insensitive", hdr("hourly Interval", "  Interval: `2`\n"), 2, true, ""},
		{"missing", hdr("Hourly interval", ""), 0, false,
			"header Interval: a `Hourly interval` trigger needs an `Interval:` line"},
		{"zero", hdr("Hourly interval", "  Interval: `0`\n"), 0, false,
			"header Interval: `0` isn't a positive whole number"},
		{"negative", hdr("Minute interval", "  Interval: `-5`\n"), 0, false,
			"header Interval: `-5` isn't a positive whole number"},
		{"fractional", hdr("Hourly interval", "  Interval: `1.5`\n"), 0, false,
			"header Interval: `1.5` isn't a positive whole number"},
		{"hourly max", hdr("Hourly interval", "  Interval: `744`\n"), 744, true, ""},
		{"hourly over", hdr("Hourly interval", "  Interval: `745`\n"), 745, true,
			"header Interval: `745` isn't between 1 and 744 hours"},
		{"minute min", hdr("Minute interval", "  Interval: `5`\n"), 5, true, ""},
		{"minute under", hdr("Minute interval", "  Interval: `4`\n"), 4, true,
			"header Interval: `4` isn't between 5 and 44640 minutes"},
		{"minute near max", hdr("Minute interval", "  Interval: `44639`\n"), 44639, true, ""},
		{"minute max is whole hours", hdr("Minute interval", "  Interval: `44640`\n"), 44640, true,
			"header Interval: `44640` minutes is whole hours: use `Hourly interval`"},
		{"minute 60 is whole hours", hdr("Minute interval", "  Interval: `60`\n"), 60, true,
			"header Interval: `60` minutes is whole hours: use `Hourly interval`"},
		{"minute over", hdr("Minute interval", "  Interval: `44641`\n"), 44641, true,
			"header Interval: `44641` isn't between 5 and 44640 minutes"},
		{"a command ignores the line", hdr("Command", "  Trigger: `x`\n  Interval: `0`\n"), 0, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, ok := ReadInterval(tt.src)
			if n != tt.wantN || ok != tt.wantOK {
				t.Errorf("ReadInterval = %d, %v; want %d, %v", n, ok, tt.wantN, tt.wantOK)
			}
			err := ValidateHeader(tt.src)
			got := ""
			if err != nil {
				got = err.Error()
			}
			if got != tt.wantErr {
				t.Errorf("ValidateHeader error = %q; want %q", got, tt.wantErr)
			}
		})
	}
}
