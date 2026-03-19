// helpers_test.go tests unexported helper functions in the review package.
//
// This file uses package review (white-box, internal test) because
// parseCommaSeparated is unexported and cannot be tested from review_test.
// All other session tests remain in session_test.go (package review_test).
package review

import (
	"reflect"
	"testing"
)

// TestParseCommaSeparated is a table-driven test covering the full contract
// of parseCommaSeparated: split on comma, trim whitespace, drop empties.
func TestParseCommaSeparated(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "two elements with spaces",
			input: "actor-x, actor-y",
			want:  []string{"actor-x", "actor-y"},
		},
		{
			name:  "single element",
			input: "single",
			want:  []string{"single"},
		},
		{
			name:  "empty string returns nil",
			input: "",
			want:  nil,
		},
		{
			name:  "extra whitespace around elements",
			input: "  a ,  b  , c  ",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "double comma drops empty segment",
			input: "a,,b",
			want:  []string{"a", "b"},
		},
		{
			name:  "only comma returns nil",
			input: ",",
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCommaSeparated(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseCommaSeparated(%q): want %v, got %v", tc.input, tc.want, got)
			}
		})
	}
}
