package main

import "testing"

func TestIsObviousJunkSpamPatterns(t *testing.T) {
	for _, title := range []string{
		"Sponsored: save 30% with this discount code",
		"Subscribe to our newsletter for the latest deals",
		"Weekly weather forecast for London",
	} {
		if !isObviousJunk(title) { t.Fatalf("expected junk: %q", title) }
	}
	if isObviousJunk("Parliament approves a new climate policy") { t.Fatal("classified reporting as junk") }
}
