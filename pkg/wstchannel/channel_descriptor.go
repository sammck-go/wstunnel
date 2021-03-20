package wstchannel

import (
	"fmt"
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

	if (!d.Reverse && d.Skeleton.Type == ChannelEndpointProtocolStdio) ||
		(d.Reverse && d.Stub.Type == ChannelEndpointProtocolStdio) {
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
