package chshare

import (
	"fmt"

	"github.com/jpillora/chisel/chprotobuf"
)

// ChannelDescriptor describes a pair of endpoints, one on the client proxy and one
// on the server proxy, which together define a single tunnelled socket service.
type ChannelDescriptor struct {
	// Reverse: if true, indicates that this is a reverse-proxy tunnel--i.e., the
	// Stub is on the server proxy and the listener is on the client proxy.
	// If False, the answerer is on the client proxy and the Skeleton is on the
	// server proxy.
	Reverse bool

	// Stub is the endpoint that listens for and accepts local connections, and forwards
	// them to the remote proxy. Ordinarily the Stub is on the client proxy, but this
	// is flipped if Reverse==true.
	Stub *ChannelEndpointDescriptor

	// Skeleton is the endpoint that receives connect requests from the remote proxy
	// and forwards them to locally accessible network services. Ordinarily the
	// Skeleton is on the server proxy, but this is flipped if Reverse==true.
	Skeleton *ChannelEndpointDescriptor
}

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

	if (!d.Reverse && d.Skeleton.Type == ChannelEndpointTypeStdio) ||
		(d.Reverse && d.Stub.Type == ChannelEndpointTypeStdio) {
		return fmt.Errorf("%s: STDIO endpoint must be on client proxy side", d.String())
	}

	return nil
}

func (d ChannelDescriptor) String() string {
	reversePrefix := ""
	if d.Reverse {
		reversePrefix = "R:"
	}
	return reversePrefix + d.Stub.String() + ":" + d.Skeleton.String()
}

// LongString converts a ChannelDescriptor to a long descriptive string
func (d ChannelDescriptor) LongString() string {
	reverseStr := "false"
	if d.Reverse {
		reverseStr = "true"
	}

	return "ChannelDescriptor(reverse='" + reverseStr + "', stub=" + d.Stub.LongString() + ", skeleton=" + d.Skeleton.LongString() + ")"
}

// ToPb converts a ChannelEndpointDescriptor to its protobuf value
func (d *ChannelDescriptor) ToPb() *chprotobuf.PbChannelDescriptor {
	return &chprotobuf.PbChannelDescriptor{
		Reverse:            d.Reverse,
		StubDescriptor:     d.Stub.ToPb(),
		SkeletonDescriptor: d.Skeleton.ToPb(),
	}
}

// FromPb initializes a ChannelDescriptor from its protobuf value
func (d *ChannelDescriptor) FromPb(pb *chprotobuf.PbChannelDescriptor) {
	d.Reverse = pb.GetReverse()
	d.Stub = PbToChannelEndpointDescriptor(pb.GetStubDescriptor())
	d.Skeleton = PbToChannelEndpointDescriptor(pb.GetSkeletonDescriptor())
}

// PbToChannelDescriptor returns a ChannelDescriptor from its protobuf value
func PbToChannelDescriptor(pb *chprotobuf.PbChannelDescriptor) *ChannelDescriptor {
	cd := &ChannelDescriptor{
		Reverse:  pb.GetReverse(),
		Stub:     PbToChannelEndpointDescriptor(pb.GetStubDescriptor()),
		Skeleton: PbToChannelEndpointDescriptor(pb.GetSkeletonDescriptor()),
	}
	return cd
}

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

// ParseChannelDescriptor parses a string representing a ChannelDescriptor
func ParseChannelDescriptor(s string) (*ChannelDescriptor, error) {
	reverse := false
	parts, err := SplitBracketedParts(s)
	if err != nil {
		return nil, err
	}
	if len(parts) > 0 && parts[0] == "R" {
		reverse = true
		parts = parts[1:]
	}
	d := &ChannelDescriptor{
		Reverse: reverse,
	}

	var skeletonParts []string
	d.Stub, skeletonParts, err = ParseNextChannelEndpointDescriptor(parts, ChannelEndpointRoleStub)
	if err != nil {
		return nil, err
	}

	remParts := skeletonParts
	if len(skeletonParts) > 0 {
		d.Skeleton, remParts, err = ParseNextChannelEndpointDescriptor(skeletonParts, ChannelEndpointRoleSkeleton)
		if err != nil {
			return nil, err
		}
	} else {
		d.Skeleton = &ChannelEndpointDescriptor{Role: ChannelEndpointRoleSkeleton}
	}

	if len(remParts) != 0 {
		return nil, fmt.Errorf("Too many parts in channel descriptor string: '%s': %s + %s + %v", s, d.Stub, d.Skeleton, remParts)
	}

	if d.Stub.Type == ChannelEndpointTypeSocks {
		// Special case, allow *only* specifying socks, in which case move it from the Stub to the
		// Skeleton where it belongs
		if d.Skeleton.Type == ChannelEndpointTypeUnknown {
			tmp := d.Stub
			d.Skeleton = d.Stub
			d.Stub = tmp
			d.Stub.Role = ChannelEndpointRoleStub
			d.Skeleton.Role = ChannelEndpointRoleSkeleton
		}
		if d.Stub.Type == ChannelEndpointTypeUnknown {
			d.Stub.Type = ChannelEndpointTypeTCP
		}
	}

	if d.Stub.Type == ChannelEndpointTypeSocks {
		return nil, fmt.Errorf("SOCKS endpoints are only allowed on the skeleton side: '%s'", s)
	}

	if d.Skeleton.Type == ChannelEndpointTypeUnknown {
		d.Skeleton.Type = ChannelEndpointTypeTCP
	}

	stubBindAddr := ""
	stubPort := UnknownPortNumber
	skeletonHost := ""
	skeletonPort := UnknownPortNumber

	if d.Stub.Type == ChannelEndpointTypeTCP {
		if len(d.Stub.Path) > 0 {
			stubBindAddr, stubPort, err = ParseHostPort(d.Stub.Path, "", UnknownPortNumber)
			if err != nil {
				return nil, err
			}
		}
	}

	if d.Skeleton.Type == ChannelEndpointTypeTCP {
		if len(d.Skeleton.Path) > 0 {
			skeletonHost, skeletonPort, err = ParseHostPort(d.Skeleton.Path, "", UnknownPortNumber)
			if err != nil {
				return nil, err
			}
		}
	}

	if d.Stub.Type == ChannelEndpointTypeTCP && stubBindAddr == "" {
		if d.Skeleton.Type == ChannelEndpointTypeSocks {
			stubBindAddr = "127.0.0.1"
		} else {
			stubBindAddr = "0.0.0.0"
		}
	}

	if d.Stub.Type == ChannelEndpointTypeTCP && stubPort == UnknownPortNumber {
		if d.Skeleton.Type == ChannelEndpointTypeSocks {
			stubPort = PortNumber(1080)
		} else if skeletonPort != UnknownPortNumber {
			stubPort = skeletonPort
		}
	}

	if d.Skeleton.Type == ChannelEndpointTypeTCP && skeletonPort == UnknownPortNumber {
		if stubPort != UnknownPortNumber {
			skeletonPort = stubPort
		}
	}

	if d.Stub.Type == ChannelEndpointTypeTCP {
		if stubBindAddr == "" {
			return nil, fmt.Errorf("Unable to determine stub bind address in channel descriptor string: '%s'", s)
		}
		if stubPort == UnknownPortNumber {
			return nil, fmt.Errorf("Unable to determine stub port number in channel descriptor string: '%s'", s)
		}

		d.Stub.Path = stubBindAddr + ":" + stubPort.String()
	}

	if d.Skeleton.Type == ChannelEndpointTypeTCP {
		if skeletonHost == "" {
			skeletonHost = "localhost"
		}
		if skeletonPort == UnknownPortNumber {
			return nil, fmt.Errorf("Unable to determine skeleton port number in channel descriptor string: '%s'", s)
		}
		d.Skeleton.Path = skeletonHost + ":" + skeletonPort.String()
	}

	if (d.Stub.Type == ChannelEndpointTypeStdio && d.Reverse) || (d.Skeleton.Type == ChannelEndpointTypeStdio && !d.Reverse) {
		return nil, fmt.Errorf("Stdio endpoints are only allowed on the client proxy side: '%s'", s)
	}

	if d.Skeleton.Type == ChannelEndpointTypeUnknown {
		return nil, fmt.Errorf("Unable to determine skeleton endpoint type: '%s'", s)
	}

	err = d.Validate()
	if err != nil {
		return nil, err
	}

	return d, nil
}
