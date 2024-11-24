// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

func ParseFile(filename string) ([][]any, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return parse(string(data), filename)
}

func Parse(s string) ([][]any, error) {
	return parse(s, "<no file>")
}

func parse(s, filename string) (_ [][]any, err error) {
	lex := newLexer(s)
	var lines [][]any
	var curline []any

	defer func() {
		if err != nil {
			err = fmt.Errorf("%s:%d: %w", filename, lex.lineno, err)
		}
	}()

	for {
		tok, err := lex.next()
		if err != nil {
			return nil, err
		}
		switch tok.kind {
		case tokEOF:
			if len(curline) > 0 {
				lines = append(lines, curline)
			}
			return lines, nil
		case '\n':
			if len(curline) > 0 {
				lines = append(lines, curline)
				curline = nil
			}
		case tokWord:
			curline = append(curline, parseWord(tok.val))
		case tokString:
			unq, err := strconv.Unquote(tok.val)
			if err != nil {
				return nil, err
			}
			curline = append(curline, unq)

		case '(', ')', '{', '}':
			return nil, errors.New("unimp")

		default:
			panic("bad token kind")
		}
	}
}

func parseWord(s string) any {
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}
