FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api

FROM alpine:3.21

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /api .
COPY --from=builder /app/migrations/ ./migrations/
COPY --from=builder /app/seed/ ./seed/

EXPOSE 8080

CMD ["./api"]
