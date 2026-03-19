# SPDX-License-Identifier: AGPL-3.0-or-later
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go mod tidy && CGO_ENABLED=0 go build -o /waitlist .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /waitlist /waitlist
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/waitlist"]
