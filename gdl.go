// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

// TODO: rewrite pkg doc.

// TODO: support unmarshaling into any.

// TODO: support a struct tag like "2-" to mean from arg 2 to the end.

// type Arg struct {
// 	Name, Type string
// }

// matches "arg size int"

// type Command struct {
// 	Name string `gdl:",id"`
// 	Args []Arg
// }

// matches

// 	command create arg size int
// 	command create arg place string

//

// gdl implements a decription language inspired by the file format of go.mod
// and similar files.
//
// # Lexical structure
//
// A gdl string is a sequence of words, separators and delimiters.
// The delimiters are parentheses.
// The separators are newline and semicolon.
// If the last non-whitespace character on a line is a backslash, the line continues
// onto the next line.
//
// A word is a sequence of non-space characters ending in a delimiter or separator.
// Words can be double-quoted or backquoted as in Go, with the same syntax.
// Comments begin with "//" and extend to the end of the line.
// Backslashes are ignored inside a comment.
//
// Parentheses mean repetition, as in Go.
// For example,
//
//	  require (
//	      example.com/a v1.2.3
//	      example.com/b v0.2.5
//	  )
//
//	is equivalent to the two values
//
//	  require example.com/a v1.2.3
//	  require example.com/b v0.2.5
//
// TODO: ID Fields.
package gdl

import (
	"fmt"
)

type Value struct {
	Words []string
	File  string
	Line  int
}

func (l Value) Pos() string {
	if l.File == "" && l.Line == 0 {
		return "?"
	}
	if l.Line == 0 {
		return l.File
	}
	if l.File == "" {
		return fmt.Sprintf("?:%d", l.Line)
	}
	return fmt.Sprintf("%s:%d", l.File, l.Line)
}
