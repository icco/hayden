package hayden

import (
	"bytes"
	"fmt"
)

// Matcher decides whether fetched content matches a target's configured value.
type Matcher interface {
	Match(content []byte, value string) (bool, error)
}

type substringMatcher struct{}

func (substringMatcher) Match(content []byte, value string) (bool, error) {
	return bytes.Contains(content, []byte(value)), nil
}

// MatcherFor returns the Matcher for a match type. css, regex, and jsonpath
// arrive in a later phase.
func MatcherFor(matchType string) (Matcher, error) {
	switch matchType {
	case "substring", "":
		return substringMatcher{}, nil
	default:
		return nil, fmt.Errorf("unsupported match type %q", matchType)
	}
}
