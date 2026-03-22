# SPDX-License-Identifier: AGPL-3.0-or-later
FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /waitlist .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /waitlist /waitlist
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/waitlist"]
