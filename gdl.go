// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

// TODO: allow open paren to be followed by a value (but not close, that would be confusing)

// gdl implements a decription language based on the file format of go.mod
// and similar files.
// The format is line-oriented.
// If the last non-whitespace character on a line is a backslash, the line continues
// onto the next line.
// Comments begin with // and extend to the end of the line.
// Backslashes are ignored inside a comment.
//
// Each line is a sequence of words separated by whitespace.
// A word can include whitespace by enclosing it in double quotes or backticks.
// Both kinds of quotations are interpreted according to Go syntax.

// An open parenthesis at the end of a line starts a sequence, which ends on a line
// consisting only of a close parenthesis.

// A word that looks like an integer is converted to an int64.
// A word that looks like a float-point number is converted to a float64.
// The words true and false are converted to bools.
// No other processing is done to a word.
package gdl

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"slices"
	"strconv"
)

type Value struct {
	Head []string
	List []Value
}

// ParseFile calls [Parse] on the contents of the file.
// The file is an implicit list of values.
func ParseFile(filename string) ([]Value, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return parse(string(data), filename)
}

// Parse parses the string and returns a slice of [Value]s.
// Each element of a line slice is either a string, int64, float64 or bool.
func Parse(s string) ([]Value, error) {
	return parse(s, "<no file>")
}

// parse parses s. The filename is only for display in errors.
func parse(s, filename string) (_ []Value, err error) {
	lex := newLexer(s)
	iter, errf := values(lex)
	vals := slices.Collect(iter)
	if err := errf(); err != nil {
		return nil, fmt.Errorf("%s:%d: %w", filename, lex.lineno, err)
	}
	return vals, nil
}

func values(lex *lexer) (iter.Seq[Value], func() error) {
	var err error
	iter := func(yield func(Value) bool) {
		var v Value
		v, err = parseValue(lex)
		if err == io.EOF {
			err = nil
			return
		}
		if err != nil {
			return
		}
		if !yield(v) {
			return
		}
	}
	return iter, func() error { return err }
}

// Called at line start. Ends at the next line start.
func parseValue(lex *lexer) (Value, error) {
	var head []string
	for {
		tok := lex.next()
		switch tok.kind {
		case tokEOF:
			// Accept a value that isn't followed by a newline.
			if len(head) > 0 {
				return Value{Head: head}, nil
			}
			return Value{}, io.EOF

		case '\n':
			if len(head) > 0 {
				return Value{Head: head}, nil
			}
			// blank line
			continue

		case tokWord:
			head = append(head, tok.val)

		case tokString:
			unq, err := strconv.Unquote(tok.val)
			if err != nil {
				return Value{}, err
			}
			head = append(head, unq)

		case '(':
			list, err := parseParenList(lex)
			if err != nil {
				return Value{}, err
			}
			return Value{Head: head, List: list}, nil

		case ')':
			return Value{}, errors.New("unexpected close paren")

		case tokErr:
			return Value{}, tok.err

		default:
			panic("bad token kind")
		}
	}
}

// Called just after '('. Ends at start of line.
func parseParenList(lex *lexer) ([]Value, error) {
	// Next token must be newline.
	tok := lex.next()
	if tok.kind != '\n' {
		return nil, cmp.Or(tok.err, errors.New("open paren must be followed by newline"))
	}

	var vs []Value
	for lex.peek() != ')' {
		v, err := parseValue(lex)
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	// Consume close paren.
	lex.next()
	// Expect newline or EOF.
	tok = lex.next()
	if tok.kind != '\n' && tok.kind != tokEOF {
		return nil, cmp.Or(tok.err, errors.New("close paren must be followed by newline or EOF"))
	}
	return vs, nil
}

// func parseWord(s string) any {
// 	if s == "true" {
// 		return true
// 	}
// 	if s == "false" {
// 		return false
// 	}
// 	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
// 		return i
// 	}
// 	if f, err := strconv.ParseFloat(s, 64); err == nil {
// 		return f
// 	}
// 	return s
// }
