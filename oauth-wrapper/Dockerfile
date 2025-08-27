FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files from parent if they exist
COPY ../go.* ./
RUN go mod download 2>/dev/null || true

# Copy the wrapper source
COPY main.go .

# Initialize module if needed
RUN go mod init oauth-wrapper 2>/dev/null || true

# Build the application
RUN go build -o oauth-wrapper main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/oauth-wrapper .

EXPOSE 8080

CMD ["./oauth-wrapper"]