# build stage
FROM golang:alpine AS build-env
LABEL maintainer="sammck@gmail.com"
RUN apk update
RUN apk add git
ENV CGO_ENABLED 0
ADD . /src
WORKDIR /src
RUN go build \
    -mod vendor \
    -ldflags "-X github.com/sammck-go/wstunnel/share.BuildVersion=$(git describe --abbrev=0 --tags)" \
    -o wstunnel
# container stage
FROM alpine
RUN apk update && apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build-env /src/wstunnel /app/wstunnel
ENTRYPOINT ["/app/wstunnel"]
