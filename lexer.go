// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

// TODO: match the lexical properties of go.mod parsing.

// TODO: record token position for gdlfmt.

// TODO: fuzz the lexer.

// TODO: any token starting with a digit should be interpreted as a number.
// But what about '-'? We want to support -foo as a bareword without the
// the problem that -12a is a bad number.

package gdl

import (
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"
)

type lexer struct {
	s        string
	lineno   int
	ungotten bool
	untok    token
	unerr    error
}

func newLexer(s string) *lexer {
	return &lexer{s: s, lineno: 1}
}

const (
	tokWord   = 'w'
	tokString = 's' // double-quoted or backquoted Go string
	tokEOF    = 'E'
)

type token struct {
	kind rune
	val  string
}

func (l *lexer) peek() rune {
	if !l.ungotten {
		l.untok, l.unerr = l.next()
		l.ungotten = true
	}
	return l.untok.kind
}

func (l *lexer) next() (_ token, err error) {
	if l.ungotten {
		l.ungotten = false
		return l.untok, l.unerr
	}
	s := l.s
	defer func() { l.s = s }()

loop:
	for {
		s = skipSpace(s)
		if len(s) == 0 {
			return token{kind: tokEOF}, nil
		}
		c, sz := utf8.DecodeRuneInString(s)
		if c == '\n' {
			l.lineno++
		}
		switch c {
		case '\n', '(', ')', '{', '}':
			s = s[sz:]
			return token{kind: c}, nil

		case '/':
			// Double slash is a comment to EOL.
			if len(s) > 1 && s[1] == '/' {
				s = s[2:]
				for i, r := range s {
					if r == '\n' {
						// This newline is definitely a token.
						s = s[i:]
						continue loop
					}
				}
				s = s[sz:]
				return token{kind: tokEOF}, nil
			}
			// Single slash starts a word.
			var word string
			word, s = scanWord(s)
			return token{kind: tokWord, val: word}, nil

		case '\\':
			s = skipSpace(s[1:])
			if len(s) == 0 {
				return token{}, errors.New("backlash at EOF")
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
			for i, r := range s[1:] {
				if r == '\n' { // TODO: \r as well?
					l.lineno++
				} else if r == '`' {
					// Include quotes, for strconv.Unquote.
					val := s[:i+2]
					s = s[i+2:]
					return token{kind: tokString, val: val}, nil
				}
			}
			return token{}, fmt.Errorf("unterminated raw string started on line %d", start)

		case '"':
			// Just scan to the end; strconv.Unquote will do the rest.
			start := l.lineno
			backslashed := false
			for i, r := range s[1:] {
				if r == '\n' {
					return token{}, fmt.Errorf("newline in double-quoted string started on line %d", start)
				}
				if r == '"' && !backslashed {
					val := s[:i+2]
					s = s[i+2:]
					return token{kind: tokString, val: val}, nil
				}
				if backslashed {
					backslashed = false
				} else if r == '\\' {
					backslashed = true
				}
			}
			return token{}, fmt.Errorf("unterminated double-quoted string started on line %d", start)

		default: // a word
			// TODO: does a comment end a word? A single slash does not.
			var word string
			word, s = scanWord(s)
			return token{kind: tokWord, val: word}, nil
		}
		panic("unreachable")
	}
}

// stop chars: any whitespace; parens; braces.
func scanWord(s string) (string, string) {
	for i, r := range s {
		if unicode.IsSpace(r) {
			return s[:i], s[i:]
		}
		switch r {
		case '(', ')', '{', '}':
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func skipSpace(s string) string {
	for i, r := range s {
		if r == '\n' || !unicode.IsSpace(r) {
			return s[i:]
		}
	}
	return ""
}
