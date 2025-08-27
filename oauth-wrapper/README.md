# OAuth Wrapper for Slack MCP Server

This OAuth wrapper service bridges Claude Teams' OAuth requirements with the Slack MCP server, allowing you to use the Slack MCP server as a Claude Teams connector.

## How It Works

The wrapper provides:
1. OAuth 2.0 authorization server endpoints that Claude Teams expects
2. Client registration for Claude Teams
3. Token management and validation
4. Proxying of authenticated requests to the underlying MCP server

## Setup Instructions

### 1. Prerequisites

- Slack User OAuth token (`xoxp-...`) already configured
- Go 1.19+ installed (to run the wrapper)
- The main Slack MCP server

### 2. Environment Variables

Set these environment variables:

```bash
# Required - Your Slack OAuth token
export SLACK_MCP_XOXP_TOKEN="xoxp-your-token-here"

# Optional - OAuth wrapper configuration
export OAUTH_WRAPPER_PORT="8080"                    # Port for OAuth wrapper (default: 8080)
export OAUTH_WRAPPER_PUBLIC_URL="https://your-domain.com"  # Public URL where wrapper is accessible

# Optional - MCP server configuration (if different from defaults)
export SLACK_MCP_HOST="127.0.0.1"                   # MCP server host (default: 127.0.0.1)
export SLACK_MCP_PORT="13080"                       # MCP server port (default: 13080)

# Optional - If you want additional security for the SSE endpoint
export SLACK_MCP_SSE_API_KEY="your-api-key"         # API key for SSE transport
```

### 3. Running the Services

#### Step 1: Start the MCP Server

First, start the original Slack MCP server:

```bash
# From the slack-mcp-server root directory
go run mcp/mcp-server.go --transport sse
```

#### Step 2: Start the OAuth Wrapper

In a new terminal:

```bash
# From the oauth-wrapper directory
go run main.go
```

The wrapper will start on port 8080 (or your configured port) and provide:
- `/.well-known/oauth-authorization-server` - OAuth metadata
- `/register` - Client registration endpoint
- `/authorize` - Authorization endpoint
- `/token` - Token exchange endpoint
- `/sse` - Proxied SSE endpoint to MCP server

### 4. Configure Claude Teams

1. In Claude Teams, add a new connector
2. Use your OAuth wrapper's public URL as the connector URL
3. Claude Teams will:
   - Fetch OAuth metadata from `/.well-known/oauth-authorization-server`
   - Register as a client via `/register`
   - Redirect users through the OAuth flow
   - Exchange codes for tokens
   - Use the token to access the MCP server via `/sse`

### 5. Production Deployment

For production use:

1. **Use HTTPS**: Deploy behind a reverse proxy (nginx, Caddy) with SSL certificates
2. **Set Public URL**: Configure `OAUTH_WRAPPER_PUBLIC_URL` to your public HTTPS URL
3. **Secure the Services**: 
   - Run both services as systemd services or in containers
   - Use a process manager for reliability
   - Set up proper logging and monitoring

#### Example nginx configuration:

```nginx
server {
    listen 443 ssl;
    server_name your-slack-connector.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # SSE specific settings
        proxy_set_header Connection '';
        proxy_http_version 1.1;
        chunked_transfer_encoding off;
        proxy_buffering off;
        proxy_cache off;
    }
}
```

#### Example systemd service:

```ini
[Unit]
Description=Slack OAuth Wrapper
After=network.target

[Service]
Type=simple
User=youruser
WorkingDirectory=/path/to/slack-mcp-server/oauth-wrapper
Environment="SLACK_MCP_XOXP_TOKEN=xoxp-your-token"
Environment="OAUTH_WRAPPER_PUBLIC_URL=https://your-domain.com"
ExecStart=/usr/local/bin/go run main.go
Restart=always

[Install]
WantedBy=multi-user.target
```

## Architecture

```
Claude Teams
    ↓
[OAuth Wrapper]
    ├── OAuth 2.0 endpoints
    ├── Client management
    ├── Token validation
    └── Proxy to →
                    [MCP Server]
                        ├── SSE transport
                        └── Slack API integration
```

## Security Notes

1. **Token Storage**: Currently stores tokens in memory. For production, consider using Redis or a database
2. **HTTPS Required**: Always use HTTPS in production to protect tokens in transit
3. **Token Expiry**: Access tokens expire after 24 hours by default
4. **Client Secrets**: Generated cryptographically secure random strings

## Troubleshooting

1. **404 Errors**: Ensure the wrapper is running and accessible at the configured URL
2. **Authentication Failures**: Check that `SLACK_MCP_XOXP_TOKEN` is set correctly
3. **Connection Issues**: Verify both the MCP server and wrapper are running
4. **SSL Issues**: Ensure proper SSL certificates are configured for HTTPS

## Testing

Test the OAuth endpoints:

```bash
# Check OAuth metadata
curl https://your-domain.com/.well-known/oauth-authorization-server

# Test health endpoint
curl https://your-domain.com/health
```