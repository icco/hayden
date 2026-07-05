package hayden

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
	"github.com/tidwall/gjson"
)

// Matcher decides whether fetched content matches a target's configured value,
// and validates that value up front.
type Matcher interface {
	Match(content []byte, value string) (bool, error)
	Validate(value string) error
}

type substringMatcher struct{}

func (substringMatcher) Match(content []byte, value string) (bool, error) {
	return bytes.Contains(content, []byte(value)), nil
}
func (substringMatcher) Validate(string) error { return nil }

// cssMatcher matches when the CSS selector selects at least one element.
type cssMatcher struct{}

func (cssMatcher) Match(content []byte, value string) (bool, error) {
	sel, err := cascadia.Compile(value)
	if err != nil {
		return false, fmt.Errorf("invalid css selector %q: %w", value, err)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return false, fmt.Errorf("parsing html: %w", err)
	}
	return doc.FindMatcher(sel).Length() > 0, nil
}
func (cssMatcher) Validate(value string) error {
	_, err := cascadia.Compile(value)
	return err
}

type regexMatcher struct{}

func (regexMatcher) Match(content []byte, value string) (bool, error) {
	re, err := regexp.Compile(value)
	if err != nil {
		return false, fmt.Errorf("invalid regex %q: %w", value, err)
	}
	return re.Match(content), nil
}
func (regexMatcher) Validate(value string) error {
	_, err := regexp.Compile(value)
	return err
}

// jsonpathMatcher matches when the gjson path resolves to a value that isn't
// missing, null, or false.
type jsonpathMatcher struct{}

func (jsonpathMatcher) Match(content []byte, value string) (bool, error) {
	res := gjson.GetBytes(content, value)
	if !res.Exists() {
		return false, nil
	}
	switch res.Type {
	case gjson.Null, gjson.False:
		return false, nil
	default:
		return true, nil
	}
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
