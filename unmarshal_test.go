// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"strings"
	"testing"
)

type Require struct {
	Module, Version string
}

type namedReqs struct {
	Name string
	Reqs []Require
}

func TestUnmarshalValue(t *testing.T) {
	type thing struct {
		Count int
		Good  bool
	}

	type enum struct {
		Name   string
		Values []string
	}

	for _, tc := range []struct {
		in   string
		p    any
		want any
	}{
		{"mod ver", &Require{}, &Require{"mod", "ver"}},
		{"17 true", &thing{}, &thing{17, true}},
		{"color red blue", &enum{}, &enum{Name: "color", Values: []string{"red", "blue"}}},
		{
			"bob req m1 v1",
			&namedReqs{},
			&namedReqs{
				Name: "bob",
				Reqs: []Require{{"m1", "v1"}},
			},
		},
	} {
		words := strings.Fields(tc.in)
		got := tc.p
		if err := UnmarshalValue(Value{Words: words}, got); err != nil {
			t.Fatal(err)
		}
		if g, w := vfmt.Sprint(got), vfmt.Sprint(tc.want); g != w {
			t.Errorf("%q: got\n%s\nwant\n%s", tc.in, g, w)
		}
	}
}
func TestUnmarshalValues(t *testing.T) {
	type thing struct {
		Count int
		Good  bool
	}

	type enum struct {
		Name   string
		Values []string
	}

	type nrs struct {
		Requires []Require
	}

	type Arg struct {
		Name, Type string
	}

	type command struct {
		Name string `gdl:",id"`
		Args []Arg
	}

	type commands struct {
		Commands []command
	}

	for _, tc := range []struct {
		in   string
		p    any
		want any
	}{
		{
			"require m1 v1; require m2 v2",
			&nrs{},
			&nrs{
				Requires: []Require{{"m1", "v1"}, {"m2", "v2"}},
			},
		},
		{
			"command create arg name string; command create arg size int",
			&commands{},
			&commands{
				Commands: []command{{Name: "create", Args: []Arg{{"name", "string"}, {"size", "int"}}}},
			},
		},
	} {
		vals, err := Parse(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		if err := UnmarshalValues(vals, tc.p); err != nil {
			t.Fatal(err)
		}
		if g, w := vfmt.Sprint(tc.p), vfmt.Sprint(tc.want); g != w {
			t.Errorf("%q: got\n%s\nwant\n%s", tc.in, g, w)
		}
	}
}
