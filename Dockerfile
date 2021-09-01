FROM golang:alpine AS builder

MAINTAINER Chris Snell <chris.snell@gmail.com>

RUN mkdir -p /go/src/github.com/chrissnell/gopherwx/
WORKDIR /go/src/github.com/chrissnell/gopherwx/

RUN apk update \
&& apk add --no-cache git \
build-base

COPY . .

RUN go get -u ./... \
&& go install

FROM alpine:latest

COPY --from=builder /go/bin/gopherwx /usr/bin/gopherwx
COPY --from=builder /go/src/github.com/chrissnell/gopherwx/entrypoint.sh /entrypoint.sh

VOLUME ["/config"]

CMD ["/entrypoint.sh"]
