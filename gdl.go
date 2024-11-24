// Copyright 2024 by Jonathan Amsterdam. 
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"strings"
)

func Parse(s string) ([][]any, error) {
	return ParseReader(strings.NewReader(s))
}

func ParseFile(filename string) ([][]any, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := ParseReader(f)
	if err != nil {
		return nil, fmt.Errorf("%s:%v", filename, err)
	}
	return r, nil
}

func ParseReader(r io.Reader) (_ [][]any, err error) {
	var (
		lines   [][]any
		curLine []any
		line    string
		lineno  int
	)

	defer func() {
		if err != nil {
			err = fmt.Errorf("%d: %w", lineno, err)
		}
	}()

	iter, errf := trimmedLines(r)

	for line, lineno = range iter {
		words, end, err := lexWords(line)
		if err != nil {
			return err
		}
		curLine = append(curLine, words...)
		if end {
			lines = append(lines, curLine)
			curLine = nil
		}
	}
	if err := errf(); err != nil {
		return nil, err
	}
	// If curLine is non-empty, the last line ended with a continuation character.
	if len(curLine) > 0 {
		return nil, errors.New("file ends with continuation character")
	}
	return lines, nil
}

// ignore blanks and comment lines, strip leading and trailing whitespace
// second value is true line number
func trimmedLines(r io.Reader) (iter.Seq2[string, int], func() error) {
	var err error
	errSet := false

	errf := func() error {
		if !errSet {
			panic("error not set")
		}
		return err
	}

	iter := func(yield func(string, int) bool) {
		scan := bufio.NewScanner(r)
		lineno := 0
		for scan.Scan() {
			lineno++
			line := strings.TrimSpace(scan.Text())
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			if !yield(line, lineno) {
				break
			}
		}
		err = scan.Err()
		errSet = true
	}

	return iter, errf
}

func lexWords(s string) (words []any, end bool, err error) {
	// s was trimmed, so it never ends in a space.
	var words []any
	for s != "" {
		var word any
		word, s, err = lexWord(s)
		if err != nil {
			return nil, false, err
		}
		end = (word != "\\")
		words = append(words, word)
	}
	if end {
		words = words[:len(words)-1] // remove backslash
	}
	return words, end, nil
}

func lexWord(s string) (any, string, error) {
	if len(strings.TrimSpace(s)) == 0 {
		panic("lexWord called on blank string")
	}
	var i int
	var r rune
	for i, r = range s {
		if unicode.IsSpace(r) {
			continue
		}
	}
	s = s[i:]
	switch r {
	case '"':
		return lexGoString(s)
	case '`':
		// TODO: we have to indicate to callers that we're in the middle of a backquoted string
	}
}

:o 
