// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

// gdl implements a decription language based on the file format of go.mod
// and similar files.
// The format is line-oriented.
// If the last non-whitespace character on a line is a backslash, the line continues
// onto the next line.
// Comments begin with // and extend to the end of the line (backslashes are ignored).
//
// Each line is a sequence of words separated by whitespace.
// A word can include whitespace by enclosing it in double quotes or backticks.
// Both kinds of quotations are interpreted according to Go syntax.
// A word that looks like an integer is converted to an int64.
// A word that looks like a float-point number is converted to a float64.
// The words true and false are converted to bools.
// No other processing is done to a word.

// # Groups
//
// TODO
package gdl

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// ParseFile calls [Parse] on the contents of the file.
func ParseFile(filename string) ([][]any, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return parse(string(data), filename)
}

// Parse parses the string and returns one []any per logical line.
// Each element of a line slice is either a string, int64, float64 or bool.
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

	var parseRec func(rune, []any) error
	parseRec = func(end rune, prefix []any) error {
		curline = prefix
		for {
			tok, err := lex.next()
			if err != nil {
				return err
			}

			switch tok.kind {
			case end:
				if len(curline) > len(prefix) {
					lines = append(lines, curline)
				}
				// A close delimiter must be followed by a newline.
				// We don't support things like "a (\nb\n) c".
				if tok.kind != tokEOF {
					if k := lex.peek(); k != '\n' && k != tokEOF {
						return errors.New("close delimiter must be followed by newline or EOF")
					}
				}
				return nil

			case tokEOF:
				// We expected a close delimiter.
				// TODO: give line of open delim
				return errors.New("missing close delimiter")

			case '\n':
				// Ignore blank lines, even if there is a prefix.
				if len(curline) > len(prefix) {
					lines = append(lines, curline)
					curline = prefix
				}

			case tokWord:
				curline = append(curline, parseWord(tok.val))

			case tokString:
				unq, err := strconv.Unquote(tok.val)
				if err != nil {
					return err
				}
				curline = append(curline, unq)

			case '(':
				// Delimiter must end a line.
				if k := lex.peek(); k != '\n' && k != tokEOF {
					return errors.New("delimiter not at end of line")
				}
				// Make the prefix safe for multiple appends.
				prefix = curline[:len(curline):len(curline)]
				return parseRec(')', prefix)

				// TODO: enforce brace prefix uniqueness.
			case '{':
				// Make the prefix safe for multiple appends.
				prefix = curline[:len(curline):len(curline)]
				return parseRec(')', prefix)

			case ')', '}':
				return errors.New("unexpected close delimiter")

			default:
				panic("bad token kind")
			}
		}
	}

	if err := parseRec(tokEOF, nil); err != nil {
		return nil, err
	}
	return lines, nil
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
