package hayden

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
	"github.com/tidwall/gjson"
)

// maxSnippet caps the matched text kept for webhooks, history, and change detection.
const maxSnippet = 1000

// Matcher decides whether fetched content matches a target's configured value,
// and validates that value up front.
type Matcher interface {
	Match(content []byte, value string) (bool, error)
	// Find is Match that also returns the matched text.
	Find(content []byte, value string) (string, bool, error)
	Validate(value string) error
}

type substringMatcher struct{}

func (m substringMatcher) Match(content []byte, value string) (bool, error) {
	_, ok, err := m.Find(content, value)
	return ok, err
}
func (substringMatcher) Find(content []byte, value string) (string, bool, error) {
	if !bytes.Contains(content, []byte(value)) {
		return "", false, nil
	}
	return value, true, nil
}
func (substringMatcher) Validate(string) error { return nil }

// cssMatcher matches when the CSS selector selects at least one element; the
// matched text is the text of every selected element.
type cssMatcher struct{}

func (m cssMatcher) Match(content []byte, value string) (bool, error) {
	_, ok, err := m.Find(content, value)
	return ok, err
}
func (cssMatcher) Find(content []byte, value string) (string, bool, error) {
	sel, err := cascadia.Compile(value)
	if err != nil {
		return "", false, fmt.Errorf("invalid css selector %q: %w", value, err)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return "", false, fmt.Errorf("parsing html: %w", err)
	}
	found := doc.FindMatcher(sel)
	if found.Length() == 0 {
		return "", false, nil
	}
	return strings.Join(strings.Fields(found.Text()), " "), true, nil
}
func (cssMatcher) Validate(value string) error {
	_, err := cascadia.Compile(value)
	return err
}

type regexMatcher struct{}

func (m regexMatcher) Match(content []byte, value string) (bool, error) {
	_, ok, err := m.Find(content, value)
	return ok, err
}
func (regexMatcher) Find(content []byte, value string) (string, bool, error) {
	re, err := regexp.Compile(value)
	if err != nil {
		return "", false, fmt.Errorf("invalid regex %q: %w", value, err)
	}
	loc := re.FindIndex(content)
	if loc == nil {
		return "", false, nil
	}
	return string(content[loc[0]:loc[1]]), true, nil
}
func (regexMatcher) Validate(value string) error {
	_, err := regexp.Compile(value)
	return err
}

// jsonpathMatcher matches when the gjson path resolves to a value that isn't
// missing, null, or false; the matched text is that value's raw JSON.
type jsonpathMatcher struct{}

func (m jsonpathMatcher) Match(content []byte, value string) (bool, error) {
	_, ok, err := m.Find(content, value)
	return ok, err
}
func (jsonpathMatcher) Find(content []byte, value string) (string, bool, error) {
	res := gjson.GetBytes(content, value)
	if !res.Exists() || res.Type == gjson.Null || res.Type == gjson.False {
		return "", false, nil
	}
	return res.Raw, true, nil
}
func (jsonpathMatcher) Validate(value string) error {
	if value == "" {
		return errors.New("jsonpath value is required")
	}
	return nil
}

// MatcherFor returns the Matcher for a match type.
func MatcherFor(matchType string) (Matcher, error) {
	switch matchType {
	case "substring", "":
		return substringMatcher{}, nil
	case "css":
		return cssMatcher{}, nil
	case "regex":
		return regexMatcher{}, nil
	case "jsonpath":
		return jsonpathMatcher{}, nil
	default:
		return nil, fmt.Errorf("unsupported match type %q", matchType)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}
