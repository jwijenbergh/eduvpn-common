// Package util implements several utility functions that are used across the codebase
package util

import (
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/go-errors/errors"
)

// MakeRandomByteSlice creates a cryptographically random bytes slice of `size`
// It returns the byte slice (or nil if error) and an error if it could not be generated.
func MakeRandomByteSlice(n int) ([]byte, error) {
	bs := make([]byte, n)
	if _, err := rand.Read(bs); err != nil {
		return nil, errors.WrapPrefix(err, "failed reading random", 0)
	}
	return bs, nil
}

// EnsureDirectory creates a directory with permission 700.
func EnsureDirectory(dir string) error {
	// Create with 700 permissions, read, write, execute only for the owner
	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		return errors.WrapPrefix(err, fmt.Sprintf("failed to create directory '%s'", dir), 0)
	}
	return nil
}

// ReplaceWAYF replaces an authorization template containing of @RETURN_TO@ and @ORG_ID@ with the authorization URL and the organization ID
// See https://github.com/eduvpn/documentation/blob/dc4d53c47dd7a69e95d6650eec408e16eaa814a2/SERVER_DISCOVERY_SKIP_WAYF.md
func ReplaceWAYF(template string, authURL string, orgID string) string {
	// We just return the authURL in the cases where the template is not given or is invalid
	if template == "" {
		return authURL
	}
	if !strings.Contains(template, "@RETURN_TO@") {
		return authURL
	}
	if !strings.Contains(template, "@ORG_ID@") {
		return authURL
	}
	// Replace authURL
	template = strings.Replace(template, "@RETURN_TO@", url.QueryEscape(authURL), 1)

	// If now there is no more ORG_ID, return as there weren't enough @ symbols
	if !strings.Contains(template, "@ORG_ID@") {
		return authURL
	}
	// Replace ORG ID
	template = strings.Replace(template, "@ORG_ID@", url.QueryEscape(orgID), 1)
	return template
}
