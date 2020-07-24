# Build stage
FROM golang:1.13-alpine AS build

WORKDIR /go/src/github.com/dnsge/docker-event-notifier

ADD go.mod .
ADD go.sum .
RUN go mod download

ADD . .
RUN go build -o /go/bin/github.com/dnsge/docker-event-notifier

# Final stage
FROM alpine

WORKDIR /app
COPY --from=build /go/bin/github.com/dnsge/docker-event-notifier /app/

ENTRYPOINT ["./docker-event-notifier"]
