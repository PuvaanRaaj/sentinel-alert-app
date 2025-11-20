# =========================
# Build stage
# =========================
FROM golang:1.22-alpine AS builder

# Install build tools (if needed later)
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Only copy go.mod first to leverage Docker layer caching
COPY go.mod ./
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

USER appuser

EXPOSE 8080

ENV PORT=8080

CMD ["./incident-viewer"]
