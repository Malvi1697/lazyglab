package util

import "regexp"

// ansiRegex matches all known terminal escape sequences and harmful control
// characters to prevent injection via untrusted GitLab data:
//
//   - CSI sequences:  ESC [ ... <final byte>  (colors, cursor, erase, scroll, etc.)
//   - OSC sequences:  ESC ] ... (BEL | ESC \)  (title, clipboard via OSC 52, etc.)
//   - DCS sequences:  ESC P ... ESC \           (device control strings)
//   - APC sequences:  ESC _ ... ESC \           (application program commands)
//   - SOS sequences:  ESC X ... ESC \           (start of string)
//   - PM  sequences:  ESC ^ ... ESC \           (privacy messages)
//   - ESC + single character:  ESC <0x20-0x7E>  (e.g. ESC H, ESC 7, ESC c)
//   - C0 control characters:   BEL, BS, and other harmful controls (0x00-0x08, 0x0B-0x0C, 0x0E-0x1F, 0x7F)
//     (TAB, LF, CR are preserved as they are needed for normal text rendering)
var ansiRegex = regexp.MustCompile(
	// OSC: ESC ] ... terminated by BEL or ST (ESC \)
	`\x1b\][\x20-\x7e]*(?:\x07|\x1b\\)` +
		`|` +
		// DCS / APC / SOS / PM: ESC (P|_|X|^) ... terminated by ST (ESC \)
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

// StripANSI removes ANSI escape sequences and harmful control characters from
// a string. This prevents malicious GitLab data (project names, MR titles, etc.)
// from injecting terminal control codes.
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}
