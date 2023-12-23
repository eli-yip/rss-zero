package md

import (
	"testing"
)

func TestH1(t *testing.T) {
	input := "Hello"
	expected := "# Hello"
	if result := H1(input); result != expected {
		t.Errorf("H1(%q) = %q; want %q", input, result, expected)
	}
}

func TestH2(t *testing.T) {
	input := "Hello"
	expected := "## Hello"
	if result := H2(input); result != expected {
		t.Errorf("H2(%q) = %q; want %q", input, result, expected)
	}
}

func TestH3(t *testing.T) {
	input := "Hello"
	expected := "### Hello"
	if result := H3(input); result != expected {
		t.Errorf("H3(%q) = %q; want %q", input, result, expected)
	}
}

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"Hello":            "> Hello",
		"Hello\nWorld":     "> Hello\n> World",
		"Hello\nWorld\n\n": "> Hello\n> World",
		"Hello\n\nWorld":   "> Hello\n> \n> World",
	}
	for input, expected := range cases {
		result := Quote(input)
		if result != expected {
			t.Errorf("Quote(%q) = %q; want %q", input, result, expected)
		}
	}
}

func TestBold(t *testing.T) {
	input := "Hello"
	expected := "**Hello**"
	if result := Bold(input); result != expected {
		t.Errorf("Bold(%q) = %q; want %q", input, result, expected)
	}
}
