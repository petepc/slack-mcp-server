# syntax=docker/dockerfile:1.4
FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./

RUN --mount=type=cache,id=railway-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=railway-gobuild,target=/root/.cache/go-build \
    go build -o /bin/slack-mcp-server .

ENTRYPOINT ["/bin/slack-mcp-server"]
