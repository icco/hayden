package hayden

import "testing"

func TestSubstringMatcher(t *testing.T) {
	m, err := MatcherFor("substring")
	if err != nil {
		t.Fatalf("MatcherFor: %v", err)
	}

	cases := []struct {
		content, value string
		want           bool
	}{
		{"hello world", "world", true},
		{"hello world", "nope", false},
		{"", "x", false},
		{"anything", "", true}, // empty value is contained by everything
	}
	for _, c := range cases {
		got, err := m.Match([]byte(c.content), c.value)
		if err != nil {
			t.Fatalf("Match(%q,%q): %v", c.content, c.value, err)
		}
		if got != c.want {
			t.Errorf("Match(%q,%q) = %v, want %v", c.content, c.value, got, c.want)
		}
	}
}

func TestMatcherForUnsupported(t *testing.T) {
	for _, mt := range []string{"css", "regex", "jsonpath", "bogus"} {
		if _, err := MatcherFor(mt); err == nil {
			t.Errorf("MatcherFor(%q) expected error (not yet supported)", mt)
		}
	}
}
