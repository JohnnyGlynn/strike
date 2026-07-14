package shared

import (
	"fmt"
	"strings"
)

// StrikeAddress represents a user@domain addressing pair.
type StrikeAddress struct {
	Username string
	Domain   string
}

// ParseAddress splits "username@domain" into its parts.
// A bare username (no @) is treated as local — Domain will be empty.
func ParseAddress(raw string) (StrikeAddress, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return StrikeAddress{}, fmt.Errorf("empty address")
	}

	parts := strings.SplitN(raw, "@", 2)
	if len(parts) == 1 {
		return StrikeAddress{Username: parts[0]}, nil
	}

	if parts[0] == "" {
		return StrikeAddress{}, fmt.Errorf("missing username in address %q", raw)
	}
	if parts[1] == "" {
		return StrikeAddress{}, fmt.Errorf("missing domain in address %q", raw)
	}

	return StrikeAddress{Username: parts[0], Domain: parts[1]}, nil
}

// FormatAddress returns "username@domain", or just "username" if domain is empty.
func FormatAddress(username, domain string) string {
	if domain == "" {
		return username
	}
	return username + "@" + domain
}

// Format returns the string form of the address.
func (a StrikeAddress) Format() string {
	return FormatAddress(a.Username, a.Domain)
}

// IsLocal returns true if the address has no domain (implying the home server).
func (a StrikeAddress) IsLocal() bool {
	return a.Domain == ""
}

// IsRemote returns true when a domain is present.
func (a StrikeAddress) IsRemote() bool {
	return a.Domain != ""
}
