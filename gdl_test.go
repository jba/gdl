// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jba/format"
	"rsc.io/diff"
)

// TODO: test errors

var vfmt = format.New().IgnoreFields(Value{}, "File", "Line")

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
		lex := newLexer(tc.in, "tc")
		got, err := parseValue(lex.next(), lex)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		want := Value{Head: tc.wantHead, List: tc.wantList}
		gf := vfmt.Sprint(got)
		wf := vfmt.Sprint(want)
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
		lex := newLexer(tc.in, "tc")
		_, err := parseValue(lex.next(), lex)
		matchError(t, tc.in, err, tc.want)
	}
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    Value
		wantErr string
	}{
		{in: "1", want: Value{Head: []string{"1"}}},
		{in: "a(b c)", want: Value{Head: []string{"a"}, List: []Value{{Head: []string{"b", "c"}}}}},
		{in: "()", want: Value{}},
		{in: "", wantErr: "no value"},
		{in: "1;2", wantErr: "more than one value"},
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

func TestValues(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Value
	}{
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
		iter, errf := Values(tc.in)
		got := slices.Collect(iter)
		if err := errf(); err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if g, w := vfmt.Sprint(got), vfmt.Sprint(tc.want); g != w {
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
			val, err := Parse(tc.in)
			if err != nil {
				t.Fatal(err)
			}
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
		{"a (b)", []string(nil), "both a head and a list"},
		{"a (b)", map[string]string{}, "map*needs empty head"},
		{"x", make(chan int), "cannot unmarshal into"},
		{"x y", 0, "scalar*one head"},
	} {
		val, err := Parse(tc.in)
		if err != nil {
			t.Fatal(err)
		}
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
