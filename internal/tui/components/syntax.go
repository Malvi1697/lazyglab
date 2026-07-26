package components

import (
	"image/color"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Highlight colours a line of source code, choosing the language from the file's
// path. A path with no known language, or anything chroma cannot tokenise, comes
// back exactly as it went in — a diff you cannot read is worse than a plain one.
//
// The colours come from the terminal's own palette by index, like the rest of the
// UI, so highlighted code sits inside your theme rather than against it. Only six
// token kinds are coloured: enough to tell strings from keywords from comments,
// while leaving most of a line at the terminal's foreground so the eye still has
// somewhere to rest.
//
// Lines are highlighted one at a time, which is what a diff gives us — a lexer
// sees no more than the line, so a string opened on the line before is not
// carried over. That costs the odd mis-tinted continuation and buys not having to
// reconstruct both sides of the file.
func Highlight(path, line string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}
	lexer := lexerFor(path)
	if lexer == nil {
		return line
	}

	it, err := lexer.Tokenise(nil, line)
	if err != nil {
		return line
	}

	var b strings.Builder
	for _, token := range it.Tokens() {
		if c, ok := tokenColor(token.Type); ok {
			b.WriteString(lipgloss.NewStyle().Foreground(c).Render(token.Value))
			continue
		}
		b.WriteString(token.Value)
	}
	return b.String()
}

// lexerCache remembers the lexer chosen for a path. Matching is by filename, so
// the same file's every line resolves once instead of on each render.
var lexerCache sync.Map // path (string) -> chroma.Lexer or nil

func lexerFor(path string) chroma.Lexer {
	if cached, ok := lexerCache.Load(path); ok {
		lexer, _ := cached.(chroma.Lexer)
		return lexer
	}

	lexer := lexers.Match(path)
	if lexer != nil {
		// Without this a line's leading whitespace is dropped from the token stream,
		// and every line of an indented block would come back flush left.
		lexer = chroma.Coalesce(lexer)
	}
	lexerCache.Store(path, lexer)
	return lexer
}

// Syntax colours, by ANSI index like the rest of the palette.
var (
	colorSyntaxComment = lipgloss.Color("8") // grey, as metadata is
	colorSyntaxKeyword = lipgloss.Color("5") // magenta
	colorSyntaxString  = lipgloss.Color("2") // green
	colorSyntaxNumber  = lipgloss.Color("6") // cyan
	colorSyntaxName    = lipgloss.Color("4") // blue: functions, classes
	colorSyntaxType    = lipgloss.Color("3") // yellow: types, decorators, tags
)

// tokenColor maps a chroma token to a palette entry, or reports that the token
// keeps the terminal's foreground.
func tokenColor(t chroma.TokenType) (color.Color, bool) {
	switch t.Category() {
	case chroma.Comment:
		return colorSyntaxComment, true
	case chroma.Keyword:
		return colorSyntaxKeyword, true
	case chroma.Literal:
		switch t.SubCategory() {
		case chroma.LiteralString:
			return colorSyntaxString, true
		case chroma.LiteralNumber:
			return colorSyntaxNumber, true
		}
		return colorSyntaxString, true
	case chroma.Name:
		switch t {
		case chroma.NameFunction, chroma.NameClass, chroma.NameFunctionMagic:
			return colorSyntaxName, true
		case chroma.NameBuiltin, chroma.NameBuiltinPseudo:
			return colorSyntaxNumber, true
		case chroma.NameDecorator, chroma.NameAttribute, chroma.NameTag, chroma.NameConstant:
			return colorSyntaxType, true
		}
		return nil, false
	case chroma.Error:
		// A lexer failing on one line of a diff is expected; it is not the user's
		// problem and must not be painted red.
		return nil, false
	}
	return nil, false
}
