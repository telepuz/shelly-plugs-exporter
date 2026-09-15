FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./...
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /exporter ./cmd/exporter/

FROM scratch
COPY --from=builder /exporter /exporter
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 9924
ENTRYPOINT ["/exporter"]
