FROM golang:alpine AS builder

MAINTAINER Chris Snell <chris.snell@gmail.com>

RUN mkdir -p /go/src/github.com/chrissnell/gopherwx/
WORKDIR /go/src/github.com/chrissnell/gopherwx/

RUN apk update && apk add --no-cache git 

COPY . .

RUN go mod tidy && go mod vendor
RUN CGO_ENABLED=0 go build


FROM alpine:latest

COPY --from=builder /go/src/github.com/chrissnell/gopherwx/gopherwx /gopherwx
COPY --from=builder /go/src/github.com/chrissnell/gopherwx/entrypoint.sh /entrypoint.sh

VOLUME ["/config"]

CMD ["/entrypoint.sh"]
