package http

import (
	"testing"
)

func TestEnsureValidURL(t *testing.T) {
	_, validErr := EnsureValidURL("%notvalid%", true)

	if validErr == nil {
		t.Fatal("Got nil error, want: non-nil")
	}

	testCases := map[string]string{
		// Make sure we set https
		"example.com/": "https://example.com/",
		// Make sure we do override the scheme to https
		"http://example.com/": "https://example.com/",
		// This URL is already valid
		"https://example.com/": "https://example.com/",
		// Make sure to add a trailing slash (/)
		"https://example.com": "https://example.com/",
		// Cleanup the path 1
		"https://example.com/////": "https://example.com/",
		// Cleanup the path 2
		"https://example.com/..": "https://example.com/",
	}

	for k, v := range testCases {
		valid, validErr := EnsureValidURL(k, true)
		if validErr != nil {
			t.Fatalf("Got: %v, want: nil", validErr)
		}
		if valid != v {
			t.Fatalf("Got: %v, want: %v", valid, v)
		}
	}
}

func Test_JoinURLPath(t *testing.T) {
	cases := []struct {
		u    string
		p    string
		want string
	}{
		{u: "https://example.com", p: "test", want: "https://example.com/test"},
		{u: "https://example.com", p: "/test", want: "https://example.com/test"},
		{u: "https://example.com", p: "../test", want: "https://example.com/test"},
		{u: "https://example.com", p: "../test/", want: "https://example.com/test"},
		{u: "https://example.com", p: "test/", want: "https://example.com/test"},
	}
	for _, c := range cases {
		got, err := JoinURLPath(c.u, c.p)
		if err != nil {
			t.Fatalf("Failed to parse join url case: %v, err: %v", c, err)
		}
		if got != c.want {
			t.Fatalf("Failed test case for joining URL, want: %v, got: %v", c.want, got)
		}
	}
}
