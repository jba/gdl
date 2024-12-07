// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

// import (
// 	"github.com/jba/format"
// )

// TODO: test more errors
// TODO: ~100% test coverage
// TODO: simplify tests by testing that whitespace is ignored after an open delimiter.
//       (Maybe redo the lexer so it really is ignored.)

// var vfmt = format.New().IgnoreFields(Value{}, "File", "Line")

// TODO: test errors
// func TestUnmarshalValue(t *testing.T) {
// 	type S1 struct {
// 		Name   string
// 		Points int `gdl:"score"`
// 		Items  []string
// 		Intp   *int
// 	}

// 	type enum struct {
// 		Name   string   `gdl:"1"`
// 		Values []string `gdl:"*"`
// 	}

// 	for _, tc := range []struct {
// 		name string
// 		in   string
// 		p    any
// 		want any
// 	}{
// 		{"string", "x", "", "x"},
// 		{"int", "-23", 0, -23},
// 		{"float", "1.5", 0.0, 1.5},
// 		{"bool", "true", false, true},
// 		{"uint", "23", uint(0), 23},
// 		// TODO: support hex and octal uint constants?
// 		{"scalar slice head", "1 2 3", make([]int, 3), []int{1, 2, 3}},
// 		{"scalar array head", "1 2 3", [3]int{}, [...]int{1, 2, 3}},
// 		{"scalar slice list", "{1; 2; 3}", make([]int, 3), []int{1, 2, 3}},
// 		{"scalar array list", "{1; 2; 3}", [3]int{}, [...]int{1, 2, 3}},
// 		{"struct", "{name Al; score 23}", S1{}, S1{Name: "Al", Points: 23}},
// 		{"struct ignore field", "{name Pat; pts 18}", S1{}, S1{Name: "Pat"}},
// 		{
// 			"struct append",
// 			"{name Al; score 23; items x y z}",
// 			S1{},
// 			S1{Name: "Al", Points: 23, Items: []string{"x", "y", "z"}},
// 		},
// 		{
// 			"pointer",
// 			"{name Andy; intp 3}",
// 			func() S1 { i := 1; return S1{Name: "Fred", Intp: &i} }(),
// 			func() S1 { i := 3; return S1{Name: "Andy", Intp: &i} }(),
// 		},
// 		{
// 			"new pointer",
// 			"{name Andy; intp 3}",
// 			S1{},
// 			func() S1 {
// 				i := 3
// 				return S1{Name: "Andy", Intp: &i}
// 			}(),
// 		},
// 		{"map", "{a 1; b 2}", map[string]int{}, map[string]int{"a": 1, "b": 2}},
// 		{"new map", "{a 1; b 2}", map[string]int(nil), map[string]int{"a": 1, "b": 2}},
// 		{
// 			"recursive",
// 			`{{name Al};
// 				{name Pat}}`,
// 			[]S1(nil),
// 			[]S1{{Name: "Al"}, {Name: "Pat"}},
// 		},
// 		{
// 			"star",
// 			"enum E {a;b;c}",
// 			enum{},
// 			enum{Name: "E", Values: []string{"a", "b", "c"}},
// 		},
// 	} {
// 		t.Run(tc.name, func(t *testing.T) {
// 			vals, err := Parse(tc.in)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			if len(vals) != 1 {
// 				t.Fatal("need one value")
// 			}
// 			val := vals[0]
// 			p := reflect.New(reflect.TypeOf(tc.p))
// 			p.Elem().Set(reflect.ValueOf(tc.p)) // for, e.g., preserving length of slices
// 			if err := UnmarshalValue(val, p.Interface()); err != nil {
// 				t.Fatal(err)
// 			}
// 			if g, w := format.Sprint(p.Elem().Interface()), format.Sprint(tc.want); g != w {
// 				t.Errorf("\ngot \n%s\nwant %s", g, w)
// 			}
// 		})
// 	}
// }
// func TestUnmarshalValueError(t *testing.T) {
// 	for _, tc := range []struct {
// 		in   string
// 		p    any
// 		want string
// 	}{
// 		{"a {b}", []string(nil), "both a head and a list"},
// 		{"a {b}", map[string]string{}, "map*needs empty head"},
// 		{"x", make(chan int), "cannot unmarshal into"},
// 		{"x y", 0, "scalar*one head"},
// 	} {
// 		vals, err := Parse(tc.in)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		if len(vals) != 1 {
// 			t.Fatal("need one value")
// 		}
// 		val := vals[0]
// 		p := reflect.New(reflect.TypeOf(tc.p))
// 		p.Elem().Set(reflect.ValueOf(tc.p)) // for, e.g., preserving length of slices
// 		matchError(t, tc.in, UnmarshalValue(val, p.Interface()), tc.want)
// 	}
// }

// func TestUnmarshalValues(t *testing.T) {
// 	type Name struct {
// 		Name string `gdl:"1"`
// 	}
// 	type Require struct {
// 		Mod     string `gdl:"1"`
// 		Version string `gdl:"2"`
// 	}
// 	type Replace struct {
// 		From string `gdl:"1"`
// 		Op   string `gdl:"2"`
// 		To   string `gdl:"3"`
// 	}
// 	type S struct {
// 		Name     Name
// 		Requires []Require
// 		Replaces []*Replace
// 	}

// 	for _, tc := range []struct {
// 		in      string
// 		want    S
// 		wantErr string
// 	}{
// 		{
// 			in: "name test; require m1 v1; require m2 v2; replace a -> b",
// 			want: S{
// 				Name:     Name{"test"},
// 				Requires: []Require{{Mod: "m1", Version: "v1"}, {Mod: "m2", Version: "v2"}},
// 				Replaces: []*Replace{{From: "a", Op: "->", To: "b"}},
// 			},
// 		},
// 		{
// 			in: "name test; require (m1 v1; m2 v2); replace a -> b",
// 			want: S{
// 				Name:     Name{"test"},
// 				Requires: []Require{{Mod: "m1", Version: "v1"}, {Mod: "m2", Version: "v2"}},
// 				Replaces: []*Replace{{From: "a", Op: "->", To: "b"}},
// 			},
// 		},
// 		{
// 			in:      "name a; name b",
// 			wantErr: "occurs more than once",
// 		},
// 	} {
// 		var got S
// 		vals, errf := Values(tc.in)
// 		if err := UnmarshalValues(vals, &got); err != nil {
// 			if tc.wantErr == "" {
// 				t.Fatalf("%q: %v", tc.in, err)
// 			}
// 			if !strings.Contains(err.Error(), tc.wantErr) {
// 				t.Fatalf("%q: got error %q, want it to contain %q", tc.in, err, tc.wantErr)
// 			}
// 			continue
// 		}
// 		if err := errf(); err != nil {
// 			t.Fatal(err)
// 		}
// 		if g, w := format.Sprint(got), format.Sprint(tc.want); g != w {
// 			t.Errorf("%q:\ngot  %s\nwant %s", tc.in, g, w)
// 		}
// 	}
// }
