// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"strings"
	"testing"

	"github.com/jba/format"
	"rsc.io/diff"
)

// TODO: test errors

func TestParseValue(t *testing.T) {
	for _, tc := range []struct {
		in       string
		wantHead []string
		wantList []Value
	}{
		{"x", []string{"x"}, nil},
		{"x y", []string{"x", "y"}, nil},
		{
			"(\nx\ny\n)",
			nil,
			[]Value{
				{Head: []string{"x"}},
				{Head: []string{"y"}},
			},
		},
		{
			"x(\n a b\n)",
			[]string{"x"},
			[]Value{{Head: []string{"a", "b"}}},
		},
		{
			"x(a b\n)",
			[]string{"x"},
			[]Value{{Head: []string{"a", "b"}}},
		},
		{
			"x(a b )",
			[]string{"x"},
			[]Value{{Head: []string{"a", "b"}}},
		},
		{
			"x(a b)",
			[]string{"x"},
			[]Value{{Head: []string{"a", "b"}}},
		},
		{
			"x\n\n",
			[]string{"x"},
			nil,
		},
		{
			"(\n\nx\n\ny\n\n)",
			nil,
			[]Value{{Head: []string{"x"}}, {Head: []string{"y"}}},
		},
		{
			"command foo 1.5 (\nargs a b\nflag foo -n `do nothing`\n)",
			[]string{"command", "foo", "1.5"},
			[]Value{
				{Head: []string{"args", "a", "b"}},
				{Head: []string{"flag", "foo", "-n", "do nothing"}},
			},
		},
	} {
		lex := newLexer(tc.in)
		got, err := parseValue(lex.next(), lex)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		want := Value{Head: tc.wantHead, List: tc.wantList}
		gf := format.Sprint(got)
		wf := format.Sprint(want)
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
		{"(\n) x", "close paren must be followed"},
	} {
		lex := newLexer(tc.in)
		_, err := parseValue(lex.next(), lex)
		if err == nil {
			t.Fatalf("%s: no error", tc.in)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not contain %q", tc.in, err, tc.want)
		}
	}
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Value
	}{
		{"", nil},
		{"x", []Value{{Head: []string{"x"}}}},
		{"x yz", []Value{{Head: []string{"x", "yz"}}}},
		{"x 2.5 3 true", []Value{{Head: []string{"x", "2.5", "3", "true"}}}},
		{
			"// a file\nx 17 \"true\"\ny 22.5 \\\ntrue",
			[]Value{
				{Head: []string{"x", "17", "true"}},
				{Head: []string{"y", "22.5", "true"}},
			},
		},
		{
			"a (\nb c)",
			[]Value{
				{
					Head: []string{"a"},
					List: []Value{
						{Head: []string{"b", "c"}},
					},
				},
			},
		},
		{
			`a (
				b x
				c y
			)`,
			[]Value{
				{
					Head: []string{"a"},
					List: []Value{
						{Head: []string{"b", "x"}},
						{Head: []string{"c", "y"}},
					},
				},
			},
		},
		{
			`a (
				b x

				// second
				c y
			)`,
			[]Value{
				{
					Head: []string{"a"},
					List: []Value{
						{Head: []string{"b", "x"}},
						{Head: []string{"c", "y"}},
					},
				},
			},
		},
		{
			"a b (\nc\nd\n)",
			[]Value{
				{
					Head: []string{"a", "b"},
					List: []Value{
						{Head: []string{"c"}},
						{Head: []string{"d"}},
					},
				},
			},
		},
		{
			"(\na\nb\n)",
			[]Value{
				{
					List: []Value{
						{Head: []string{"a"}},
						{Head: []string{"b"}},
					},
				},
			},
		},
		{
			"(a b)",
			[]Value{
				{
					List: []Value{
						{Head: []string{"a", "b"}},
					},
				},
			},
		},
		{
			"(a b)\nc(d)",
			[]Value{
				{
					List: []Value{
						{Head: []string{"a", "b"}},
					},
				},
				{
					Head: []string{"c"},
					List: []Value{
						{Head: []string{"d"}},
					},
				},
			},
		},
	} {
		got, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if g, w := format.Sprint(got), format.Sprint(tc.want); g != w {
			t.Errorf("%q:\ngot  %s\nwant %s", tc.in, g, w)
		}
	}
}
