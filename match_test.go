package hayden

import "testing"

func TestMatchers(t *testing.T) {
	cases := []struct {
		name, mt, content, value string
		want                     bool
	}{
		{"substring hit", "substring", "hello world", "world", true},
		{"substring miss", "substring", "hello world", "nope", false},
		{"css hit", "css", `<div class="x">hi</div>`, ".x", true},
		{"css miss", "css", `<div class="x">hi</div>`, ".y", false},
		{"regex hit", "regex", "abc123", `\d+`, true},
		{"regex miss", "regex", "abcdef", `\d+`, false},
		{"jsonpath true", "jsonpath", `{"inStock":true}`, "inStock", true},
		{"jsonpath false", "jsonpath", `{"inStock":false}`, "inStock", false},
		{"jsonpath missing", "jsonpath", `{"inStock":true}`, "other", false},
		{"jsonpath string", "jsonpath", `{"name":"ps5"}`, "name", true},
		{"jsonpath filter", "jsonpath", `{"items":[{"stock":0},{"stock":3}]}`, "items.#(stock>0)", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := MatcherFor(c.mt)
			if err != nil {
				t.Fatalf("MatcherFor(%q): %v", c.mt, err)
			}
			got, err := m.Match([]byte(c.content), c.value)
			if err != nil {
				t.Fatalf("Match: %v", err)
			}
			if got != c.want {
				t.Errorf("Match(%q, %q) = %v, want %v", c.content, c.value, got, c.want)
			}
		})
	}
}

func TestMatcherValidate(t *testing.T) {
	ok := []struct{ mt, value string }{
		{"substring", "anything"},
		{"css", ".add-to-cart"},
		{"regex", `\d{4}`},
		{"jsonpath", "inStock"},
	}
	for _, c := range ok {
		m, _ := MatcherFor(c.mt)
		if err := m.Validate(c.value); err != nil {
			t.Errorf("Validate(%q, %q) = %v, want nil", c.mt, c.value, err)
		}
	}

	bad := []struct{ mt, value string }{
		{"css", "a["},     // malformed selector
		{"regex", "(unclosed"}, // malformed regex
		{"jsonpath", ""},  // empty path
	}
	for _, c := range bad {
		m, _ := MatcherFor(c.mt)
		if err := m.Validate(c.value); err == nil {
			t.Errorf("Validate(%q, %q) = nil, want error", c.mt, c.value)
		}
	}
}

func TestMatcherForUnsupported(t *testing.T) {
	for _, mt := range []string{"xpath", "bogus"} {
		if _, err := MatcherFor(mt); err == nil {
			t.Errorf("MatcherFor(%q) expected error", mt)
		}
	}
}
