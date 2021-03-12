// Package wstchannel provides abstract, extensible, interface-based manipulation of proxy
// endpoints and channels.
//
// To distinguish between the various entities in a tunneled
// network environment, we adopt the vocabulary of distributed object
// communication, in particular "Stub" for the proxy's channel
// of a network service, and "Skeleton" for the proxy's impersonation
// of a network client.
//   (see https://en.wikipedia.org/wiki/Distributed_object_communication).
//
// A "Wstunnel Proxy" is an instance of the proxy code in a single process, running as either a
// "Wstunnel Proxy Client" or a "Wstunnel Proxy Server". A "Wstunnel Proxy Session" consists
// of a single Wstunnel Proxy Client and a single Wstunnel Proxy Server, communicating with
// one another over a single protocol connection (typically aTCP connection using a encrypted
// SSH protocol over WebSockets.)
// A Wstunnel Proxy Client participates in exactly one Wstunnel Proxy Session, but a Wstunnel Proxy
// Server may have many concurrent Wstunnel Proxy Sessions with different Wstunnel Proxy Clients.
// All traffic through a Wstunnel Proxy Session, including proxied network service
// traffic, is encrypted with the SSH protocol. Provided the private key of the Wstunnel Proxy
// Server is kept private, and the public key fingerprint of the Wstunnel Proxy Server is known
// in advance by all Wstunnel Proxy Clients, there is no need for additional encryption of
// proxied traffic (it may still be desirable to encrypt proxied traffic between Wstunnel Proxies and
// the applications that connect to the proxy).
//
// A Local Wstunnel Proxy is the Wstunnel Proxy that is directly network-reachable in the context
// of a particular discussion. It may be either a Wstunnel Proxy Client or a Wstunnel Proxy Server.
//
// A Remote Wstunnel Proxy is the other Wstunnel Proxy in the same Wstunnel Proxy Session as a
// given Local Wstunnel Proxy.
//
// A "Caller" is an originating application that wishes to connect to a logical
// network "Called Service" that can not be reached locally, but which must be reached
// through the Wstunnel Proxy Session. Typically this is a TCP client, though other protocols
// are supported.
//
// A "Called Service" is a logical network service that needs to be accessed by
// "Callers" that cannot reach it locally, but must reach it through a Wstunnel Proxy Session.
// Typically this is a TCP service listening at a particular host/port, though other
// protocols are supported.
//
// A ChannelEndpoint is a logical network endpoint on a Local Wstunnel Proxy that
// is paired with another ChannelEndpoint on the Remote Wstunnel Proxy.
// A ChannelEndpoint is either a "Stub", meaning that it impersonates a remote Called Service
// and listens for and accepts connection requests from local Callers and forwards them
// to the peered ChannelEndpoint on the Remote Wstunnel Proxy, or a Skeleton, meaning that it responds to
// connection requests from the Remote Wstunnel Proxy and impersonates the remote Caller
// by connecting to locally available Called Services.
//
// A Stub looks like a Called Service to a Caller, and a Skeleton looks like
// a Caller to a Called Service.
//
// We will refer to the combination of a Stub ChannelEndpoint and its peered remote
// Skeleton ChannelEndpoint as a "Channel Endpoint Pair". Since it is a distributed
// entity, a Channel Endpoint Pair has no direct representation in code. A Channel Endpoint
// Pair in which the Stub ChannelEndpoint is on the Wstunnel Proxy Client is referred to
// as operating in "forward proxy mode". A Channel Endpoint Pair in which the Stub
// ChannelEndpoint is on the Wstunnel Proxy Server is referred to as operating in
// "reverse proxy mode".
//
// Just as a socket service listener can accept and service multiple concurrent
// connections from clients, a Channel Endpoint Pair can accept and service multiple
// concurrent connections from clients at the Stub side, and proxy them to multiple
// concurrent service connections at the Skeleton side. Each individual proxied
// connection of this type is called a Channel. Traffic on each Channel is completely
// independent of and asynchronous to traffic on other Channels. all Channel
// traffic is multiplexed through a Wstunnel Proxy Session on the single TCP connection
// used by the Wstunnel Proxy Session; flow control is employed to ensure that a delay
// in reading from one Channel has no effect on the availability of other channels or on
// traffic in the opposite direction on the same channel.
//
// A ChannelEndpoint is described in a serializable form by a ChannelEndpointDescriptor.
//
// A Channel Endpoint Pair is described in serialized form by a ChannelDescriptor.
//
// One type of ChannelEndpoint that deserves special mention is a "Loop" ChannelEndpoint.
// A Loop Stub ChannelEndpoint operates very much like a Unix Domain Socket Stub (it
// listens on a specified name in a string-based namespace), except that it can only
// accept connections from Loop Skeleton ChannelEndpoints on the Same Wstunnel Proxy (it has
// no visibility outside of the Wstunnel Proxy). The advantage of Loop Channels is that
// traffic is directly forwarded between a Caller's Channel and a Called service's
// Channel, when both the Caller and the Called Service are reachable only through
// a Wstunnel Proxy Session. This effectively saves two network hops for traffic
// in both directions (writing to a unix domain socket and reading back from the
// same unix domain socket)
//
//                           +-----------------------------+             +--------------------------------+
//                           |     Wstunnel Proxy Client     |    Wstunnel   |     Wstunnel Proxy Server        |
//                           |                             |   Session   |                                |
//                           |                             | ==========> |                                |
//   +----------------+      |    +-------------------|    |             |    +--------------------+      |      +----------------+
//   | Client-side    |      |    |  forward-proxy    |    |             |    |  forward-proxy     |      |      | Server-side    |
//   | Caller App     | =====|==> |  Stub             | :::|:::::::::::::|::> |  Skeleton          | =====|====> | Called Service |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |      |      |                |
//   +----------------+      |    +-------------------+    |             |    +--------------------+      |      +----------------+
//                           |                             |             |                                |
//   +----------------+      |    +-------------------|    |             |    +--------------------+      |      +----------------+
//   | Client-side    |      |    |  reverse-proxy    |    |             |    |  reverse-proxy     |      |      | Server-side    |
//   | Called Service | <====|=== |  Skeleton         | <::|:::::::::::::|::: |  Stub              | <====|===== | Caller App     |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |      |      |                |
//   +----------------+      |    +-------------------+    |             |    +--------------------+      |      +----------------+
//                           |                             |             |                                |
//   +----------------+      |    +-------------------|    |             |    +--------------------+      |
//   | Client-side    |      |    |  forward-proxy    |    |             |    |  forward-proxy     |      |
//   | Caller App     | =====|==> |  Stub             | :::|:::::::::::::|::> |  Skeleton "Loop"   | ===\ |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |    | |
//   +----------------+      |    +-------------------+    |             |    +--------------------+    | |
//                           |                             |             |                              | |
//                           +-----------------------------|             |                              | |
//                                                                       |                              | |
//                           +-----------------------------+             |                              | |
//                           |     Wstunnel Proxy Client     |    Wstunnel   |                              | |
//                           |                             |   Session   |                              | |
//                           |                             | ==========> |                              | |
//   +----------------+      |    +-------------------|    |             |    +--------------------+    | |
//   | Client-side    |      |    |  reverse-proxy    |    |             |    |  reverse-proxy     |    | |
//   | Called Service | <====|=== |  Skeleton         | <::|:::::::::::::|::: |  Stub "Loop"       | <==/ |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |      |
//   +----------------+      |    +-------------------+    |             |    +--------------------+      |
//                           |                             |             |                                |
//                           +-----------------------------+             +--------------------------------+
//
package wstchannel
