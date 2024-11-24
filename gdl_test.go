// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TODO: test errors

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want [][]any
	}{
		{"", nil},
		{"x", [][]any{{"x"}}},
		{"x y z", [][]any{{"x", "y", "z"}}},
		{"x 2.5 3 true", [][]any{{"x", 2.5, int64(3), true}}},
		{
			"// a file\nx 17 \"true\"\ny 22.5 \\\ntrue",
			[][]any{
				{"x", int64(17), "true"},
				{"y", 22.5, true},
			},
		},
		{
			"a (\nb c)",
			[][]any{
				{"a", "b", "c"},
			},
		},
		{
			`a (
				b x
				c y
			)`,
			[][]any{
				{"a", "b", "x"},
				{"a", "c", "y"},
			},
		},
		{
			`a (
				b x
				
				// second
				c y
			)`,
			[][]any{
				{"a", "b", "x"},
				{"a", "c", "y"},
			},
		},
		{
			"a b (\nc\nd\n)",
			[][]any{
				{"a", "b", "c"},
				{"a", "b", "d"},
			},
		},
	} {
		got, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !cmp.Equal(got, tc.want) {
			t.Errorf("%q:\ngot  %v\nwant %v", tc.in, got, tc.want)
		}
	}
}
