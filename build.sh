#!/bin/bash
go build \
    -ldflags "-X github.com/sammck-go/wstunnel/share.BuildVersion=$(git describe --abbrev=0 --tags)" \
    -o wstunnel
