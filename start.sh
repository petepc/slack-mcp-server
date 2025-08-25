#!/bin/sh
# Start script for Railway deployment

set -e

# Start MCP server in background
echo "Starting MCP server..."
SLACK_MCP_HOST=127.0.0.1 SLACK_MCP_PORT=13080 ./mcp-server --transport sse &
MCP_PID=$!

# Give MCP server time to start
sleep 2

# Check if MCP server is running
if ! kill -0 $MCP_PID 2>/dev/null; then
    echo "Failed to start MCP server"
    exit 1
fi

echo "MCP server started with PID $MCP_PID"

# Start OAuth wrapper in foreground
echo "Starting OAuth wrapper on port ${PORT:-8080}..."
exec ./oauth-wrapper