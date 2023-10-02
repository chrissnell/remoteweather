FROM golang:alpine AS builder

MAINTAINER Chris Snell <chris.snell@gmail.com>

RUN mkdir -p /go/src/github.com/chrissnell/gopherwx/
WORKDIR /go/src/github.com/chrissnell/gopherwx/

RUN apk update && apk add --no-cache git protobuf protobuf-dev

COPY . .

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
RUN ln -s /go/bin/protoc-gen-go /usr/bin/protoc-gen-go

RUN /usr/bin/protoc --go_out=. \
        --go_opt=paths=source_relative \
        --go-grpc_out=. \
        --go-grpc_opt=paths=source_relative \
        protobuf/gopherwx.proto

RUN go mod tidy && go mod vendor
RUN CGO_ENABLED=0 go build

FROM alpine:latest

COPY --from=builder /go/src/github.com/chrissnell/gopherwx/gopherwx /gopherwx
COPY --from=builder /go/src/github.com/chrissnell/gopherwx/entrypoint.sh /entrypoint.sh

VOLUME ["/config"]

CMD ["/entrypoint.sh"]
