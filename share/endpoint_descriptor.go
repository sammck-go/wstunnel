package chshare

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpillora/chisel/chprotobuf"
)

// ChannelEndpointRole defines whether an endpoint is acting as
// the Stub or the Skeleton for al ChannelEndpointPair
type ChannelEndpointRole string

const (
	// ChannelEndpointRoleUnknown is an unknown (uninitialized) endpoint role
	ChannelEndpointRoleUnknown ChannelEndpointRole = "unknown"

	// ChannelEndpointRoleStub is associated with an endpoint whose chisel
	// instance must listen for and accept new connections from local clients,
	// forward connection requests to the remote proxy where services
	// are locally available. An Stub endpoint may accept multiple
	// connection requests from local clients, resulting in multiple concurrent
	// tunnelled connections to the remote Skeleton endpoint proxy.
	ChannelEndpointRoleStub ChannelEndpointRole = "stub"

	// ChannelEndpointRoleSkeleton is associated with an endpoint whose chisel
	// instance must accept connection requests from the remote proxy
	// and actively reach out and connect to locally available services.
	// A Skeleton endpoint may accept multiple connection requests from the
	// remote proxy, resulting in multiple concurrent socket connections to
	// locally avaiable services
	ChannelEndpointRoleSkeleton ChannelEndpointRole = "skeleton"
)

var pbEndpointRoleToChannelEndpointRole = map[chprotobuf.PbEndpointRole]ChannelEndpointRole{
	chprotobuf.PbEndpointRole_UNKNOWN:  ChannelEndpointRoleUnknown,
	chprotobuf.PbEndpointRole_STUB:     ChannelEndpointRoleStub,
	chprotobuf.PbEndpointRole_SKELETON: ChannelEndpointRoleSkeleton,
}

var channelEndpointRoleToPbEndpointRole = map[ChannelEndpointRole]chprotobuf.PbEndpointRole{
	ChannelEndpointRoleUnknown:  chprotobuf.PbEndpointRole_UNKNOWN,
	ChannelEndpointRoleStub:     chprotobuf.PbEndpointRole_STUB,
	ChannelEndpointRoleSkeleton: chprotobuf.PbEndpointRole_SKELETON,
}

// ToPb converts a ChannelEndpointRole to its protobuf value
func (x ChannelEndpointRole) ToPb() chprotobuf.PbEndpointRole {
	result, ok := channelEndpointRoleToPbEndpointRole[x]
	if !ok {
		result = chprotobuf.PbEndpointRole_UNKNOWN
	}
	return result
}

// PbToChannelEndpointRole returns a ChannelEndpointRole from its protobuf value
func PbToChannelEndpointRole(pbRole chprotobuf.PbEndpointRole) ChannelEndpointRole {
	result, ok := pbEndpointRoleToChannelEndpointRole[pbRole]
	if !ok {
		result = ChannelEndpointRoleUnknown
	}
	return result
}

// FromPb initializes a ChannelEndpointRole from its protobuf value
func (x *ChannelEndpointRole) FromPb(pbRole chprotobuf.PbEndpointRole) {
	*x = PbToChannelEndpointRole(pbRole)
}

// ChannelEndpointType describes the protocol used for a particular endpoint
type ChannelEndpointType string

const (
	// ChannelEndpointTypeUnknown is an unknown (uninitialized) endpoint type
	ChannelEndpointTypeUnknown ChannelEndpointType = "unknown"

	// ChannelEndpointTypeTCP is a TCP endpoint--either a host/port for Skeleton or
	//  a local bind address/port for Stub
	ChannelEndpointTypeTCP ChannelEndpointType = "tcp"

	// ChannelEndpointTypeUnix is a Unix Domain Socket (AKA local socket) endpoint, identified
	// by filesystem pathname, for either a Skeleton or Stub.
	ChannelEndpointTypeUnix ChannelEndpointType = "unix"

	// ChannelEndpointTypeSocks is a logical SOCKS server. Only meaningful for Skeleton. When
	// a connection request is received from the remote proxy (on behalf of the respective Stub),
	// it is connected to an internal SOCKS server.
	ChannelEndpointTypeSocks ChannelEndpointType = "socks"

	// ChannelEndpointTypeStdio is a preconnected virtual socket connected to its proxy process's stdin
	// and stdout. For an Stub, the connection is established and forwarded to the remote proxy
	// immediately after the remote proxy session is initialized. For a Skeleton, the connection is
	// considered active as soon as a connect request is received from the remote proxy service. This type of
	// endpoint can only be associated with the proxy client's end, can only be connected once,
	// and once that connection is closed, it can no longer be used or reconnected for the duration
	// of the session with the remote proxy. There can only be one Stdio endpoint defined on a given
	// proxy client.
	ChannelEndpointTypeStdio ChannelEndpointType = "stdio"

	// ChannelEndpointTypeLoop ChannelEndpointType is a virtual loopack socket, identified by an opaque
	// endpoint name. It is similar to a Unix domain socket in that it is identified by a unique
	// string name to which both a Caller and a Called Service ultimately bind. However, The name is only
	// resovable within a particular Chisel Proxy, and a Stub endpoint of this type can only be reached
	// by a Skeleton endpoint of this type on the same Chisel Proxy. Traffic on Channels of this type is
	// directly forwarded between the Stub and the Skeleton on the Chisel Proxy server, eliminating two
	// open os socket handles and two extra socket hops that would be required if ordinary sockets were used.
	ChannelEndpointTypeLoop ChannelEndpointType = "loop"
)

// ToPb converts a ChannelEndpointType to its protobuf value
func (x ChannelEndpointType) ToPb() string {
	return string(x)
}

// PbToChannelEndpointType returns a ChannelEndpointType from its protobuf value
func PbToChannelEndpointType(pbType string) ChannelEndpointType {
	return ChannelEndpointType(pbType)
}

// FromPb initializes a ChannelEndpointType from its protobuf value
func (x *ChannelEndpointType) FromPb(pbType string) {
	*x = PbToChannelEndpointType(pbType)
}

// ChannelEndpointDescriptor describes one end of a ChannelDescriptor
type ChannelEndpointDescriptor struct {
	// Which end of the tunnel pair this endpoint occupies (Stub or Skeleton)
	Role ChannelEndpointRole `json:"role"`

	// What type of endpoint is this (e.g., TCP, unix domain socket, stdio, etc...)
	Type ChannelEndpointType `json:"type"`

	// The "name" associated with the endpoint. This depends on role and type:
	//
	//     TYPE    ROLE        PATH
	//     TCP     Stub        <local-ipv4-bind-address>:<port> for listen
	//     TCP     Skeleton    <hostname>:<port> for connect
	//     Unix    Stub        <Filesystem path of domain socket> for listen
	//     Unix    Skeleton    <Filesystem path of domain socket> for connect
	//     SOCKS   Skeleton    nil
	//     Stdio   Stub        nil
	//     Stdio   Skeleton    nil
	//     Loop    Stub        <loop-endpoint-name> for listen
	//     Loop    Skeleton    <loop-endpoint-name> for connect
	Path string `json:"path"`
}

// ToPb converts a ChannelEndpointDescriptor to its protobuf value
func (d *ChannelEndpointDescriptor) ToPb() *chprotobuf.PbEndpointDescriptor {
	return &chprotobuf.PbEndpointDescriptor{
		Role: d.Role.ToPb(),
		Type: d.Type.ToPb(),
		Path: d.Path,
	}
}

// FromPb initializes a ChannelEndpointDescriptor from its protobuf value
func (d *ChannelEndpointDescriptor) FromPb(pb *chprotobuf.PbEndpointDescriptor) {
	d.Role.FromPb(pb.GetRole())
	d.Type.FromPb(pb.GetType())
	d.Path = pb.GetPath()
}

// PbToChannelEndpointDescriptor returns a ChannelEndpointDescriptor from its protobuf value
func PbToChannelEndpointDescriptor(pb *chprotobuf.PbEndpointDescriptor) *ChannelEndpointDescriptor {
	ced := &ChannelEndpointDescriptor{
		Role: PbToChannelEndpointRole(pb.GetRole()),
		Type: PbToChannelEndpointType(pb.GetType()),
		Path: pb.GetPath(),
	}
	return ced
}

// Validate a ChannelEndpointDescriptor
func (d ChannelEndpointDescriptor) Validate() error {
	if d.Role != ChannelEndpointRoleStub && d.Role != ChannelEndpointRoleSkeleton {
		return fmt.Errorf("%s: Unknown role type '%s'", d.String(), d.Role)
	}
	if d.Type == ChannelEndpointTypeTCP {
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
	} else if d.Type == ChannelEndpointTypeUnix {
		if d.Path == "" {
			return fmt.Errorf("%s: Unix domain socket endpoint requires a socket pathname", d.String())
		}
	} else if d.Type == ChannelEndpointTypeLoop {
		if d.Path == "" {
			return fmt.Errorf("%s: Loop endpoint requires a loop name", d.String())
		}
	} else if d.Type == ChannelEndpointTypeStdio {
		if d.Path != "" {
			return fmt.Errorf("%s: STDIO endpoint cannot have a path", d.String())
		}
	} else if d.Type == ChannelEndpointTypeSocks {
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

func (d ChannelEndpointDescriptor) String() string {
	typeName := string(d.Type)
	if typeName == "" {
		typeName = "unknown"
	}
	pathName := d.Path
	return "<" + typeName + ":" + pathName + ">"
}

// LongString converts a ChannelEndpointDescriptor to a long descriptive string
func (d ChannelEndpointDescriptor) LongString() string {
	roleName := string(d.Role)
	if roleName == "" {
		roleName = "unknown"
	}
	typeName := string(d.Type)
	if typeName == "" {
		typeName = "unknown"
	}

	return "ChannelEndpointDescriptor(role='" + roleName + "', type='" + typeName + "', path='" + d.Path + "')"
}

type bracketStack struct {
	runes []rune
	n     int
}

func (s *bracketStack) pushBracket(r rune) {
	s.runes = append(s.runes[:s.n], r)
	s.n++
}

func (s *bracketStack) popBracket() rune {
	var c rune
	if s.n > 0 {
		s.n--
		c = s.runes[s.n]
	}
	return c
}

func (s *bracketStack) isBalanced() bool {
	return s.n == 0
}

// SplitBracketedParts breaks a ":"-delimited channel descriptor string
// into its parts, respecting the following escaping mechanisms:
//
//    * Except as indicated below, the presence of '[' or '<' anywhere in a descriptor element causes all
//        characters up to a balanced closing bracket to be included as part of the parsed element.
//    '\:' will be a converted to a single ':' within an element but will not be recognized as a delimiter
//    '\\' will be converted to a single '\' within an element
//    '\<' Will be converted to a single '<' and will not be considered for bracket balancing
//    '\>' will be converted to a single '>' and will not be considered for bracket balancing
//    '\[' Will be converted to a single '[' and will not be considered for bracket balancing
//    '\]' will be converted to a single ']' and will not be considered for bracket balancing
func SplitBracketedParts(s string) ([]string, error) {
	bStack := &bracketStack{}

	var result []string
	partial := ""
	haveBackslash := false
	closeToOpen := map[rune]rune{
		'>': '<',
		']': '[',
	}

	flushPartial := func(final bool) error {
		if !bStack.isBalanced() {
			return fmt.Errorf("SplitChannelDescriptorParts: unmatched '%c' in descriptor '%s'", bStack.popBracket(), s)
		}
		if haveBackslash {
			return fmt.Errorf("SplitChannelDescriptorParts: descriptor ends in backslash: '%s'", s)
		}
		if !(final && len(partial) == 0 && len(result) == 0) {
			result = append(result, partial)
			partial = ""
		}
		return nil
	}

	for _, c := range s {
		if haveBackslash {
			partial += string(c)
			haveBackslash = false
		} else if c == '\\' {
			haveBackslash = true
		} else if c == '[' {
			partial += string(c)
			bStack.pushBracket('[')
		} else if c == '<' {
			partial += string(c)
			bStack.pushBracket('<')
		} else if c == '>' || c == ']' {
			if bStack.isBalanced() {
				return nil, fmt.Errorf("SplitChannelDescriptorParts: unmatched '%c' in descriptor '%s'", c, s)
			}
			actualOpen := bStack.popBracket()
			expectedOpen := closeToOpen[c]
			if actualOpen != expectedOpen {
				return nil, fmt.Errorf(
					"SplitChannelDescriptorParts: mismatched bracket types, "+
						"opened with '%c', closed with '%c' in descriptor '%s'", actualOpen, c, s)
			}
			partial += string(c)
		} else if c == ':' && bStack.isBalanced() {
			err := flushPartial(false)
			if err != nil {
				return nil, err
			}
		} else {
			partial += string(c)
		}
	}
	err := flushPartial(true)
	if err != nil {
		return nil, err
	}

	return result, nil
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

func isAngleBracketed(s string) bool {
	if len(s) < 2 || s[0] != '<' || s[len(s)-1] != '>' {
		return false
	}

	bStack := &bracketStack{}

	haveBackslash := false
	closePos := -1

	for i, c := range s {
		if haveBackslash {
			haveBackslash = false
		} else if c == '\\' {
			haveBackslash = true
		} else if c == '<' {
			bStack.pushBracket('<')
		} else if c == '>' || c == ']' {
			bStack.popBracket()
			if bStack.isBalanced() {
				closePos = i
				break
			}
		}
	}

	return closePos == len(s)-1
}

// StripAngleBrackets removes balanced leading and trailing '<' and '>' pair on a string, if they are present
func StripAngleBrackets(s string) string {
	if isAngleBracketed(s) {
		s = s[1 : len(s)-1]
	}
	return s
}

// ParseHostPort breaks a <hostname>:<port>, <hostname>, or <port> into a tuple.
//   <hostname> may contain balanced square or angle brackets, inside which ':'
//   characters are not considered as a delimiter. This allows for IPV6 host/port
//   specification such as [2001:0000:3238:DFE1:0063:0000:0000:FEFB]:80
//   In addition the entire path or the host, (but not the port) may be enclosed in
//   angle brackets, which will be stripped.
func ParseHostPort(path string, defaultHost string, defaultPort PortNumber) (string, PortNumber, error) {
	var port PortNumber
	var host string

	bpath := StripAngleBrackets(path)

	parts, err := SplitBracketedParts(bpath)
	if err != nil {
		return "", InvalidPortNumber, fmt.Errorf("Invalid TCP host/port string: %s: %s", err, path)
	}

	if len(parts) > 2 {
		return "", InvalidPortNumber, fmt.Errorf("Too many ':'-delimited parts in TCP host/port string: %s", path)
	} else if len(parts) == 1 {
		part := parts[0]
		port, err = ParsePortNumber(part)
		if err != nil {
			port = UnknownPortNumber
			host = StripAngleBrackets(part)
		}
	} else if len(parts) == 2 {
		host = StripAngleBrackets(parts[0])
		port, err = ParsePortNumber(parts[1])
		if err != nil {
			return "", InvalidPortNumber, fmt.Errorf("Invalid port in TCP host/port string: %s: %s", err, path)
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

// ParseNextChannelEndpointDescriptor parses the next ChannelEndpointDescriptor out of a presplit ":"-delimited string,
// returning the remainder of unparsed parts
func ParseNextChannelEndpointDescriptor(parts []string, role ChannelEndpointRole) (*ChannelEndpointDescriptor, []string, error) {
	s := strings.Join(parts, ":")
	if len(parts) <= 0 {
		return nil, parts, fmt.Errorf("Empty endpoint descriptor string not allowed: '%s'", s)
	}

	d := &ChannelEndpointDescriptor{Role: role}

	haveType := false
	havePath := false
	lastI := len(parts) - 1

	for i, p := range parts {
		sp := StripAngleBrackets(p)
		if sp == "stdio" {
			if haveType {
				break
			}
			d.Type = ChannelEndpointTypeStdio
			lastI = i
			break
		} else if sp == "socks" {
			if haveType {
				break
			}
			d.Type = ChannelEndpointTypeSocks
			lastI = i
			break
		} else if sp == "tcp" {
			if haveType {
				break
			}
			d.Type = ChannelEndpointTypeTCP
			haveType = true
		} else if sp == "unix" {
			if haveType {
				break
			}
			d.Type = ChannelEndpointTypeUnix
			haveType = true
		} else if sp == "loop" {
			if haveType {
				break
			}
			d.Type = ChannelEndpointTypeLoop
			haveType = true
		} else if IsPortNumberString(sp) {
			if haveType && d.Type != ChannelEndpointTypeTCP {
				break
			}
			d.Type = ChannelEndpointTypeTCP
			port, _ := ParsePortNumber(sp)
			d.Path = d.Path + ":" + port.String()
			lastI = i
			break
		} else {
			// Not an endpoint type name or a port number. Either
			//  1: An angle-bracketed endpoint specifier
			//  2: A path associated with an already parsed edpoint type
			//  3: A path with an implicit endpoint type (tcp or unix)
			if havePath {
				lastI = i
				break
			}
			if !haveType {
				spParts, err := SplitBracketedParts(sp)
				if err != nil {
					return nil, parts, fmt.Errorf("Invalid endpoint descriptor string '%s': '%s'", s, err)
				}
				if len(spParts) > 1 {
					// This must be an angle-bracketed standalone endpoint descriptor, so we will recurse
					d, err = ParseChannelEndpointDescriptor(sp, role)
					if err != nil {
						return nil, parts, err
					}
					lastI = i
					break
				}

				var spp0 string
				if len(spParts) > 0 {
					spp0 = StripAngleBrackets(spParts[0])
				}

				if spp0 == "stdio" {
					d.Type = ChannelEndpointTypeStdio
					lastI = i
					break
				}

				if spp0 == "socks" {
					d.Type = ChannelEndpointTypeSocks
					lastI = i
					break
				}

				if strings.HasPrefix(spp0, "/") || strings.HasPrefix(spp0, ".") {
					d.Type = ChannelEndpointTypeUnix
					d.Path = spp0
					lastI = i
					break
				}

				d.Type = ChannelEndpointTypeTCP
				d.Path = spp0
				haveType = true
				havePath = true
			} else {
				// a path to go with explicitly provided endpoint type
				if d.Type != ChannelEndpointTypeTCP {
					d.Path = StripAngleBrackets(sp)
					havePath = true
					lastI = i
					break
				}
				// A TCP path may contain a port number already in it, or
				// consist of nothing but a port
				host, port, err := ParseHostPort(sp, "", UnknownPortNumber)
				if err != nil {
					return nil, parts, fmt.Errorf("Invalid TCP host/port in endpoint descriptor string'%s': '%s'", s, err)
				}
				if port == UnknownPortNumber {
					d.Path = host
					havePath = true
				} else {
					d.Path = host + ":" + port.String()
					havePath = true
					lastI = i
					break
				}

			}
		}
	}

	if d.Type == ChannelEndpointTypeUnknown {
		return nil, parts, fmt.Errorf("Unable to determine type from endpoint descriptor string '%s'", s)
	}

	if (d.Type == ChannelEndpointTypeUnix || d.Type == ChannelEndpointTypeLoop) && d.Path == "" {
		return nil, parts, fmt.Errorf("Missing endpoint path in endpoint descriptor string '%s'", s)
	}

	// We allow unspecified path for TCP because it is implicitly determined from remote
	// endpoint in some cases

	return d, parts[lastI+1:], nil
}

// ParseChannelEndpointDescriptor parses a single standalone channel endpoint descriptor string
func ParseChannelEndpointDescriptor(s string, role ChannelEndpointRole) (*ChannelEndpointDescriptor, error) {
	parts, err := SplitBracketedParts(s)
	if err != nil {
		return nil, fmt.Errorf("Badly formed channel endpoint descriptor '%s': %s", s, err)
	}
	d, remaining, err := ParseNextChannelEndpointDescriptor(parts, role)
	if err != nil {
		return nil, err
	}
	if len(remaining) > 1 || (len(remaining) == 1 && remaining[0] != "") {
		return nil, fmt.Errorf("Too many parts in channel endpoint descriptor string '%s': '%s' + %v", s, d, remaining)
	}
	return d, nil
}
