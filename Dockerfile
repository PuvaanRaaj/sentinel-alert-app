# =========================
# Build stage
# =========================
FROM golang:1.22-alpine AS builder

# Install build tools (if needed later)
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy go.mod and go.sum (if exists) to leverage Docker layer caching
COPY go.mod go.sum* ./
RUN go mod download

# Now copy the rest of the source
COPY . .

# Build a static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o incident-viewer main.go

# =========================
# Runtime stage
# =========================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates && \
    adduser -D appuser

WORKDIR /home/appuser

# Copy the compiled binary from builder
COPY --from=builder /app/incident-viewer .

# Copy web assets (templates and static files)
COPY --from=builder /app/web ./web

# Change ownership to appuser
USER root
RUN chown -R appuser:appuser /home/appuser
USER appuser

EXPOSE 8080

ENV PORT=8080

CMD ["./incident-viewer"]
