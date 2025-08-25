# Deploying to Runway

This guide explains how to deploy the Slack MCP OAuth wrapper on Runway for use with Claude Teams.

## Prerequisites

1. A Runway account and project
2. Your Slack OAuth token (`xoxp-...`)
3. Runway CLI installed (optional)

## Deployment Steps

### 1. Set Environment Variables in Runway

In your Runway project settings, add these environment variables:

```bash
SLACK_MCP_XOXP_TOKEN=xoxp-your-slack-token-here
SLACK_MCP_SSE_API_KEY=generate-a-secure-random-key  # Optional but recommended
```

### 2. Deploy to Runway

#### Option A: Using Runway CLI

```bash
runway deploy
```

#### Option B: Using Git Push

1. Connect your repository to Runway
2. Push to your deployment branch:

```bash
git add .
git commit -m "Add OAuth wrapper for Claude Teams"
git push origin main  # or your configured branch
```

### 3. Get Your Runway App URL

After deployment, Runway will provide you with a URL like:
```
https://your-app-name.runway.app
```

### 4. Configure Claude Teams

1. In Claude Teams, go to Connectors
2. Add a new connector
3. Use your Runway app URL: `https://your-app-name.runway.app`
4. Claude Teams will automatically:
   - Discover OAuth endpoints
   - Register as a client
   - Handle the authorization flow

## How It Works on Runway

The deployment includes two services:

1. **OAuth Wrapper** (Public-facing on port 8080)
   - Handles OAuth flow for Claude Teams
   - Validates and manages tokens
   - Proxies authenticated requests

2. **MCP Server** (Internal service)
   - Runs the actual Slack MCP integration
   - Only accessible from the OAuth wrapper
   - Uses your Slack token to access Slack APIs

## Architecture on Runway

```
Internet → [Runway Load Balancer]
              ↓
        [OAuth Wrapper]
              ↓
        [MCP Server] → Slack API
```

## Configuration Details

### Runway.yml Configuration

The `runway.yml` file configures:
- Two services (oauth-wrapper and mcp-server)
- Environment variable passthrough
- Health checks
- Service dependencies

### Automatic Environment Variables

Runway automatically sets:
- `PORT` - The port your web service should listen on
- `RUNWAY_APP_URL` - Your app's public URL

The OAuth wrapper automatically uses these when available.

## Monitoring

Check your deployment status:

```bash
# View logs
runway logs

# Check service health
curl https://your-app-name.runway.app/health

# Check OAuth metadata
curl https://your-app-name.runway.app/.well-known/oauth-authorization-server
```

## Troubleshooting

### Service Won't Start

1. Check that `SLACK_MCP_XOXP_TOKEN` is set in Runway environment variables
2. View logs: `runway logs`

### Claude Teams Can't Connect

1. Ensure your Runway app is publicly accessible
2. Check OAuth metadata endpoint is responding:
   ```bash
   curl https://your-app-name.runway.app/.well-known/oauth-authorization-server
   ```

### Authentication Errors

1. Verify your Slack token is valid
2. Check both services are running: `runway status`
3. Review logs for specific errors: `runway logs`

## Security Considerations

1. **Use HTTPS**: Runway provides HTTPS by default
2. **Set SSE API Key**: Add `SLACK_MCP_SSE_API_KEY` for internal service communication
3. **Token Security**: Never commit tokens to git; use Runway's environment variables

## Scaling

Runway automatically handles:
- Load balancing
- Auto-scaling based on traffic
- Health checks and restarts
- SSL/TLS termination

For high traffic, consider:
- Adding Redis for token storage (modify `oauth-wrapper/main.go`)
- Implementing rate limiting
- Adding monitoring/alerting