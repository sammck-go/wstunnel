package wstchannel

type ChannelProviderRegistry interface {
	AsyncShutdowner
	Register(epType ChannelEndpointProtocol, provider ChannelProvider) error
}
