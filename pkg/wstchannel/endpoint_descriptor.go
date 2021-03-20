package wstchannel

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// ChannelEndpointRole defines whether an endpoint is acting as
// the Stub or the Skeleton for al ChannelEndpointPair
type ChannelEndpointRole string

const (
	// ChannelEndpointRoleUnknown is an unknown (uninitialized) endpoint role
	ChannelEndpointRoleUnknown ChannelEndpointRole = "unknown"

	// ChannelEndpointRoleStub is associated with an endpoint whose wstunnel
	// instance must listen for and accept new connections from local clients,
	// forward connection requests to the remote proxy where services
	// are locally available. An Stub endpoint may accept multiple
	// connection requests from local clients, resulting in multiple concurrent
	// tunnelled connections to the remote Skeleton endpoint proxy.
	ChannelEndpointRoleStub ChannelEndpointRole = "stub"

	// ChannelEndpointRoleSkeleton is associated with an endpoint whose wstunnel
	// instance must accept connection requests from the remote proxy
	// and actively reach out and connect to locally available services.
	// A Skeleton endpoint may accept multiple connection requests from the
	// remote proxy, resulting in multiple concurrent socket connections to
	// locally avaiable services
	ChannelEndpointRoleSkeleton ChannelEndpointRole = "skeleton"
)

// ChannelEndpointProtocol describes the protocol used for a particular endpoint. Each protocol
// has a factory that knows how to create an endpoint from an endpoint descriptor. The namespace
// is extensible so that new channel providers can be registered. Some well-known ones are
// enumerated here.
type ChannelEndpointProtocol string

const (
	// ChannelEndpointProtocolUnknown is an unknown (uninitialized) endpoint type
	ChannelEndpointProtocolUnknown ChannelEndpointProtocol = "unknown"

	// ChannelEndpointProtocolTCP is a TCP endpoint--either a host/port for Skeleton or
	//  a local bind address/port for Stub. Both IPv6 and IPv4 are supported.
	ChannelEndpointProtocolTCP ChannelEndpointProtocol = "tcp"

	// ChannelEndpointProtocolUnix is a Unix Domain Socket (AKA local socket) endpoint, identified
	// by filesystem pathname, for either a Skeleton or Stub.
	ChannelEndpointProtocolUnix ChannelEndpointProtocol = "unix"

	// ChannelEndpointProtocolSocks is a logical SOCKS server. Only meaningful for Skeleton. When
	// a connection request is received from the remote proxy (on behalf of the respective Stub),
	// it is connected to an internal SOCKS server.
	ChannelEndpointProtocolSocks ChannelEndpointProtocol = "socks"

	// ChannelEndpointProtocolStdio is a preconnected virtual socket connected to its proxy process's stdin
	// and stdout. For an Stub, the connection is established and forwarded to the remote proxy
	// immediately after the remote proxy session is initialized. For a Skeleton, the connection is
	// considered active as soon as a connect request is received from the remote proxy service. This type of
	// endpoint can only be associated with the proxy client's end, can only be connected once,
	// and once that connection is closed, it can no longer be used or reconnected for the duration
	// of the session with the remote proxy. There can only be one Stdio endpoint defined on a given
	// proxy client.
	ChannelEndpointProtocolStdio ChannelEndpointProtocol = "stdio"

	// ChannelEndpointProtocolLoop ChannelEndpointProtocol is a virtual loopack socket, identified by an opaque
	// endpoint name. It is similar to a Unix domain socket in that it is identified by a unique
	// string name to which both a Caller and a Called Service ultimately bind. However, The name is only
	// resovable within a particular Wstunnel Proxy, and a Stub endpoint of this type can only be reached
	// by a Skeleton endpoint of this type on the same Wstunnel Proxy. Traffic on Channels of this type is
	// directly forwarded between the Stub and the Skeleton on the Wstunnel Proxy server, eliminating two
	// open os socket handles and two extra socket hops that would be required if ordinary sockets were used.
	ChannelEndpointProtocolLoop ChannelEndpointProtocol = "loop"
)

type ChannelEndpointDescriptor interface {
	// Stringer converts the descriptor to a concise descriptive string; generally this will be derived from the endpoint path
	fmt.Stringer

	// LongString gets a complete descriptive string, which typically is the json of the descriptor
	LongString() string

	// GetRole returns the role of this endpoint, either "stub" or "skeleton". A stub is a listener
	// that receives connections from protocol-specific clients and forwards them through the proxy, A
	// skeleton is a dialer that receives connection requests from the proxy and forwards then to
	// protocol-specific sevices.
	GetRole() ChannelEndpointRole

	// GetType returns what type of endpoint is this (e.g., "tcp"", "unix"", "stdio", etc. This namespace is extensible
	// so that new custom channel providers can be added...)
	GetType() ChannelEndpointProtocol

	// GetParamsPath returns a path string that represents parameters for this channel endpoints descriptor, without the
	// ChannelEndpointProtocol prefix.  If json parameters were provided, then this
	// is just the compressed json string representation of the descriptor. If the endpoint does not have any
	// parameters, an empty string is be returned.
	GetParamsPath() string

	// GetParamsRaw returns the arbitrary undecoded raw JSON parameter value specific to the channel provider. This will
	// return nil if there are no parameters, or will unmarshall to one of:
	//       1. a single JSON string (unmarshalls to string) which is the params path.
	//       2. A JSON object (unmarshalls to map[string]interface{} that has provider-specific endpoint configuration data
	GetParamsRaw() json.RawMessage

	// GetParamsVar returns the generic decoded raw JSON parameter value specific to the channel provider. This will
	// be castable to one of:
	//       1. string, if the params are provided as an opaque string
	//       2. map[string]interface{}, if params are provided as a JSON object
	//       3. nil if the endpoint has no configuration parameters
	GetParamsVar() interface{}

	// GetParamsMap returns the decoded raw JSON parameter object specific to the channel provider, as a map from
	// string to a variant. This will be nil if the parameters were specified as a path string rather than a json object.
	GetParamsMap() map[string]interface{}
	
}

// RawChannelEndpointDescriptor describes one end of a ChannelDescriptor, in a generic serializable way. It is not able to
// actually create endpoints.
type RawChannelEndpointDescriptor struct {

	// ChannelEndpointRole is which end of the tunnel pair this endpoint occupies ("stub" or "skeleton"). A stub is a listener
	// that receives connections from protocol-specific clients and forwards them through the proxy, A
	// skeleton is a dialer that receives connection requests from the proxy and forwards then to
	// protocol-specific sevices.
	EpRole ChannelEndpointRole `json:"role"`

	// ChannelEndpointProtocol is what type of endpoint is this (e.g., "tcp"", "unix"", "stdio", etc. This namespace is extensible
	// so that new custom channel providers can be added...)
	EpType ChannelEndpointProtocol `json:"protocol"`

	// Version is an optional version string that helps specify how to interpret data.
	Version string `json:"version,omitempty",`

	// RawParams is the arbitrary undecoded raw JSON parameter value specific to the channel provider. This will
	// either unmarshall to:
	//       1. a single JSON string (unmarshalls to string) which is a ChannelParamsPath.
	//       2. A JSON object (unmarshalls to map[string]interface{} that has provider-specific endpoint configuration data
	//       3. a JSON null (unmarshals to nil) if the endpoint has no configuration parameters
	RawParams json.RawMessage `json:"params"`

	// varParams is the decoded raw JSON parameter value specific to the channel provider. This will
	// be castable to one of:
	//       1. ChannelParamsPath, if the params are provided as an opaque string
	//       2. map[string]interface{}, if params are provided as a JSON object
	//       3. nil if the endpoint has no configuration parameters
	varParams interface{} `json:"-"`

	// paramsPath is varParams.(string) if the params are represented as a path. It is an empty string if
	// there are no params. Otherwise it is the min-length JSON representation of paramsMap.
	paramsPath string `json:"-"`

	// paramsMap is the map representation of the of the provider-specific JSON parameter object, if parameters
	// were suplied as an object rather than a path, or nil if parameters were supplied as a string and the object
	// form is not known.
	paramsMap map[string]interface{} `json:"-"`

	// shortDescription is a terse summary that is derived from the content and cached
	shortDescription string `json:"-"`

	/*
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
	*/
}

// NewChannelEndpointDescriptorWithJson creates a ChannelEndpointDescriptor with parameters encoded
// in a json.RawMessage.
func NewChannelEndpointDescriptorWithJson(
	epRole ChannelEndpointRole,
	epType ChannelEndpointProtocol,
	version string,
	jsonParams json.RawMessage,
	paramsPath string, /* Optional; if not an empty string, will override default generated from json. */
) (ChannelEndpointDescriptor, error) {
	var varParams interface{}
	var dupJsonParams json.RawMessage
	var err error
	if jsonParams != nil {
		varParams, err = DecodeGenericJsonRawMessage(jsonParams)
		if err != nil {
			return nil, fmt.Errorf("Bad channel params JSON: %v", err)
		}
		dupJsonParams = make([]byte, len(jsonParams))
		copy(dupJsonParams, jsonParams)
	}

	var paramsPath2 string
	var paramsMap map[string]interface{}
	var ok bool

	if varParams != nil {
		if paramsPath2, ok = varParams.(string); ok {
			paramsMap = nil
		} else if paramsMap, ok = varParams.(map[string]interface{}); ok {
			paramsPath2, err = ToCompactJsonString(paramsMap)
			if err != nil {
				return nil, fmt.Errorf("Unable to remarshal channel params to JSON: %v", err)
			}
		} else {
			return nil, fmt.Errorf("Unsupported JSON value type for channel params: %T", varParams)
		}
	}

	if paramsPath == "" {
		paramsPath = paramsPath2
	}

	d := &RawChannelEndpointDescriptor{
		EpRole:           epRole,
		EpType:           epType,
		Version:          version,
		RawParams:        dupJsonParams,
		varParams:        varParams,
		paramsPath:       paramsPath,
		paramsMap:        paramsMap,
		shortDescription: "",
	}

	d.initShortDescription()

	return d, nil
}

// NewChannelEndpointDescriptorWithParamsPath creates a ChannelEndpointDescriptor with parameters encoded
// in a path parameter string.
// If decodeJsonPath is true and paramsPath starts with "{", then paramsPath is interpreted as a JSON object definition
// and the descriptor is initialized with JSON.
// If an error occurs, nb indicates a best guess at the byte offset where the error occurred
func NewChannelEndpointDescriptorWithParamsPath(
	epRole ChannelEndpointRole,
	epType ChannelEndpointProtocol,
	version string,
	paramsPath string,
	decodeJsonPath bool, // If true and paramsPath starts with '{', it will be decoded as json and used as a params object
) (d ChannelEndpointDescriptor, nb int, err error) {
	var rawParams json.RawMessage
	var err error

	if decodeJsonPath && len(paramsPath) > 0 && paramsPath[0] == '{' {
		rawParams, nb, err := UnmarshParseJsonValueInString(paramsPath)
		if err != nil {
			return nil, nb, fmt.Errorf("Bad JSON at char offset %d in channel params <%s>: %v", utf8.RuneCountInString(paramsPath[:nb]), paramsPath, err)
		}
		d, err := NewChannelEndpointDescriptorWithJson(registry, epRole, epType, version, rawParams, paramsPath)
		return d, err
	}

	if paramsPath != "" {
		rawParams, err = json.Marshal(paramsPath)
		if err != nil {
			return nil, fmt.Errorf("Unable to marshal channel param path to JSON string: %v", err)
		}
	}

	d := &RawChannelEndpointDescriptor{
		EpRole:           epRole,
		EpType:           epType,
		Version:          version,
		RawParams:        rawParams,
		varParams:        paramsPath,
		paramsPath:       paramsPath,
		paramsMap:        nil,
		shortDescription: "",
	}

	d.initShortDescription()

	return d, nil
}

func (d *RawChannelEndpointDescriptor) initShortDescription() {
	if d.paramsPath == "" {
		d.shortDescription = fmt.Sprintf("<Endpoint %s %s>", d.epRole, d.epType)
	} else {
		d.shortDescription = fmt.Sprintf("<Endpoint %s <%s:%s>>", d.epRole, d.epType, d.paramsPath)
	}
}

// String gets a concise descriptive string, which typically is the params path of the descriptor
func (d *RawChannelEndpointDescriptor) String() string {
	return d.shortDescription
}

// LongString gets a complete descriptive string, which typically is the json of the descriptor
func (d *RawChannelEndpointDescriptor) LongString() string {
	js, err := ToPrettyJsonString(d)
	if err != nil {
		return d.String()
	}
	return fmt.Sprint("Endpoint %s", js)
}

// ValidateLocal ensures that the descriptor is valid for local instantiation.  For generic remote descriptors,
// validation is minimal and strict checking is done by the remote proxy.  This allows channel providers on the
// remote proxy to be utilized without having a matching local implementation.
func (d *RawChannelEndpointDescriptor) ValidateLocal() error

// GetRole returns the role of this endpoint, either "stub" or "skeleton". A stub is a listener
// that receives connections from protocol-specific clients and forwards them through the proxy, A
// skeleton is a dialer that receives connection requests from the proxy and forwards then to
// protocol-specific sevices.
func (d *RawChannelEndpointDescriptor) GetRole() ChannelEndpointRole {
	return d.EpRole
}

// GetType returns what type of endpoint is this (e.g., "tcp"", "unix"", "stdio", etc. This namespace is extensible
// so that new custom channel providers can be added...)
func (d *RawChannelEndpointDescriptor) GetType() ChannelEndpointProtocol {
	return d.EpType
}

// GetParamsPath returns a path string that represents parameters for this channel endpoints descriptor, without the
// ChannelEndpointProtocol prefix.  If json parameters were provided, then this
// is just the compressed json string representation of the descriptor. If the endpoint does not have any
// parameters, an empty string is be returned.
func (d *RawChannelEndpointDescriptor) GetParamsPath() string {
	return d.paramsPath
}

// GetParamsRaw returns the arbitrary undecoded raw JSON parameter value specific to the channel provider. This will
// return nil if there are no parameters, or will unmarshall to one of:
//       1. a single JSON string (unmarshalls to string) which is the params path.
//       2. A JSON object (unmarshalls to map[string]interface{} that has provider-specific endpoint configuration data
func (d *RawChannelEndpointDescriptor) GetParamsRaw() json.RawMessage
{
	return d.RawParams
}

// GetParamsVar returns the generic decoded raw JSON parameter value specific to the channel provider. This will
// be castable to one of:
//       1. string, if the params are provided as an opaque string
//       2. map[string]interface{}, if params are provided as a JSON object
//       3. nil if the endpoint has no configuration parameters
func (d *RawChannelEndpointDescriptor) GetParamsVar() interface{} {
	return d.varParams
}

// GetParamsMap returns the decoded raw JSON parameter object specific to the channel provider, as a map from
// string to a variant. This will be nil if the parameters were specified as a path string rather than a json object.
func (d *RawChannelEndpointDescriptor) GetParamsMap() map[string]interface{} {
	return d.paramsMap
}
