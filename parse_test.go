// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"path"
	"testing"

	"github.com/jba/format"
	"rsc.io/diff"
)

var vfmt = format.New().IgnoreFields(Value{}, "File", "Line")

func TestParseValues(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Value
	}{
		{"x", []Value{{Words: []string{"x"}}}},
		{"x y", []Value{{Words: []string{"x", "y"}}}},
		{
			"(\nx\ny\n)",
			[]Value{
				{Words: []string{"x"}},
				{Words: []string{"y"}},
			},
		},
		{
			"(\n\nx\n\ny\n\n)",
			[]Value{
				{Words: []string{"x"}},
				{Words: []string{"y"}},
			},
		},
		{
			"x(\n a b\n)",
			[]Value{{Words: []string{"x", "a", "b"}}},
		},
		{
			"x(a b\n)",
			[]Value{{Words: []string{"x", "a", "b"}}},
		},
		{
			"x(a b )",
			[]Value{{Words: []string{"x", "a", "b"}}},
		},
		{
			"x(a b)",
			[]Value{{Words: []string{"x", "a", "b"}}},
		},
		{
			"x\n\n",
			[]Value{{Words: []string{"x"}}},
		},
		{
			"h1 h2 (args a b; f(c; d))",
			[]Value{
				{Words: []string{"h1", "h2", "args", "a", "b"}},
				{Words: []string{"h1", "h2", "f", "c"}},
				{Words: []string{"h1", "h2", "f", "d"}},
			},
		},
	} {
		lex := newLexer(tc.in, "tc")
		got, err := parseValues(lex.next(), lex)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		gf := vfmt.Sprint(got)
		wf := vfmt.Sprint(tc.want)
		if gf != wf {
			t.Errorf("%s: mismatch (-want, +got):\n%s", tc.in, diff.Format(gf, wf))
		}
	}
}

func TestParseValueError(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"", "EOF"},
		{"(\n) x", "close * must be followed"},
		{"(\n} x", "mismatch"},
	} {
		lex := newLexer(tc.in, "tc")
		_, err := parseValues(lex.next(), lex)
		matchError(t, tc.in, err, tc.want)
	}
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    []Value
		wantErr string
	}{
		{in: "1", want: []Value{{Words: []string{"1"}}}},
		{in: "a(b c)", want: []Value{{Words: []string{"a", "b", "c"}}}},
		{in: "()", want: nil},
		{in: "", want: nil},
		{in: "1;2", want: []Value{{Words: []string{"1"}}, {Words: []string{"2"}}}},
		{
			in: "a (;b c)",
			want: []Value{
				{Words: []string{"a", "b", "c"}},
			},
		},
		{
			in: `a (
				b x
				c y
			)`,
			want: []Value{
				{Words: []string{"a", "b", "x"}},
				{Words: []string{"a", "c", "y"}},
			},
		},
		{
			in: `a (b x; c y)`,
			want: []Value{
				{Words: []string{"a", "b", "x"}},
				{Words: []string{"a", "c", "y"}},
			},
		},
		{
			in: "a b (\nc\nd\n)",
			want: []Value{
				{Words: []string{"a", "b", "c"}},
				{Words: []string{"a", "b", "d"}},
			},
		},
		{
			in: "(\na\nb\n)",
			want: []Value{
				{Words: []string{"a"}},
				{Words: []string{"b"}},
			},
		},
		{
			in:   "(a b)",
			want: []Value{{Words: []string{"a", "b"}}},
		},
		{
			in: "(a b)\nc(d)",
			want: []Value{
				{Words: []string{"a", "b"}},
				{Words: []string{"c", "d"}},
			},
		},
	} {
		got, err := Parse(tc.in)
		if tc.wantErr != "" {
			matchError(t, tc.in, err, tc.wantErr)
		} else if err != nil {
			t.Errorf("%q: unexpected error %q", tc.in, err)
		} else if g, w := vfmt.Sprint(got), vfmt.Sprint(tc.want); g != w {
			t.Errorf("%q:\ngot  %s\nwant %s", tc.in, g, w)
		}
	}
}

func matchError(t *testing.T, prefix string, err error, glob string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: got nil, want error", prefix)
		return
	}
	m, err1 := path.Match("*"+glob+"*", err.Error())
	if err1 != nil {
		t.Fatalf("bad glob: %q: %v", glob, err1)
	}
	if !m {
		t.Errorf("%s:\ngot error %q\nwant it to match %q", prefix, err, glob)
	}
}
