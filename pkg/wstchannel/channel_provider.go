package wstchannel

import (
	"encoding/json"
)

// ChannelProvider is an interface for a pluggable subsystem that can parse protocol-specific serialized ChannelEndpointDescriptor
// objects, and through them can instantiate protocol-specific stubs and/or skeletons
type ChannelProvider interface {
	AsyncShutdowner

	// OnRegister is called by the registry when the ChannelEndpointProvider becomes registered with a specific type name.
	// The same provider may be registered multiple times with different type names. The first name should be used for
	// logging/reporting purposes
	OnRegister(registry ChannelProviderRegistry, t ChannelEndpointProtocol)

	// UnmarshalEndpointJson takes the generic contents of a RawChannelEndpointDescriptor and turns them into a
	// functional ChannelEndpointDescriptor that is able to instantiate protocol-specific ChannelEndpoint objects.
	// This method should validate the consistency of the parameters and support for the requested role and protocol
	// version.
	// If version is "", then no version is specified.
	// "role" will be either "stub" for a listener endpoint that will be used to forward protocol-specific connections through a proxy,
	// or "skeleton" for a dialer object that will receive forwarded connection requests from the proxy and forward them to
	// protocol-specific services.
	UnmarshalEndpointJson(raw json.RawMessage, role ChannelEndpointRole, version string) (ChannelEndpointDescriptor, error)

	// UnmarshaEndpointPath takes a protocol-specific endpoint path string and turns it into a
	// functional ChannelEndpointDescriptor that is able to instantiate protocol-specific ChannelEndpoint objects.
	// This method should validate the consistency of the path and support for the requested role and protocol
	// version.
	// "role" will be either "stub" for a listener endpoint that will be used to forward protocol-specific connections through a proxy,
	// or "skeleton" for a dialer object that will receive forwarded connection requests from the proxy and forward them to
	// protocol-specific services.
	// The path does not include the ChannelEndpointProtocol prefix or its ":" delimeter.
	// The builtin "shorthand" notations for well-known endpoint types has already been expanded to a standard form in path.
	//
	// Some example paths:
	//
	//  tcp:
	//    127.0.0.1:3000
	//    foobar.com:3000
	//
	//  unix:
	//    /var/run/mysocket.sock
	//
	//  stdio:
	//    <an empty string>)
	//
	//  loop:
	//    /my/name/space/loop-name
	//
	UnmarshalEndpointPath(path string, role ChannelEndpointRole) (ChannelEndpointDescriptor, error)
}
