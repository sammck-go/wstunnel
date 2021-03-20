package wstchannel

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

/*
// Validate a ChannelDescriptor
func (d ChannelDescriptor) Validate() error {
	err := d.Stub.Validate()
	if err != nil {
		return err
	}
	err = d.Skeleton.Validate()
	if err != nil {
		return err
	}
	if d.Stub.Role != ChannelEndpointRoleStub {
		return fmt.Errorf("%s: Role of stub must be ChannelEndpointRoleStub", d.String())
	}
	if d.Skeleton.Role != ChannelEndpointRoleSkeleton {
		return fmt.Errorf("%s: Role of skeleton must be ChannelEndpointRoleSkeleton", d.String())
	}

	if (!d.Reverse && d.Skeleton.Type == ChannelEndpointProtocolStdio) ||
		(d.Reverse && d.Stub.Type == ChannelEndpointProtocolStdio) {
		return fmt.Errorf("%s: STDIO endpoint must be on client proxy side", d.String())
	}

	return nil
}

*/

// Fully qualified ChannelDescriptor
//    ["R:"]<stub-type>:<stub-path>:<skeleton-type>:<skeleton-path>
//
// Where the optional "R:" prefix indicates a reverse-proxy
//   <stub-type> is one of TCP, UNIX, STDIO, or LOOP.
//   <skeleton-type> is one of: TCP, UNIX, SOCKS, STDIO, or LOOP
//   <stub-path> and <skeleton-path> are formatted according to respective type:
//        stub TCP:        <IPV4 bind addr>:<port>                          0.0.0.0:22
//                         [<IPV6 bind addr>]:<port>                        0.0.0.0:22
//        skeleton TCP:
//
// Note that any ":"-delimited descriptor element that contains a ":" may be escaped in the following ways:
//    * Except as indicated below, the presence of '[' or '<' anywhere in a descriptor element causes all
//        characters up to a balanced closing bracket to be included as part of the parsed element.
//    * An element that begins and ends with '<' and a balanced '>' will have the beginning and ending characters
//        stripped off of the final parsed element
//    '\:' will be a converted to a single ':' within an element but will not be recognized as a delimiter
//    '\\' will be converted to a single '\' within an element
//    '\<' Will be converted to a single '<' and will not be considered for bracket balancing
//    '\>' will be converted to a single '>' and will not be considered for bracket balancing
//    '\[' Will be converted to a single '[' and will not be considered for bracket balancing
//    '\]' will be converted to a single ']' and will not be considered for bracket balancing
//
//
// Short-hand conversions
//   3000 ->
//     local  127.0.0.1:3000
//     remote 127.0.0.1:3000
//   foobar.com:3000 ->
//     local  127.0.0.1:3000
//     remote foobar.com:3000
//   3000:google.com:80 ->
//     local  127.0.0.1:3000
//     remote google.com:80
//   192.168.0.1:3000:google.com:80 ->
//     local  192.168.0.1:3000
//     remote google.com:80

/*
// Validate a ChannelEndpointDescriptor
func (d ChannelEndpointDescriptor) Validate() error {
	if d.Role != ChannelEndpointRoleStub && d.Role != ChannelEndpointRoleSkeleton {
		return fmt.Errorf("%s: Unknown role type '%s'", d.String(), d.Role)
	}
	if d.Type == ChannelEndpointProtocolTCP {
		if d.Path == "" {
			if d.Role == ChannelEndpointRoleStub {
				return fmt.Errorf("%s: TCP stub endpoint requires a bind address and port", d.String())
			}
			return fmt.Errorf("%s: TCP skeleton endpoint requires a target hostname and port", d.String())
		}
		host, port, err := ParseHostPort(d.Path, "", InvalidPortNumber)
		if err != nil {
			if d.Role == ChannelEndpointRoleStub {
				return fmt.Errorf("%s: TCP stub endpoint <bind-address>:<port> is invalid: %v", d.String(), err)
			}
			return fmt.Errorf("%s: TCP skeleton endpoint <hostname>:<port> is invalid: %v", d.String(), err)
		}
		if host == "" {
			if d.Role == ChannelEndpointRoleStub {
				return fmt.Errorf("%s: TCP stub endpoint requires a bind address: %v", d.String(), err)
			}
			return fmt.Errorf("%s: TCP skeleton endpoint requires a target hostname: %v", d.String(), err)
		}
		if port == InvalidPortNumber {
			return fmt.Errorf("%s: TCP endpoint requires a port number", d.String())
		}
	} else if d.Type == ChannelEndpointProtocolUnix {
		if d.Path == "" {
			return fmt.Errorf("%s: Unix domain socket endpoint requires a socket pathname", d.String())
		}
	} else if d.Type == ChannelEndpointProtocolLoop {
		if d.Path == "" {
			return fmt.Errorf("%s: Loop endpoint requires a loop name", d.String())
		}
	} else if d.Type == ChannelEndpointProtocolStdio {
		if d.Path != "" {
			return fmt.Errorf("%s: STDIO endpoint cannot have a path", d.String())
		}
	} else if d.Type == ChannelEndpointProtocolSocks {
		if d.Path != "" {
			return fmt.Errorf("%s: SOCKS endpoint cannot have a path", d.String())
		}
		if d.Role != ChannelEndpointRoleSkeleton {
			return fmt.Errorf("%s: SOCKS endpoint must be placed on the skeleton side", d.String())
		}
	} else {
		return fmt.Errorf("%s: Unknown endpoint type '%s'", d.String(), d.Type)
	}
	return nil
}
*/

type bracketStack struct {
	btypes []rune
}

func (s *bracketStack) pushBracket(c rune) {
	s.btypes = append(s.btypes, c)
}

func (s *bracketStack) popBracket() rune {
	var c rune
	n := len(s.btypes)
	if n > 0 {
		c = s.btypes[n-1]
		s.btypes = s.btypes[:n-1]
	}
	return c
}

func (s *bracketStack) isBalanced() bool {
	return len(s.btypes) == 0
}

var closeToOpen = map[rune]rune{
	'}': '{',
	'>': '<',
	']': '[',
	')': '(',
}

var openToClose = map[rune]rune{
	'{': '}',
	'<': '>',
	'[': ']',
	'(': ')',
}

func isOpenBracket(c rune) bool {
	_, ok := openToClose[c]
	return ok
}

func isCloseBracket(c rune) bool {
	_, ok := closeToOpen[c]
	return ok
}

// ParseNextBracketedBlock returns a string containing the next bracketed block in s. s Must start with an
// open bracket.
//
//  '{':  A complete JSON object definition string parsed and returned , with JSON escaping rules
//  '[':  all balanced elements up to and including the balancing ']' are returned
//  '<':  all balanced elements up to and including the balancing '>' are returned
//  '(':  all balanced elements up to and including the balancing ')' are returned
//
// An error is returned if the s does not start with an open bracket, block terminators are mismatched,
// a block is unterminated, an escape is hanging, or a rune is incomplete.
// On return, nb is set to the number of bytes consumed from the original string.  Currently, on
// success, this is always equal to len(bs).  On error, nb is set to a best guess at the number of bytes of s
// that were consumed before an error occurred.
func parseNextBracketedBlock(s string) (bs string, nb int, err error) {
	openingRune, orsize := utf8.DecodeRuneInString(s)
	if openingRune == utf8.RuneError {
		return "", 0, fmt.Errorf("Balanced block does not begin with a valid rune")
	}
	if openingRune == '{' {
		_, nb, err := ParseNextJsonValueInString(s)
		if err != nil {
			return "", nb, err
		}
		return s[:nb], nb, err
	}
	expectedClose, ok := openToClose[openingRune]
	if !ok {
		return "", orsize, fmt.Errorf("Invalid opening bracket '%c'", openingRune)
	}

	result := make([]byte, 0, 16)

	for i := orsize; i < len(s); {
		c, csize := utf8.DecodeRuneInString(s)
		if c == utf8.RuneError {
			return "", i, fmt.Errorf("Incomplete UTF-8 rune")
		}
		if isCloseBracket(c) {
			if c != expectedClose {
				return "", i, fmt.Errorf("Mismatched brackets--found '%c'; expected '%c'", c, expectedClose)
			}
			result = append(result, string(c)...)
			return string(result), i + csize, nil
		}
		bs, nb, err := ParseNextElement(s[i:])
		if err != nil {
			return "", i + nb, err
		}
		result = append(result, bs...)
		i += nb
	}

	return "", len(s), fmt.Errorf("Unterminated bracketed block; expected '%c'", expectedClose)
}

// ParseNextSingleQuotedString parses a quoted string that begins and ends with a single quote "'".
// There is no escape mechanism.  The quotes are included in the returned string.
// An error is returned if the string does not begin with "'", the string is unterminated,
// or a rune is incomplete.
// On return, nb is set to the number of bytes consumed from the original string.  Currently, on
// success, this is always equal to len(qs).  On error, nb is set to a best guess at the number of bytes of s
// that were consumed before an error occurred.
func parseNextSingleQuotedString(s string) (qs string, nb int, err error) {
	if len(s) == 0 || s[0] != '\'' {
		return "", 0, fmt.Errorf("Expected single-quoted string")
	}

	for i := 1; i < len(s); {
		c, csize := utf8.DecodeRuneInString(s)
		if c == utf8.RuneError {
			return "", i, fmt.Errorf("Incomplete UTF-8 rune")
		}
		i += csize
		if c == '\'' {
			return s[:i], i, nil
		}
	}

	return "", len(s), fmt.Errorf("Unterminated quoted string; expected \"'\"")
}

// ParseNextDoubleQuotedString parses a quoted string that begins and ends with a double quote '"'.
// Backslash escapes are supported.  The quotes are included in the returned string, as are any backslashes.
// An error is returned if the string does not begin with '"', the string is unterminated,
// or a rune is incomplete.
// On return, nb is set to the number of bytes consumed from the original string.  Currently, on
// success, this is always equal to len(qs).  On error, nb is set to a best guess at the number of bytes of s
// that were consumed before an error occurred.
func parseNextDoubleQuotedString(s string) (qs string, nb int, err error) {
	if len(s) == 0 || s[0] != '"' {
		return "", 0, fmt.Errorf("Expected double-quoted string")
	}

	for i := 1; i < len(s); {
		c, csize := utf8.DecodeRuneInString(s)
		if c == utf8.RuneError {
			return "", i, fmt.Errorf("Incomplete UTF-8 rune")
		}
		i += csize
		if c == '\\' {
			if i >= len(s) {
				return "", i, fmt.Errorf("Dangling backslash escape")
			}
			e, esize := utf8.DecodeRuneInString(s[i:])
			if e == utf8.RuneError {
				return "", i, fmt.Errorf("Incomplete UTF-8 rune in backslash escape")
			}
			i += esize
		} else if c == '"' {
			return s[:i], i, nil
		}
	}

	return "", len(s), fmt.Errorf("Unterminated quoted string; expected '\"'")
}

// ParseNextElement returns a string containing the next rune in s unless the first character in s is
// one of the following special characters:
//
//  '\':  The backslash  and the following rune is returned as an element. An
//        error is returned if there is not a complete rune following the backslash.
//  '{':  A complete JSON object definition string parsed and returned , with JSON escaping rules
//  '[':  all balanced elements up to and including the balancing ']' are returned
//  '<':  all balanced elements up to and including the balancing '>' are returned
//  '(':  all balanced elements up to and including the balancing ')' are returned
//  '"':  allcharacters up to and including the nect '"' are returned -- backslash escaping is supported.
//  '\'':  all characters up to and including the next '\'' are returned -- no backslash escaping
//
// if s is an empty string, returns an empty string without error.
// An error is returned if block terminators are mismatched, a block is unterminated, an escape is
// hanging, or a rune is incomplete.
// On return, nb is set to the number of bytes consumed from the original string.  Currently, on
// success, this is always equal to len(bs).  On error, nb is set to a best guess at the number of bytes of s
// that were consumed before an error occurred.
func ParseNextElement(s string) (bs string, nb int, err error) {
	if s == "" {
		return "", 0, nil
	}
	c, csize := utf8.DecodeRuneInString(s)
	if c == utf8.RuneError {
		return "", 0, fmt.Errorf("Incomplete UTF-8 rune")
	}
	if c == '\\' {
		if len(s) <= csize {
			return "", 0, fmt.Errorf("Dangling backslash escape")
		}
		e, esize := utf8.DecodeRuneInString(s[csize:])
		if e == utf8.RuneError {
			return "", csize, fmt.Errorf("Incomplete UTF-8 rune in backslash escape")
		}
		return s[:csize+esize], csize + esize, nil
	} else if isOpenBracket(c) {
		bs, nb, err = parseNextBracketedBlock(s)
	} else if c == '"' {
		bs, nb, err = parseNextDoubleQuotedString(s)
	} else if c == '\'' {
		bs, nb, err = parseNextSingleQuotedString(s)
	}
	return bs, nb, err
}

// ParseElementsToDelim parses balanced elements up to but not including a provided delimeter, or to end of string,
// respecting the following escaping/grouping mechanisms:
//
//    * Except as indicated below, the presence of '[', '(', or '<' anywhere in a descriptor element causes all
//        characters up to a balanced closing bracket to be included as part of the parsed element.
//    * The presence of '{' begins a JSON-encoded object that will be parsed and escaped with JSON rules up to
//      a matching '}'
//    * A "'" character begins a single quoted string. nothing is escaped ot recognized specially until a matching "'"
//    * A '"' character begins a double quoted string. Backslash escapes are respected; nothing else is recognized specially until a matching "'"
//    * The presense of a ':' immediately followed by "//" is not recognized as a ':' delimeter
//    * '\x', where x is any rune, will be preserved and not be considered for a delimiter. The backslash is kept in.
//    * As a special case, ":" is never recognized as a delimeter if it is immediately followed by "//".
// If s is an empty string, returns an empty string without error.
// An error is returned if block terminators are mismatched, a block is unterminated, an escape is
// hanging, or a rune is incomplete.
// On return, nb is set to the number of bytes consumed from the original string, including the delimeter.
// delim is set to the delimiter that was encountered, or 0 if end of string was encountered before a delimter.
// On error, nb is set to a best guess at the number of bytes of s that were consumed before an error occurred.
func ParseElementsToDelim(s string, delims []rune) (bs string, nb int, delim rune, err error) {
	if s == "" {
		return "", 0, 0, nil
	}
	void := struct{}{}
	delimSet := map[rune]struct{}{}
	if delims != nil {
		for _, d := range delims {
			delimSet[d] = void
		}
	}

	result := make([]byte, 0, 16)

	var i int
	for i = 0; i < len(s); {
		c, _ := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError {
			return "", i, 0, fmt.Errorf("Incomplete UTF-8 rune")
		}
		if _, ok := delimSet[c]; ok {
			// special case == "://" is not a delimeter even if ":" is in delimeter list
			if c != ':' || i+3 > len(s) || s[i+1:i+3] != "//" {
				delim = c
				break
			}
		}
		e, nb, err := ParseNextElement(s[i:])
		if err != nil {
			return "", i + nb, 0, err
		}
		result = append(result, e...)
		i += nb
	}

	return string(result), i, delim, nil
}

// SplitBalanced breaks a delimited string
// into its parts, respecting the following escaping/grouping mechanisms:
//
//    * Except as indicated below, the presence of '[', '(', or '<' anywhere in a descriptor element causes all
//        characters up to a balanced closing bracket to be included as part of the parsed element.
//    * The presence of '{' begins a JSON-encoded object that will be parsed and escaped with JSON rules up to
//      a matching '}'
//    * A "'" character begins a single quoted string. nothing is escaped ot recognized specially until a matching "'"
//    * A '"' character begins a double quoted string. Backslash escapes are respected; nothing else is recognized specially until a matching "'"
//    * The presense of a ':' immediately followed by "//" is not recognized as a ':' delimeter
//    * '\x', where x is any rune, will be preserved and not be considered for a delimiter. The backslash is kept in.
// If delims is nil or empty, ':' is used as a delimiter
// If s is an empty string, an empty slice is returned without error.
// An error is returned if block terminators are mismatched, a block is unterminated, an escape is
// hanging, or a rune is incomplete.
// On error, nb is set to a best guess at the number of bytes of s that were consumed before an error occurred.
func SplitBalanced(s string, delims []rune) (parts []string, nb int, err error) {
	result := make([]string, 0, 4)

	if s == "" {
		return result, 0, nil
	}

	if delims == nil || len(delims) == 0 {
		delims = []rune{':'}
	}

	for i := 0; ; {
		e, nb, delim, err := ParseElementsToDelim(s[i:], delims)
		if err != nil {
			return nil, i + nb, err
		}
		result = append(result, e)
		i += nb
		if delim == 0 {
			break
		}
	}

	return result, len(s), nil
}

// PortNumber is a TCP port number in the range 0-65535. 0 is defined as UnknownPortNumber
// and 65535 is defined as InvalidPortNumber
type PortNumber uint16

// UnknownPortNumber is an unknown TCP port number. The zero value for PortNumber
const UnknownPortNumber PortNumber = 0

// InvalidPortNumber is an invalid TCP port number. Equal to uint16(65535)
const InvalidPortNumber PortNumber = 65535

// ParsePortNumber converts a string to a PortNumber
//   An error will be returned if the string is not a valid integer in the range
//   1-65534. If the string is 0, UnknownPortNumber will be returned as the
//   value. All other error conditionss will return InvalidPortNumber as the value.
func ParsePortNumber(s string) (PortNumber, error) {
	p64, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return InvalidPortNumber, fmt.Errorf("Invalid port number %s: %s", s, err)
	}
	p := PortNumber(uint16(p64))
	if p == InvalidPortNumber {
		err = fmt.Errorf("65535 is a reserved invalid port number")
	} else if p == UnknownPortNumber {
		err = fmt.Errorf("0 is a reserved unknown port number")
	}
	return p, err
}

func (x PortNumber) String() string {
	var result string
	if x == InvalidPortNumber {
		result = "<invalid>"
	} else if x == UnknownPortNumber {
		result = "<unknown>"
	} else {
		result = strconv.FormatUint(uint64(x), 10)
	}
	return result
}

// IsPortNumberString returns true if the string can be parsed into a valid TCP PortNumber
func IsPortNumberString(s string) bool {
	_, err := ParsePortNumber(s)
	return err == nil
}

// isAngleBracketed returns true if and only if s starts with '<' and ends with a balanced '>'. Escape
// characters and abalanced [] pairs, <> pairs, () pairs, and {} JSON blocks are accounted for.
func isAngleBracketed(s string) bool {
	if len(s) < 2 || s[0] != '<' || s[len(s)-1] != '>' {
		return false
	}

	_, nb, err := parseNextBracketedBlock(s)
	return err == nil && nb == len(s)
}

// StripAngleBrackets removes a single balanced leading and trailing '<' and '>' pair on a string, if they are present
// True is returned if stripping was performed
func StripAngleBrackets(s string) (string, bool) {
	found := false
	if isAngleBracketed(s) {
		found = true
		s = s[1 : len(s)-1]
	}
	return s, found
}

// ParseHostPort breaks a <hostname>:<port>, <hostname>, or <port> into a tuple.
//   <hostname> may contain balanced square or angle brackets, inside which ':'
//   characters are not considered as a delimiter. This allows for IPV6 host/port
//   specification such as [2001:0000:3238:DFE1:0063:0000:0000:FEFB]:80
//   In addition the entire host:port,  or just the host, (but not the port) may be enclosed in
//   angle brackets, which will be stripped.
func ParseHostPort(path string, defaultHost string, defaultPort PortNumber) (string, PortNumber, error) {
	var port PortNumber
	var host string

	bpath, _ := StripAngleBrackets(path)

	parts, nb, err := SplitBalanced(bpath, []rune{':'})
	if err != nil {
		return "", InvalidPortNumber, fmt.Errorf("Invalid TCP host/port string at offset %d of \"%s\": %v", nb, path, err)
	}

	if len(parts) > 2 {
		return "", InvalidPortNumber, fmt.Errorf("Too many ':'-delimited parts in TCP host/port string \"%s\"", path)
	} else if len(parts) == 1 {
		part := parts[0]
		port, err = ParsePortNumber(part)
		if err != nil {
			port = UnknownPortNumber
			host, _ = StripAngleBrackets(part)
		}
	} else if len(parts) == 2 {
		host, _ = StripAngleBrackets(parts[0])
		port, err = ParsePortNumber(parts[1])
		if err != nil {
			return "", InvalidPortNumber, fmt.Errorf("Invalid port in TCP host/port string \"%s\": %s", path, err)
		}
	}

	if host == "" {
		host = defaultHost
	}

	if port == UnknownPortNumber {
		port = defaultPort
	}

	return host, port, nil
}

// parseProtocolPrefix parses a protocol prefix from the front of a string.
// If the string has a <protocol>:// prefix, returns the protocol and the number of bytes to skip to get past the "//"
// If the string does not have a protocol prefix, returns an empty string and 0.
// The protocol is normalized to lowercase. Protocols must consist of only a-z, A-Z, 0-9, and must start with a letter.
func parseProtocolPrefix(s string) (protocol ChannelEndpointProtocol, nb int) {
	for i, c := range s {
		if c == ':' {
			if i < 1 || len(s) < i+3 || s[i+1:i+3] != "//" {
				return "", 0
			}
			protocol = ChannelEndpointProtocol(strings.ToLower(s[:i]))
			nb = i + 3
			return protocol, nb
		} else if c >= 'a' && c <= 'z' {
		} else if c >= 'A' && c <= 'Z' {
		} else if c >= '0' && c <= '9' {
			if i == 0 {
				return "", 0
			}
		} else {
			return "", 0
		}
	}

	return "", 0
}

// ParseNextLegacyChannelEndpointItem parses the next endpoint or endpoint:port out of a presplit ":"-delimited string,
// returning the remainder of unparsed parts
func ParseNextLegacyChannelEndpointDescriptor(parts []string) (epProtocol ChannelEndpointProtocol, epParams string, port PortNumber, remParts []string, nb int, err error) {
	s := strings.Join(parts, ":")
	if len(parts) <= 0 {
		return ChannelEndpointProtocolUnknown, "", UnknownPortNumber, nil, 0, fmt.Errorf("Empty endpoint descriptor: '%s'", s)
	}
	if IsPortNumberString(parts[0]) {
		port, _ = ParsePortNumber(parts[0])
		return ChannelEndpointProtocolTCP, "", port, parts[1:], len(parts[0]), nil
	}
	sp := strings.ToLower(parts[0])
	if sp == "stdio" {
		return ChannelEndpointProtocolStdio, "", UnknownPortNumber, parts[1:], len(parts[0]), nil
	} else if sp == "socks" {
		return ChannelEndpointProtocolStdio, "", UnknownPortNumber, parts[1:], len(parts[0]), nil
	} else {
		port = UnknownPortNumber
		np := 1
		nb = len(parts[0])
		if len(parts) > 1 && IsPortNumberString(parts[0]) {
			port, _ = ParsePortNumber(parts[1])
			np = 2
			nb += len(parts[1]) + 1
		}
		return ChannelEndpointProtocolTCP, parts[0], port, parts[np:], nb, nil
	}
}

// ParseLegacyChannelDescriptorPath parses a concise string into a ChannelDescriptor.
//
// A path is constructed as:
//
//        [ "R:" ] <forward-channel-spec> ]
//
//     The optional "R:" prefix indicates a reverse-proxy.
//
// <forward-channel-spec> is one of:
//
//     socks
//     <legacy-stub-spec> [ ":" <legacy-skeleton-spec> ]
//
//       <legacy-stub-spec> is one of:
//         <IPV4-bind-address>
//         '[' <IPV6-bind-address ']'
//         <TCP-bind-port-number>
//         <IPV4-bind address> ':' <TCP bind port number>
//         '[' <IPV6 bind address> ']' ':' <TCP bind port number>
//         "stdio"
//		   "socks"
//
//       <legacy-skeleton-spec> is one of:
//         <TCP target port number>
//         <IPV4 target address> ':' <TCP bind port number>
//         '[' <IPV6 target address> ']' ':' <TCP bind port number>
//         <target hostname> ':' <TCP bind port number>
//         "socks"
// If an error occurs, nb indicates a best guess at the byte offset of the error.
func ParseLegacyChannelDescriptorPath(s string) (d ChannelEndpointDescriptor, nb int, err error) {
	reverse := false
	nbr := 0
	if strings.HasPrefix(s, "R:") {
		nbr = 2
		reverse = true
	}

	parts, nb, err := SplitBalanced(s[nbr:], []rune{':'})
	if err != nil {
		return nil, nb, fmt.Errorf("Invalid channel descriptor at offset %d of \"%s\": %v", utf8.RuneCountInString(s[:nb]), err)
	}
	if len(parts) == 0 {
		return nil, len(s), fmt.Errorf("Empty channel descriptor \"%s\"", s)
	}
	stubProtocol, stubParams, stubPort, remParts1, nb1, err := ParseNextLegacyChannelEndpointDescriptor(parts)
	if err != nil {
		return nil, nbr + nb1, fmt.Errorf("Invalid stub channel descriptor \"%s\": %v", s, err)
	}
	var skeletonProtocol ChannelEndpointProtocol
	var skeletonPort PortNumber = UnknownPortNumber
	var skeletonParams string
	if len(remParts1) == 0 {
		skeletonProtocol = ChannelEndpointProtocolUnknown
		skeletonPort = UnknownPortNumber
		skeletonParams = ""
	} else {
		var remParts2 []string
		var nb2 int
		skeletonProtocol, skeletonParams, skeletonPort, remParts2, nb2, err = ParseNextLegacyChannelEndpointDescriptor(remParts1)
		if err != nil {
			return nil, nbr + nb1 + 1 + nb2, fmt.Errorf("Invalid skeleton channel descriptor \"%s\": %v", s, err)
		}
		if len(remParts2) > 0 {
			return nil, nbr + nb1 + 1 + nb2, fmt.Errorf("Extraneous ':' delimeter in channel descriptor \"%s\"", s)
		}
	}

	if skeletonProtocol == ChannelEndpointProtocolUnknown {
		skeletonProtocol = ChannelEndpointProtocolTCP
	}

	if stubProtocol == ChannelEndpointProtocolTCP && stubParams == "" {
		if skeletonProtocol == ChannelEndpointProtocolSocks {
			stubParams = "127.0.0.1"
		} else {
			stubParams = "0.0.0.0"
		}
	}

	if stubProtocol == ChannelEndpointProtocolTCP && stubPort == UnknownPortNumber {
		if skeletonProtocol == ChannelEndpointProtocolSocks {
			stubPort = PortNumber(1080)
		} else if skeletonPort != UnknownPortNumber {
			stubPort = skeletonPort
		}
	}

	if skeletonProtocol == ChannelEndpointProtocolTCP && skeletonPort == UnknownPortNumber {
		if stubPort != UnknownPortNumber {
			skeletonPort = stubPort
		}
	}

	if stubProtocol == ChannelEndpointProtocolTCP {
		if stubParams == "" {
			return nil, len(s), fmt.Errorf("Unable to determine stub bind address in channel descriptor string: '%s'", s)
		}
		if stubPort == UnknownPortNumber {
			return nil, len(s), fmt.Errorf("Unable to determine stub port number in channel descriptor string: '%s'", s)
		}
		stubParams = fmt.Sprintf("%s:%d", stubParams, stubPort)
	}

	if skeletonProtocol == ChannelEndpointProtocolTCP {
		if skeletonParams == "" {
			skeletonParams = "localhost"
		}
		if skeletonPort == UnknownPortNumber {
			return nil, len(s), fmt.Errorf("Unable to determine skeleton port number in channel descriptor string: '%s'", s)
		}
		skeletonParams = fmt.Sprintf("%s:%d", skeletonParams, skeletonPort)
	}

	if skeletonProtocol == ChannelEndpointProtocolUnknown {
		return nil, len(s), fmt.Errorf("Unable to determine skeleton endpoint type: '%s'", s)
	}

	stub, _, err := NewChannelEndpointDescriptorWithParamsPath(ChannelEndpointRoleStub, stubProtocol, "", stubParams, false)
	if err != nil {
		return nil, len(s), fmt.Errorf("Invalid stub descriptor \"%s\": %v", s, err)
	}

	skeleton, _, err := NewChannelEndpointDescriptorWithParamsPath(ChannelEndpointRoleSkeleton, skeletonProtocol, "", skeletonParams, false)
	if err != nil {
		return nil, len(s), fmt.Errorf("Invalid skeleton descriptor \"%s\": %v", s, err)
	}

	d, err = NewChannelDescriptor(stub, skeleton, reverse)

	return d, len(s), err
}

// ParseFullEndpointDescriptorPath parses a concise string into a ChannelEndpointDescriptor.
//
//         [ "stub:" | "skeleton:" ] <protocol> "://" [ <protocol-params> ]
//
//  Embedded quoted strings, backslash escaping, balanced brackets of all kinds, and balanced json object definition
//  If role is ChannelEndpointRoleUnknown, then either a "stub:" or "skeleton:" prefix must be present.
//  If role is not ChannelEndpointRoleUnknown then if a "stub:" or "skeleton:" prefix is present, it must match the provided role.
// If the first character in <protocol-params> is '{', then it is parsed as JSON and provided to the descriptor in object form.
// If an error occurs, nb indicates a best guess at the byte offset of the error.
func ParseFullEndpointDescriptorPath(s string, role ChannelEndpointRole) (d ChannelEndpointDescriptor, nb int, err error) {
	rnb := 0

	parsedRole := ChannelEndpointRoleUnknown
	if strings.HasPrefix(s, string(ChannelEndpointRoleStub)+":") {
		parsedRole = ChannelEndpointRoleStub
		rnb = len(ChannelEndpointRoleStub) + 1
	} else if strings.HasPrefix(s, string(ChannelEndpointRoleSkeleton)+":") {
		parsedRole = ChannelEndpointRoleSkeleton
		rnb = len(ChannelEndpointRoleSkeleton) + 1
	}
	if role == ChannelEndpointRoleUnknown {
		if parsedRole == ChannelEndpointRoleUnknown {
			return nil, rnb, fmt.Errorf("Endpoint descriptor missing required role (stub or skeleton): \"%s\"", s)
		}
		role = parsedRole
	} else {
		if parsedRole != ChannelEndpointRoleUnknown && parsedRole != role {
			return nil, rnb, fmt.Errorf("Endpoint descriptor has role %s; expected %s: \"%s\"", parsedRole, role, s)
		}
	}

	protocol, nbProtocol := parseProtocolPrefix(s[rnb:])
	if protocol == "" {
		return nil, rnb, fmt.Errorf("Endpoint descriptor missing required <protocol>:// prefix: \"%s\"", s)
	}
	nbp := rnb + nbProtocol
	paramsPath := s[nbp:]

	d, nb, err = NewChannelEndpointDescriptorWithParamsPath(role, protocol, "", paramsPath, true)
	if err != nil {
		return nil, nbp + nb, fmt.Errorf("Invalid endpoint descriptor at char offset %d of \"%s\": %v", utf8.RuneCountInString(s[:nbp+nb]), s, err)
	}
	return d, len(s), nil
}

// ParseFullChannelDescriptorPath parses a concise string into a ChannelDescriptor.
//
// A path is constructed as:
//
//        [ "R:" ] <full-stub-spec> ',' <full-skeleton-spec>
//
//     The optional "R:" prefix indicates a reverse-proxy.
//
//     <full-stub-spec> is:
//         [ "stub:" ] <protocol> "://" [ <protocol-params> ]
//
//     <full-skeleton-spec> is:
//         [ "skeleton:" ] <protocol> "://" [ <protocol-params> ]
//
//  Embedded quoted strings, backslash escaping, balanced brackets of all kinds, and balanced json object definition
//  strings are respected to prevent ambiguity in delimiting. The entire <forward-channel-spec> may be optionally enclosed in angle brackets
//  "<>" which will be stripped off if present. If the first character in <protocol-params> is '{', then it is parsed as JSON and provided
//  to the descriptor in object form.
//  If an error occurs, nb indicates a best guess at the byte offset of the error.
func ParseFullChannelDescriptorPath(s string) (d ChannelDescriptor, nb int, err error) {
	reverse := false
	rnb := 0
	if strings.HasPrefix(s, "R:") {
		rnb = 2
		reverse = true
	}
	parts, nb, err := SplitBalanced(s[rnb:], []rune{','})
	if err != nil {
		return nil, rnb + nb, fmt.Errorf("Invalid channel descriptor at offset %d of \"%s\": %v", utf8.RuneCountInString(s[:rnb+nb]), err)
	}
	if len(parts) < 2 {
		return nil, len(s), fmt.Errorf("Missing comma in channel descriptor \"%s\"", s)
	}
	boffs = []int{rnb, rnb + len(parts[0]) + 1, rnb + len(parts[0]) + 1 + len(parts[1])}
	if len(parts) > 2 {
		return nil, boffs[2], fmt.Errorf("Extraneous comma at char offset %d of channel descriptor \"%s\"",
			utf8.RuneCountInString(s[:boffs[2]]), s)
	}
	stub, nb0, err := ParseFullEndpointDescriptorPath(parts[0], ChannelEndpointRoleStub)
	if err != nil {
		return nil, boffs[0] + nb0, fmt.Errorf("Bad stub descriptor at char offset %d of \"%s\": %v",
			utf8.RuneCountInString(s[:boffs[0]+nb0]), s, err)
	}
	skeleton, nb1, err := ParseFullEndpointDescriptorPath(parts[1], ChannelEndpointRoleSkeleton)
	if err != nil {
		return nil, boffs[1] + nb1, fmt.Errorf("Bad skeleton descriptor at char offset %d of \"%s\": %v",
			utf8.RuneCountInString(s[:boffs[1]+nb1]), s, err)
	}
	d, err = NewChannelDescriptor(stub, skeleton, reverse)
	return d, len(s), err
}

// ParseChannelDescriptorPath parses a concise string into a ChannelDescriptor.
//
// A path is constructed as:
//
//        [ "R:" ] <forward-channel-spec> ]
//
//     The optional "R:" prefix indicates a reverse-proxy.
//
// <forward-channel-spec> may be presented on one of two forms (the presence of a ","" or "://" anywhere
//          in the path indicates the full form):
//
//   Legacy/abbreviated form (suitable only for TCP and unparameterized socks/stdio endpoints) is one of:
//     socks
//     <legacy-stub-spec> [ ":" <legacy-skeleton-spec> ]
//
//       <legacy-stub-spec> is one of:
//         <IPV4-bind-address>
//         '[' <IPV6-bind-address ']'
//         <TCP-bind-port-number>
//         <IPV4-bind address> ':' <TCP bind port number>
//         '[' <IPV6 bind address> ']' ':' <TCP bind port number>
//         "stdio"
//		   "socks"
//
//       <legacy-skeleton-spec> is one of:
//         <TCP target port number>
//         <IPV4 target address> ':' <TCP bind port number>
//         '[' <IPV6 target address> ']' ':' <TCP bind port number>
//         <target hostname> ':' <TCP bind port number>
//         "socks"
//
//
//   Full/extensible form of <forward-channel-spec>:
//
//        <full-stub-spec> ',' <full-skeleton-spec>
//
//     <full-stub-spec> is:
//         [ "stub:" ] <protocol> "://" [ <protocol-params> ]
//
//     <full-skeleton-spec> is:
//         [ "skeleton:" ] <protocol> "://" [ <protocol-params> ]
//
//    In the full form, embedded quoted strings, backslash escaping, balanced brackets of all kinds, and balanced json object definition
//    strings are respected to prevent ambiguity in delimiting. The entire <forward-channel-spec> may be optionally enclosed in angle brackets
//    "<>" which will be stripped off if present. If the first character in <protocol-params> is '{', then it is parsed as JSON and provided
//    to the descriptor in object form.
//
//  If an error occurs, nb indicates a best guess at the byte offset of the error.
func ParseChannelDescriptorPath(s string) (d ChannelDescriptor, nb int, err error) {
	if strings.Contains(s, ",") || strings.Contains(s, "://") {
		d, nb, err = ParseFullChannelDescriptorPath(s)
	} else {
		d, nb, err = ParseLegacyChannelDescriptorPath(s)
	}
	return d, nb, err
}
