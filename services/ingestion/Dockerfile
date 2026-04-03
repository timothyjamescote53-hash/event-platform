FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY go.mod ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /ingestion-service ./cmd/main.go

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /ingestion-service /ingestion-service
EXPOSE 8080
ENTRYPOINT ["/ingestion-service"]
# multistage
# multistage
# multistage
# builder
# scratch
# go sum removed
# builder
# scratch
# go sum removed
# builder
# scratch
