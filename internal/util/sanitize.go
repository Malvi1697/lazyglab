package util

import "regexp"

// ansiRegex matches all known terminal escape sequences and harmful control characters
// to prevent injection via untrusted GitLab data: - CSI sequences: ESC [ ...
var ansiRegex = regexp.MustCompile(
	// OSC: ESC ] ...
	`\x1b\][\x20-\x7e]*(?:\x07|\x1b\\)` +
		`|` +
		// DCS / APC / SOS / PM: ESC (P|_|X|^) ...
		`\x1b[P_X\^][\s\S]*?\x1b\\` +
		`|` +
		// CSI: ESC [ <parameter bytes> <intermediate bytes> <final byte 0x40-0x7E>
		`\x1b\[[\x30-\x3f]*[\x20-\x2f]*[\x40-\x7e]` +
		`|` +
		// ESC + single printable character (two-char escape sequences)
		`\x1b[\x20-\x7e]` +
		`|` +
		// Harmful C0 control characters (exclude \t=0x09, \n=0x0A, \r=0x0D)
		`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`,
)

// StripANSI removes ANSI escape sequences and harmful control characters from a string.
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}
