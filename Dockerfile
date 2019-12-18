FROM golang:1.13.5-alpine3.10 AS build

WORKDIR /go/src

COPY go.mod go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ../bin

FROM scratch

COPY --from=build /go/bin/* /

ENTRYPOINT ["/namespacenodeselector"]
