package store

import "testing"

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern, key string
		want         bool
	}{
		{"*", "", true},
		{"*", "any/thing", true},
		{"user:*", "user:42", true},
		{"user:*", "order:42", false},
		{"h?llo", "hello", true},
		{"h?llo", "hllo", false},
		{"h?llo", "h/llo", true},
		{"h*llo", "heeeello", true},
		{"h*llo", "hello!", false},
		{"h**o", "hello", true},
		{"h[ae]llo", "hallo", true},
		{"h[ae]llo", "hillo", false},
		{"h[^e]llo", "hallo", true},
		{"h[^e]llo", "hello", false},
		{"h[a-c]llo", "hbllo", true},
		{"h[c-a]llo", "hbllo", true},
		{"h[a-c]llo", "hdllo", false},
		{`h\*llo`, "h*llo", true},
		{`h\*llo`, "hello", false},
		{`[\]]`, "]", true},
		{"[abc", "b", true},
		{"a*b*c", "aXXbYYc", true},
		{"a*b*c", "aXXbYY", false},
	}
	for _, test := range tests {
		if got := matchGlob(test.pattern, test.key); got != test.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", test.pattern, test.key, got, test.want)
		}
	}
}
