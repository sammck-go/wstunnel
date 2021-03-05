package chshare

import (
	"regexp"
	"strings"
)

// UserAllowAll is a regular expression used to match any address
var UserAllowAll = regexp.MustCompile("")

// ParseAuth parses a ":"-delimited authorization string pair. Returns
// two empty strings if the input does not contain ":"
func ParseAuth(auth string) (string, string) {
	if strings.Contains(auth, ":") {
		pair := strings.SplitN(auth, ":", 2)
		return pair[0], pair[1]
	}
	return "", ""
}

// User describes a single user's authorization info, including name, password,
// and a list of channel endpoint regular expressions that are allowed
type User struct {
	Name  string
	Pass  string
	Addrs []*regexp.Regexp
}

// HasAccess returns True if a given address matches the allowed address patterns
// for the user
func (u *User) HasAccess(addr string) bool {
	m := false
	for _, r := range u.Addrs {
		if r.MatchString(addr) {
			m = true
			break
		}
	}
	return m
}
