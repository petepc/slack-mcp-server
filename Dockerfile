FROM golang:1.24.4-alpine AS build

ENV CGO_ENABLED=0

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /go/bin/mcp-server ./cmd/slack-mcp-server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates curl
COPY --from=build /go/bin/mcp-server /usr/local/bin/mcp-server

ENV SLACK_MCP_HOST=0.0.0.0
ENV SLACK_MCP_PORT=3001
EXPOSE 3001
CMD ["mcp-server", "--transport", "sse"]
