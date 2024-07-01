FROM golang:alpine AS build-env

ENV GOPATH /go
WORKDIR /go/src

RUN go env -w GOCACHE=/tmp/go-cache
RUN go env -w GOMODCACHE=/tmp/gomod-cache

COPY ./go.* ./

WORKDIR /go/src/genezio-build
RUN --mount=type=cache,target=/tmp/gomod-cache go mod download

COPY . /go/src/genezio-build

RUN --mount=type=cache,target=/tmp/gomod-cache --mount=type=cache,target=/tmp/go-cache go build -o genezio-build ./cmd/main.go

FROM alpine
WORKDIR /

COPY --from=build-env /go/src/genezio-build/genezio-build /

# Mutually exclusive with the above line
ENTRYPOINT [ "./genezio-build" ]