FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o wikicollab-backend .

# Minimal runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/wikicollab-backend .

# Migrations directory — populated via Docker Compose volume or build arg
# The MIGRATIONS_DIR env var tells the backend where to find migrations.
ENV MIGRATIONS_DIR=/migrations

EXPOSE 8080

CMD ["./wikicollab-backend"]
