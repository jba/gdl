// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

// TODO: test lex errors

package gdl

import (
	"slices"
	"testing"
)

func TestLexerNext(t *testing.T) {
	char := func(r rune) token { return token{kind: r} }
	word := func(s string) token { return token{kind: tokWord, val: s} }
	str := func(s string) token { return token{kind: tokString, val: s} }

	for _, tc := range []struct {
		in   string
		want []token
	}{
		{"", nil},
		{"(", []token{char('(')}},
		{"\n", []token{char('\n')}},
		{";", []token{char('\n')}},
		{"foo", []token{word("foo")}},
		{"foo bar baz", []token{word("foo"), word("bar"), word("baz")}},
		{"foo(bar)", []token{word("foo"), char('('), word("bar"), char(')')}},
		{"a\nb", []token{word("a"), char('\n'), word("b")}},
		{"a;b", []token{word("a"), char('\n'), word("b")}},
		{"a\n   \nb", []token{word("a"), char('\n'), char('\n'), word("b")}},
		{"a;   ;b", []token{word("a"), char('\n'), char('\n'), word("b")}},
		// raw strings
		{"`x`", []token{str("`x`")}},
		{"`x` is raw", []token{str("`x`"), word("is"), word("raw")}},
		{"`x y` is raw", []token{str("`x y`"), word("is"), word("raw")}},
		{"`x y` is`raw", []token{str("`x y`"), word("is`raw")}},
		// escaped strings
		{`"x"`, []token{str(`"x"`)}},
		{`"\t"`, []token{str(`"\t"`)}},
		{`"x y\t"`, []token{str(`"x y\t"`)}},
		{`"x\"y\t\""`, []token{str(`"x\"y\t\""`)}},
		// comments
		{"\n// a comment\na b // more", []token{char('\n'), char('\n'), word("a"), word("b")}},
		{"a(//comment\n)", []token{word("a"), char('('), char('\n'), char(')')}},
		{" not//a comment", []token{word("not//a"), word("comment")}},
		// continuations
		{"a b\\c", []token{word("a"), word("b\\c")}},
		{"a b\\\nc", []token{word("a"), word("b\\"), char('\n'), word("c")}},
		{"a b \\\nc", []token{word("a"), word("b"), word("c")}},
	} {
		l := newLexer(tc.in)
		var got []token
		for {
			tok := l.next()
			if tok.kind == tokErr {
				t.Fatalf("%q: lexer error: %v", tc.in, tok.err)
			}
			if tok.kind == tokEOF {
				break
			}
			got = append(got, tok)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%q:\ngot  %v\nwant %v", tc.in, got, tc.want)
		}
	}
}

func TestSkipHorizontalSpace(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"foo", "foo"},
		{" \tfoo", "foo"},
		{" \t\nfoo", "\nfoo"},
		{" \t\n foo", "\n foo"},
	} {
		got := skipHorizontalSpace(tc.in)
		if got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestScanWord(t *testing.T) {
	for _, tc := range []struct {
		in                 string
		wantWord, wantRest string
	}{
		{"foo", "foo", ""},
		{"foo bar", "foo", " bar"},
		{"/path/name x", "/path/name", " x"},
		{"f(x)", "f", "(x)"},
		{"a/%@(b)", "a/%@", "(b)"},
		{"w\ny", "w", "\ny"},
	} {
		gotWord, gotRest := scanWord(tc.in)
		if gotWord != tc.wantWord || gotRest != tc.wantRest {
			t.Errorf("%q: got (%q, %q), want (%q, %q)", tc.in, gotWord, gotRest, tc.wantWord, tc.wantRest)
		}
	}
}
