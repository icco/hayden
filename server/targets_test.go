package main

import (
	"testing"

	"go.icco.me/hayden"
)

func TestValidateTarget(t *testing.T) {
	good := &hayden.Target{URL: "https://example.com/ps5", MatchValue: "in stock"}
	if err := validateTarget(good); err != nil {
		t.Fatalf("valid target: %v", err)
	}
	if good.MatchType != "substring" || good.FetchMode != "http" || good.NotifyMode != "once" {
		t.Errorf("defaults not applied: %+v", good)
	}

	for name, tg := range map[string]*hayden.Target{
		"missing url":   {},
		"file url":      {URL: "file:///etc/passwd"},
		"relative url":  {URL: "/ps5"},
		"bad hook":      {URL: "https://a.example", Hook: "gopher://x"},
		"bad regex":     {URL: "https://a.example", MatchType: "regex", MatchValue: "("},
		"bad fetch":     {URL: "https://a.example", FetchMode: "pigeon"},
		"bad notify":    {URL: "https://a.example", NotifyMode: "always"},
		"negative wait": {URL: "https://a.example", Period: -1},
	} {
		if err := validateTarget(tg); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
