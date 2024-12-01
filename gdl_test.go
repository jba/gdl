// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/jba/format"
	"rsc.io/diff"
)

// TODO: test more errors
// TODO: ~100% test coverage
// TODO: simplify tests by testing that whitespace is ignored after an open delimiter.
//       (Maybe redo the lexer so it really is ignored.)

var vfmt = format.New().IgnoreFields(Value{}, "File", "Line")

func TestParseValues(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Value
	}{
		{"x", []Value{{Head: []string{"x"}}}},
		{"x y", []Value{{Head: []string{"x", "y"}}}},
		{
			"(\nx\ny\n)",
			[]Value{
				{Head: []string{"x"}},
				{Head: []string{"y"}},
			},
		},
		{
			"(\n\nx\n\ny\n\n)",
			[]Value{
				{Head: []string{"x"}},
				{Head: []string{"y"}},
			},
		},
		{
			"x(\n a b\n)",
			[]Value{{Head: []string{"x", "a", "b"}}},
		},
		{
			"x(a b\n)",
			[]Value{{Head: []string{"x", "a", "b"}}},
		},
		{
			"x(a b )",
			[]Value{{Head: []string{"x", "a", "b"}}},
		},
		{
			"x(a b)",
			[]Value{{Head: []string{"x", "a", "b"}}},
		},
		{
			"x\n\n",
			[]Value{{Head: []string{"x"}}},
		},
		{
			"h1 h2 (args a b; f(c; d))",
			[]Value{
				{Head: []string{"h1", "h2", "args", "a", "b"}},
				{Head: []string{"h1", "h2", "f", "c"}},
				{Head: []string{"h1", "h2", "f", "d"}},
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
		{in: "1", want: []Value{{Head: []string{"1"}}}},
		{in: "a(b c)", want: []Value{{Head: []string{"a", "b", "c"}}}},
		{in: "()", want: nil},
		{in: "", want: nil},
		{in: "1;2", want: []Value{{Head: []string{"1"}}, {Head: []string{"2"}}}},
		{
			in: "a (;b c)",
			want: []Value{
				{Head: []string{"a", "b", "c"}},
			},
		},
		{
			in: `a {
				b x
				c y
			}`,
			want: []Value{
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
			in: `a (
				b x
				c y
			)`,
			want: []Value{
				{Head: []string{"a", "b", "x"}},
				{Head: []string{"a", "c", "y"}},
			},
		},
		{
			in: `a {b x; c y}`,
			want: []Value{
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
			in: `a (b x; c y)`,
			want: []Value{
				{Head: []string{"a", "b", "x"}},
				{Head: []string{"a", "c", "y"}},
			},
		},
		{
			in: `a {
				b x

				// second
				c y
			}`,
			want: []Value{
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
			in: "a b {\nc\nd\n}",
			want: []Value{
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
			in: "a b (\nc\nd\n)",
			want: []Value{
				{Head: []string{"a", "b", "c"}},
				{Head: []string{"a", "b", "d"}},
			},
		},
		{
			in: "a b {;c;d;}",
			want: []Value{
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
			in: "{\na\nb\n}",
			want: []Value{
				{
					List: []Value{
						{Head: []string{"a"}},
						{Head: []string{"b"}},
					},
				},
			},
		},
		{
			in: "(\na\nb\n)",
			want: []Value{
				{Head: []string{"a"}},
				{Head: []string{"b"}},
			},
		},
		{
			in: "{a b}",
			want: []Value{
				{
					List: []Value{
						{Head: []string{"a", "b"}},
					},
				},
			},
		},
		{
			in:   "(a b)",
			want: []Value{{Head: []string{"a", "b"}}},
		},
		{
			in: "{a b}\nc{d}",
			want: []Value{
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
			in: "(a b)\nc(d)",
			want: []Value{
				{Head: []string{"a", "b"}},
				{Head: []string{"c", "d"}},
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

// TODO: test errors
func TestUnmarshalValue(t *testing.T) {
	type S1 struct {
		Name   string
		Points int `gdl:"score"`
		Items  []string
		Intp   *int
	}

	type enum struct {
		Name   string   `gdl:"1"`
		Values []string `gdl:"*"`
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
		{"scalar slice list", "{1; 2; 3}", make([]int, 3), []int{1, 2, 3}},
		{"scalar array list", "{1; 2; 3}", [3]int{}, [...]int{1, 2, 3}},
		{"struct", "{name Al; score 23}", S1{}, S1{Name: "Al", Points: 23}},
		{"struct ignore field", "{name Pat; pts 18}", S1{}, S1{Name: "Pat"}},
		{
			"struct append",
			"{name Al; score 23; items x y z}",
			S1{},
			S1{Name: "Al", Points: 23, Items: []string{"x", "y", "z"}},
		},
		{
			"pointer",
			"{name Andy; intp 3}",
			func() S1 { i := 1; return S1{Name: "Fred", Intp: &i} }(),
			func() S1 { i := 3; return S1{Name: "Andy", Intp: &i} }(),
		},
		{
			"new pointer",
			"{name Andy; intp 3}",
			S1{},
			func() S1 {
				i := 3
				return S1{Name: "Andy", Intp: &i}
			}(),
		},
		{"map", "{a 1; b 2}", map[string]int{}, map[string]int{"a": 1, "b": 2}},
		{"new map", "{a 1; b 2}", map[string]int(nil), map[string]int{"a": 1, "b": 2}},
		{
			"recursive",
			`{{name Al};
				{name Pat}}`,
			[]S1(nil),
			[]S1{{Name: "Al"}, {Name: "Pat"}},
		},
		{
			"star",
			"enum E {a;b;c}",
			enum{},
			enum{Name: "E", Values: []string{"a", "b", "c"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vals, err := Parse(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if len(vals) != 1 {
				t.Fatal("need one value")
			}
			val := vals[0]
			p := reflect.New(reflect.TypeOf(tc.p))
			p.Elem().Set(reflect.ValueOf(tc.p)) // for, e.g., preserving length of slices
			if err := UnmarshalValue(val, p.Interface()); err != nil {
				t.Fatal(err)
			}
			if g, w := format.Sprint(p.Elem().Interface()), format.Sprint(tc.want); g != w {
				t.Errorf("\ngot \n%s\nwant %s", g, w)
			}
		})
	}
}
func TestUnmarshalValueError(t *testing.T) {
	for _, tc := range []struct {
		in   string
		p    any
		want string
	}{
		{"a {b}", []string(nil), "both a head and a list"},
		{"a {b}", map[string]string{}, "map*needs empty head"},
		{"x", make(chan int), "cannot unmarshal into"},
		{"x y", 0, "scalar*one head"},
	} {
		vals, err := Parse(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 1 {
			t.Fatal("need one value")
		}
		val := vals[0]
		p := reflect.New(reflect.TypeOf(tc.p))
		p.Elem().Set(reflect.ValueOf(tc.p)) // for, e.g., preserving length of slices
		matchError(t, tc.in, UnmarshalValue(val, p.Interface()), tc.want)
	}
}

func TestUnmarshalValues(t *testing.T) {
	type Name struct {
		Name string `gdl:"1"`
	}
	type Require struct {
		Mod     string `gdl:"1"`
		Version string `gdl:"2"`
	}
	type Replace struct {
		From string `gdl:"1"`
		Op   string `gdl:"2"`
		To   string `gdl:"3"`
	}
	type S struct {
		Name     Name
		Requires []Require
		Replaces []*Replace
	}

	for _, tc := range []struct {
		in      string
		want    S
		wantErr string
	}{
		{
			in: "name test; require m1 v1; require m2 v2; replace a -> b",
			want: S{
				Name:     Name{"test"},
				Requires: []Require{{Mod: "m1", Version: "v1"}, {Mod: "m2", Version: "v2"}},
				Replaces: []*Replace{{From: "a", Op: "->", To: "b"}},
			},
		},
		{
			in: "name test; require (m1 v1; m2 v2); replace a -> b",
			want: S{
				Name:     Name{"test"},
				Requires: []Require{{Mod: "m1", Version: "v1"}, {Mod: "m2", Version: "v2"}},
				Replaces: []*Replace{{From: "a", Op: "->", To: "b"}},
			},
		},
		{
			in:      "name a; name b",
			wantErr: "occurs more than once",
		},
	} {
		var got S
		vals, errf := Values(tc.in)
		if err := UnmarshalValues(vals, &got); err != nil {
			if tc.wantErr == "" {
				t.Fatalf("%q: %v", tc.in, err)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("%q: got error %q, want it to contain %q", tc.in, err, tc.wantErr)
			}
			continue
		}
		if err := errf(); err != nil {
			t.Fatal(err)
		}
		if g, w := format.Sprint(got), format.Sprint(tc.want); g != w {
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
