// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"reflect"
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
			"a (;b c)",
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
			`a (b x; c y)`,
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
			"a b (;c;d;)",
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
			"(;a;b;)",
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
		{
			"(a b);c(d)",
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

func TestUnmarshal(t *testing.T) {
	type S1 struct {
		Name   string
		Points int `gdl:"score"`
		Items  []string
		Intp   *int
	}

	for _, tc := range []struct {
		name string
		in   string
		p    any
		want any
	}{
		{"string", "x", "", "x"},
		{"int", "-23", 0, -23},
		{"float", "1.5", 0.0, 1.5},
		{"bool", "true", false, true},
		{"uint", "23", uint(0), 23},
		// TODO: support hex and octal uint constants?
		{"scalar slice head", "1 2 3", make([]int, 3), []int{1, 2, 3}},
		{"scalar array head", "1 2 3", [3]int{}, [...]int{1, 2, 3}},
		{"scalar slice list", "(1; 2; 3)", make([]int, 3), []int{1, 2, 3}},
		{"scalar array list", "(1; 2; 3)", [3]int{}, [...]int{1, 2, 3}},
		{"struct", "(name Al; score 23)", S1{}, S1{Name: "Al", Points: 23}},
		{"struct ignore field", "(name Pat; pts 18)", S1{}, S1{Name: "Pat"}},
		{
			"struct append",
			"(name Al; score 23; items x y z)",
			S1{},
			S1{Name: "Al", Points: 23, Items: []string{"x", "y", "z"}},
		},
		{
			"pointer",
			"(name Andy; intp 3)",
			func() S1 { i := 1; return S1{Name: "Fred", Intp: &i} }(),
			func() S1 { i := 3; return S1{Name: "Andy", Intp: &i} }(),
		},
		{
			"new pointer",
			"(name Andy; intp 3)",
			S1{},
			func() S1 {
				i := 3
				return S1{Name: "Andy", Intp: &i}
			}(),
		},
		{"map", "(a 1; b 2)", map[string]int{}, map[string]int{"a": 1, "b": 2}},
		{"new map", "(a 1; b 2)", map[string]int(nil), map[string]int{"a": 1, "b": 2}},
		{
			"recursive",
			`((name Al);
				(name Pat))`,
			[]S1(nil),
			[]S1{{Name: "Al"}, {Name: "Pat"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vals, err := Parse(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if len(vals) != 1 {
				t.Fatal("need exactly one value")
			}
			p := reflect.New(reflect.TypeOf(tc.p))
			p.Elem().Set(reflect.ValueOf(tc.p)) // for, e.g., preserving length of slices
			if err := UnmarshalValue(vals[0], p.Interface()); err != nil {
				t.Fatal(err)
			}
			if g, w := format.Sprint(p.Elem().Interface()), format.Sprint(tc.want); g != w {
				t.Errorf("\ngot \n%s\nwant %s", g, w)
			}
		})
	}
}
