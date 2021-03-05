package chshare

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/jpillora/chisel/chprotobuf"
)

// SessionConfigRequest describes a chisel proxy/client session configuration. It is
// sent from the client to the server during initialization
type SessionConfigRequest struct {
	Version            string
	ChannelDescriptors []*ChannelDescriptor
}

// ToPb converts a SessionConfigRequest to its protobuf value
func (c *SessionConfigRequest) ToPb() *chprotobuf.PbSessionConfigRequest {
	numChannels := len(c.ChannelDescriptors)
	pbcds := make([]*chprotobuf.PbChannelDescriptor, numChannels)
	for i, cd := range c.ChannelDescriptors {
		pbcds[i] = cd.ToPb()
	}
	return &chprotobuf.PbSessionConfigRequest{
		ClientVersion:      c.Version,
		ChannelDescriptors: pbcds,
	}
}

// FromPb initializes a SessionConfigRequest from its protobuf value
func (c *SessionConfigRequest) FromPb(pb *chprotobuf.PbSessionConfigRequest) {
	c.Version = pb.GetClientVersion()
	numChannels := len(pb.ChannelDescriptors)
	c.ChannelDescriptors = make([]*ChannelDescriptor, numChannels)
	for i, pbcd := range pb.ChannelDescriptors {
		c.ChannelDescriptors[i] = PbToChannelDescriptor(pbcd)
	}
}

// PbToSessionConfigRequest returns a SessionConfigRequest from its protobuf value
func PbToSessionConfigRequest(pb *chprotobuf.PbSessionConfigRequest) *SessionConfigRequest {
	numChannels := len(pb.ChannelDescriptors)
	cds := make([]*ChannelDescriptor, numChannels)
	for i, pbcd := range pb.ChannelDescriptors {
		cds[i] = PbToChannelDescriptor(pbcd)
	}
	return &SessionConfigRequest{
		Version:            pb.GetClientVersion(),
		ChannelDescriptors: cds,
	}
}

// Unmarshal unserializes a SessionConfigRequest from protobuf bytes
func (c *SessionConfigRequest) Unmarshal(b []byte) error {
	pbc := &chprotobuf.PbSessionConfigRequest{}
	err := proto.Unmarshal(b, pbc)
	if err != nil {
		return fmt.Errorf("Invalid protobuf data for SessionConfigRequest")
	}
	c.FromPb(pbc)
	return nil
}

// Marshal serializes a SessionConfigRequest to protobuf bytes
func (c *SessionConfigRequest) Marshal() ([]byte, error) {
	pbc := c.ToPb()
	return proto.Marshal(pbc)
}
