module github.com/sammck-go/wstunnel

go 1.16

replace github.com/sammck-go/asyncobj v1.1.0 => ../asyncobj
replace github.com/sammck-go/logger v1.1.1 => ../logger

require (
	github.com/andrew-d/go-termutil v0.0.0-20150726205930-009166a695a2 // indirect
	github.com/armon/go-socks5 v0.0.0-20160902184237-e75332964ef5
	github.com/fsnotify/fsnotify v1.4.9
	github.com/golang/protobuf v1.4.3
	github.com/gorilla/websocket v1.4.2
	github.com/jpillora/ansi v1.0.2 // indirect
	github.com/jpillora/backoff v1.0.0
	github.com/jpillora/requestlog v1.0.0
	github.com/jpillora/sizestr v1.0.0
	github.com/prep/socketpair v0.0.0-20171228153254-c2c6a7f821c2
	github.com/sammck-go/asyncobj v1.1.0
	github.com/sammck-go/logger v1.1.1
	github.com/tomasen/realip v0.0.0-20180522021738-f0c99a92ddce // indirect
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
)
