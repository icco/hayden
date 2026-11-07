package hayden

import (
	"strings"
	"testing"
)

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

func TestMatcherFind(t *testing.T) {
	cases := []struct {
		name, mt, content, value, want string
	}{
		{"substring", "substring", "hello world", "world", "world"},
		{"css text", "css", `<p class="price"> $499
		</p><p class="price">$549</p>`, ".price", "$499 | $549"},
		{"regex", "regex", "now only $499.99!", `\$\d+\.\d+`, "$499.99"},
		{"jsonpath", "jsonpath", `{"price":{"amount":499}}`, "price", `{"amount":499}`},
		{"jsonpath string", "jsonpath", `{"name":"ps5"}`, "name", "ps5"},
	}
	for _, c := range cases {
		m, _ := MatcherFor(c.mt)
		got, ok, err := m.Find([]byte(c.content), c.value)
		if err != nil || !ok || got != c.want {
			t.Errorf("%s: Find = (%q, %v, %v), want %q", c.name, got, ok, err, c.want)
		}
	}
}

func TestBoundedTextAndExcerpt(t *testing.T) {
	m := cssMatcher{}
	got, _, _ := m.Find([]byte(`<b>1</b><b>23</b>`), "b")
	if got != "1 | 23" {
		t.Errorf("css text = %q, want %q", got, "1 | 23")
	}
	nested := strings.Repeat("<div>", 50) + strings.Repeat("word ", 20000) + strings.Repeat("</div>", 50)
	got, _, _ = m.Find([]byte(nested), "div")
	if len(got) > maxMatchText+100 {
		t.Errorf("css text len = %d, want about %d", len(got), maxMatchText)
	}
	if e := excerpt("a\x00b\xff", 10); e != "ab" {
		t.Errorf("excerpt = %q, want %q", e, "ab")
	}
}

func TestBoundedTextKeepsNodeWhitespace(t *testing.T) {
	m := cssMatcher{}
	a, _, _ := m.Find([]byte(`<p>foo<span>bar</span></p>`), "p")
	b, _, _ := m.Find([]byte(`<p>foo <span>bar</span></p>`), "p")
	if a != "foobar" || b != "foo bar" {
		t.Errorf("got %q and %q, want %q and %q", a, b, "foobar", "foo bar")
	}
}
