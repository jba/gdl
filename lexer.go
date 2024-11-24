// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

// TODO: match the lexical properties of go.mod parsing.

// TODO: any token starting with a digit should be interpreted as a number.
// But what about '-'? We want to support -foo as a bareword without the
// the problem that -12a is a bad number.

package gdl

import (
	"bufio"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
)

type lexer struct {
	r      *bufio.Reader
	lineno int
}

func newLexer(r io.Reader) *lexer {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &lexer{r: br, lineno: 1}
}

type tokenKind int

const (
	tokenInvalid tokenKind = iota
	tokenWord
	tokenEOF
)

const (
	betweenTokens = iota
	afterSlash
	afterBackslash
	inComment
	inWord
	inEscapedString
	inEscapedStringAfterBacklash
	inRawString
)

func (l *lexer) nextToken() (tokenKind, string, error) {
	var word []rune
	state := betweenTokens
	for {
		r, sz, err := l.r.ReadRune()
		if err != nil {
			return tokenInvalid, "", fmt.Errorf("%d: %w", l.lineno, err)
		}
		if r == '\n' {
			l.lineno++
		}
		switch state {
		case betweenTokens:
			if r != '\n' && unicode.IsSpace(r) {
				continue
			}
			switch r {
			case '\n', '(', ')', '{', '}':
				return tokenKind(r), "", nil
			case '/':
				state = afterSlash
			case '\\':
				state = afterBackslash
			case '"':
				state = inEscapedString
			case '`':
				state = inRawString
			default:
				state = inWord
				word = []rune{r}
			}

		case afterSlash:
			if r == '/' {
				state = inComment
			} else {
				state = inWord
				word = []rune{r}
			}

		case afterBackslash:
			if r == '\n' {
				// Line continuation.
				state = betweenTokens
			} else {
				state = inWord
				word = []rune{r}
			}

		case inComment:
			if r == '\n' {
				state = betweenTokens
			}

		case inWord:
			if unicode.IsSpace(r) {
				return tokenWord, string(word), nil
			}
			switch r {
			case '(', ')', '{', '}':
				l.ungotten = r
				return tokenWord, string(word), nil
			default:
				word = append(word, r)
			}

		case inRawString:
			if r == '\' {
				return tokenWord, string(word), nil
			} else {
				word = append(word, r)
			}

		case inEscapedString:
			// TODO: too many states here, with \uxxxx and \OOO.
			// Also, we want to use strconv.Unquote.
			

		default:
			panic("bad state")
		}

	}
}

func (l *lexer) nextRune() rune {
	r, _, err := l.r.ReadRune()
	if err != nil {
		l.err = err
		r = unicode.ReplacementChar
	}
	if r == '\n'{
		r.lineno++
	}
	return r
}

// reports whether it saw end
// func (l *lexer) scanUntilRune(end rune) ([]rune, bool) {
// 	for r := l.nextRune(); l.err == nil; r = l.nextRune() {
// 		if r == end {
// 			return true
// 		}
// 		l.word = append(l.word, r)


// 		r, sz, err := l.r.ReadRune()
// 		if err != nil {
			
// 		}
// 	}


	
// }

func skipSpace(s string) string {
	for i, r := range s {
		if r == '\n'  || !unicode.IsSpace(r) {
			return s[i:]
		}
	}
	return ""
}

func (l *lexer) next(s string) (tokenKind, string, string, error) {
	loop:
	for {
		s = skipSpace(s)
		if len(s) == 0 {
			return tokenEOF, "", ""
		}
		c, sz := utf8.DecodeRuneInString(s)
		if c== '\n' {
			l.lineno++
		}
		s= s[sz:]
		switch c {
		case '\n', '(', ')', '{', '}':
			return tokenKind(r), "", s
		case '/':
			if len(s) > 0 && s[0] == '/' {
				for i, r := range s{
					if r == '\n' {
						// This newline is definitely a token.
						s = s[i:]
						continue loop
					}
				}
				return tokenEOF, "", ""
		case '\\':
			s = skipSpace(s)
			if len(s) == 0 {
				return tokenInvalid, "", "", errors.New("backlash at EOF")
			}
			if s[0] == '\n' {
				// Continuation line. The newline is not a token.
				// You can't use \ to continue a word because there must be
				// a non-word rune before it, else it would be part of the word.
				l.lineno++
				s = s[1:]
				continue loop
			}
		case '`':
			start := l.lineno
			for i, r := range s {
				if r == '\n' {l.lineno++}
				else if r == '`' {
					return tokenRawString, s[:i], s[i+1:]
				}
			}
			return tokenInvalid, "", "", fmt.Errorf("unterminated raw string started on line %d", start)
		case '"':
			// Just scan to the end; strconv.Unquote will do the rest.
			start := l.lineno
			backslashed := false
			for i, r := range s {
				if r == '\n' {
					return tokenInvalid, "", "", fmt.Errorf("newline in double-quoted string started on line %d", start)
				}
				if r == '"' && !backslashed{
					return tokenString, s[:i+1], s[i+1:], nil
				}
				if r == '\\' {
					backslashed = !backslashed
				}
			}
			return tokenInvalid, "", "", fmt.Errorf("unterminated double-quoted string started on line %d", start)
		default: // a word
			// TODO: does a comment end a word? A single slash does not.
			var i int
			var r rune
			wordloop:
			for i, r = range s {
				if unicode.IsSpace(r) {break}
				switch r {
				case '\n', '(', ')', '{', '}':
					break wordloop
				}
			}
			return tokenWord, s[:i], s[i:], nil
		}
	}
}
