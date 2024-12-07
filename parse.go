// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
)

// ParseFile calls [Parse] on the contents of the file.
func ParseFile(filename string) ([]Value, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return parse(string(data), filename)
}

// Parse parses the string and returns one Line per logical line.
func Parse(s string) ([]Value, error) {
	return parse(s, "<no file>")
}

func parse(s, filename string) (_ []Value, err error) {
	lex := newLexer(s, filename)

	defer func() {
		if err != nil {
			err = fmt.Errorf("%s:%d: %w", filename, lex.lineno, err)
		}
	}()

	var vals []Value

	for {
		tok := skipNewlines(lex)
		switch tok.kind {
		case tokEOF:
			return vals, nil
		case ')':
			return nil, errors.New("unexpected close paren")
		default:
			vs, err := parseValues(tok, lex)
			if err != nil {
				return nil, err
			}
			vals = append(vals, vs...)
		}
	}
}

// Called at line start. Ends at the next line start or EOF.
// Only called when there is a value.
func parseValues(tok token, lex *lexer) ([]Value, error) {
	var words []string
	for {
		switch tok.kind {
		case tokEOF:
			// Accept a value that isn't followed by a newline.
			if len(words) > 0 {
				return []Value{newValue(words, lex)}, nil
			}
			return nil, io.ErrUnexpectedEOF

		case '\n':
			if len(words) > 0 {
				return []Value{newValue(words, lex)}, nil
			}
			return nil, errors.New("unexpected newline")

		case tokWord:
			words = append(words, tok.val)

		case tokString:
			unq, err := strconv.Unquote(tok.val)
			if err != nil {
				return nil, err
			}
			words = append(words, unq)

		case '(':
			list, err := parseList(lex, ')')
			if err != nil {
				return nil, err
			}
			var vals []Value
			for _, lv := range list {
				vals = append(vals, newValue(slices.Concat(words, lv.Words), lex))
			}
			return vals, nil

		case ')', '}', ']':
			if len(words) == 0 {
				panic("bad close delimiter")
			}
			// We're here after getting b in something like
			//    (a; b)
			// The close delim is part of the enclosing list.
			lex.unget(tok)
			return []Value{newValue(words, lex)}, nil

		// case '{':
		// 	list, err := parseList(lex, '}')
		// 	if err != nil {
		// 		return nil, err
		// 	}
		// 	return []Value{newValue(head, list, lex)}, nil

		case tokErr:
			return nil, tok.err

		default:
			panic("bad token kind")
		}
		tok = lex.next()
	}
}

// Called just after open delimiter. Ends just after close delimiter.
// Works for parens, braces and square brackets.
func parseList(lex *lexer, close rune) ([]Value, error) {
	var vs []Value
	for {
		tok := skipNewlines(lex)
		switch tok.kind {
		case tokEOF:
			return nil, io.ErrUnexpectedEOF
		case close:
			k := lex.peek()
			switch k {
			case tokErr:
				return nil, lex.next().err
			case close, '\n', tokEOF:
				return vs, nil
			default:
				return nil, errors.New("close delimiter must be followed by newline, EOF or another close delimiter")
			}
		case ')', '}', ']':
			return nil, errors.New("mismatched close delimiter")
		}
		vs1, err := parseValues(tok, lex)
		if err != nil {
			return nil, err
		}
		vs = append(vs, vs1...)
	}
}

func skipNewlines(lex *lexer) token {
	for {
		tok := lex.next()
		if tok.kind != '\n' {
			return tok
		}
	}
}

func newValue(words []string, lex *lexer) Value {
	return Value{
		Words: words,
		File:  lex.filename,
		Line:  lex.lineno,
	}
}
