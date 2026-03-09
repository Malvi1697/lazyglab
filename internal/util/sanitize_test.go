package util

import "testing"

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Basic passthrough
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "plain text with preserved whitespace",
			input: "line1\nline2\ttab\rcarriage",
			want:  "line1\nline2\ttab\rcarriage",
		},

		// CSI sequences
		{
			name:  "CSI simple color (red)",
			input: "\x1b[31mred text\x1b[0m",
			want:  "red text",
		},
		{
			name:  "CSI bold+color",
			input: "\x1b[1;34mbold blue\x1b[0m",
			want:  "bold blue",
		},
		{
			name:  "CSI multiple sequences inline",
			input: "\x1b[32mgreen\x1b[0m and \x1b[33myellow\x1b[0m",
			want:  "green and yellow",
		},
		{
			name:  "CSI cursor movement",
			input: "\x1b[2Jclear screen\x1b[H",
			want:  "clear screen",
		},
		{
			name:  "CSI 256-color",
			input: "\x1b[38;5;196mcolor256\x1b[0m",
			want:  "color256",
		},
		{
			name:  "CSI RGB truecolor",
			input: "\x1b[38;2;255;100;0mtruecolor\x1b[0m",
			want:  "truecolor",
		},

		// OSC sequences
		{
			name:  "OSC set title (BEL terminated)",
			input: "\x1b]0;my title\x07visible",
			want:  "visible",
		},
		{
			name:  "OSC set title (ST terminated)",
			input: "\x1b]0;my title\x1b\\visible",
			want:  "visible",
		},
		{
			name:  "OSC 52 clipboard",
			input: "\x1b]52;c;SGVsbG8=\x1b\\visible",
			want:  "visible",
		},

		// DCS sequences
		{
			name:  "DCS sequence",
			input: "\x1bPdevice control\x1b\\visible",
			want:  "visible",
		},

		// APC sequences
		{
			name:  "APC sequence",
			input: "\x1b_app command\x1b\\visible",
			want:  "visible",
		},

		// SOS sequences
		{
			name:  "SOS sequence",
			input: "\x1bXstart of string\x1b\\visible",
			want:  "visible",
		},

		// PM sequences
		{
			name:  "PM sequence",
			input: "\x1b^privacy message\x1b\\visible",
			want:  "visible",
		},

		// ESC + single character
		{
			name:  "ESC single char (save cursor)",
			input: "\x1b7text\x1b8",
			want:  "text",
		},
		{
			name:  "ESC reset (ESC c)",
			input: "\x1bctext after reset",
			want:  "text after reset",
		},

		// C0 control characters
		{
			name:  "NUL stripped",
			input: "before\x00after",
			want:  "beforeafter",
		},
		{
			name:  "BEL stripped",
			input: "before\x07after",
			want:  "beforeafter",
		},
		{
			name:  "BS stripped",
			input: "before\x08after",
			want:  "beforeafter",
		},
		{
			name:  "VT stripped",
			input: "before\x0bafter",
			want:  "beforeafter",
		},
		{
			name:  "FF stripped",
			input: "before\x0cafter",
			want:  "beforeafter",
		},
		{
			name:  "DEL stripped",
			input: "before\x7fafter",
			want:  "beforeafter",
		},
		{
			name:  "TAB preserved",
			input: "col1\tcol2",
			want:  "col1\tcol2",
		},
		{
			name:  "LF preserved",
			input: "line1\nline2",
			want:  "line1\nline2",
		},
		{
			name:  "CR preserved",
			input: "line1\rline2",
			want:  "line1\rline2",
		},

		// Mixed
		{
			name:  "mixed ANSI and control chars",
			input: "\x1b[1;31m\x07ERROR\x1b[0m: something \x00bad\x1b]0;alert\x07 happened",
			want:  "ERROR: something bad happened",
		},
		{
			name:  "no ANSI codes returns unchanged",
			input: "perfectly normal string 123 !@#",
			want:  "perfectly normal string 123 !@#",
		},
		{
			name:  "unicode text preserved",
			input: "\x1b[36m日本語テスト\x1b[0m",
			want:  "日本語テスト",
		},
		{
			name:  "only ANSI codes yields empty",
			input: "\x1b[31m\x1b[0m",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.input)
			if got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
