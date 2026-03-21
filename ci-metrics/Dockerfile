# SPDX-License-Identifier: AGPL-3.0-or-later
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /ci-metrics .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /ci-metrics /ci-metrics
USER 65534:65534
EXPOSE 8081
ENTRYPOINT ["/ci-metrics"]
