FROM golang:1.26-alpine

RUN apk add --no-cache git && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
