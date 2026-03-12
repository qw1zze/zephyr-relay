# ── builder ──────────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/relay ./cmd/server

# ── runtime ───────────────────────────────────────────────────────────────────
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app
COPY --from=builder /app/relay .

EXPOSE 8081

CMD ["/app/relay"]
