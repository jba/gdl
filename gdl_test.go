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
			"\n\nx\n\n",
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
		got, err := parseValue(lex)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
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
		{"()", "open paren must be followed"},
		{"(\n) x", "close paren must be followed"},
		{"x y )", "unexpected close paren"},
	} {
		lex := newLexer(tc.in)
		_, err := parseValue(lex)
		if err == nil {
			t.Fatalf("%s: no error", tc.in)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not contain %q", tc.in, err, tc.want)
		}
	}
}

// func TestParse(t *testing.T) {
// 	for _, tc := range []struct {
// 		in   string
// 		want []Value
// 	}{
// 		{"", nil},
// 		{"x", []Value{{"x"}}},
// 		{"x y z", [][]any{{"x", "y", "z"}}},
// 		{"x 2.5 3 true", [][]any{{"x", 2.5, int64(3), true}}},
// 		{
// 			"// a file\nx 17 \"true\"\ny 22.5 \\\ntrue",
// 			[][]any{
// 				{"x", int64(17), "true"},
// 				{"y", 22.5, true},
// 			},
// 		},
// 		{
// 			"a (\nb c)",
// 			[][]any{
// 				{"a", "b", "c"},
// 			},
// 		},
// 		{
// 			`a (
// 				b x
// 				c y
// 			)`,
// 			[][]any{
// 				{"a", "b", "x"},
// 				{"a", "c", "y"},
// 			},
// 		},
// 		{
// 			`a (
// 				b x

// 				// second
// 				c y
// 			)`,
// 			[][]any{
// 				{"a", "b", "x"},
// 				{"a", "c", "y"},
// 			},
// 		},
// 		{
// 			"a b (\nc\nd\n)",
// 			[][]any{
// 				{"a", "b", "c"},
// 				{"a", "b", "d"},
// 			},
// 		},
// 		{
// 			"(\na\nb\n)",
// 			[][]any{
// 				{"a"},
// 				{"b"},
// 			},
// 		},
// 	} {
// 		got, err := Parse(tc.in)
// 		if err != nil {
// 			t.Fatalf("%q: %v", tc.in, err)
// 		}
// 		if !cmp.Equal(got, tc.want) {
// 			t.Errorf("%q:\ngot  %v\nwant %v", tc.in, got, tc.want)
// 		}
// 	}
// }
