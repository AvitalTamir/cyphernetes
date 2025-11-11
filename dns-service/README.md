# Cyphernetes DNS Service

A Cloudflare Worker that provides dynamic DNS and HTTPS tunneling for Cyphernetes notebook sharing.

## Features

- **Dynamic Subdomain Generation**: Creates random subdomains like `abc123.go.cyphernet.es`
- **Token-Based Security**: Each subdomain has a secure token and expiration time
- **WebSocket Tunneling**: Routes HTTP requests through WebSocket tunnels to local servers
- **Cloudflare KV Storage**: Uses Cloudflare KV for fast, distributed token storage
- **Auto-Expiry**: Tokens automatically expire and clean up

## Architecture

```
User Browser → abc123.go.cyphernet.es → Cloudflare Worker → WebSocket Tunnel → Local Server
```

## API Endpoints

### POST /api/subdomain
Creates a new subdomain mapping.

**Request:**
```json
{
  "expires_in": 600
}
```

**Response:**
```json
{
  "subdomain": "abc123def456",
  "expires_at": "2024-01-01T12:00:00Z",
  "expires_in": 600
}
```

### WebSocket /tunnel/{subdomain}
Establishes tunnel connection for a subdomain.

**Headers:**
- `Authorization: Bearer {token}`
- `Upgrade: websocket`

## Setup

1. **Install dependencies:**
   ```bash
   npm install
   ```

2. **Create KV namespace:**
   ```bash
   wrangler kv:namespace create "SUBDOMAINS"
   ```

3. **Update wrangler.toml with your KV namespace ID**

4. **Set up custom domain:**
   - Add `go.cyphernet.es` to your Cloudflare zone
   - Update the route in `wrangler.toml`

5. **Deploy:**
   ```bash
   npm run deploy
   ```

## Development

```bash
# Start local development server
npm run dev

# Test subdomain creation
curl -X POST http://localhost:8787/api/subdomain \
  -H "Content-Type: application/json" \
  -d '{"expires_in": 600}'

# Visit generated subdomain
curl http://abc123.localhost:8787/
```

## Environment Variables

- `TUNNEL_DOMAIN`: Base domain for tunnels (default: `go.cyphernet.es`)
- `MAX_SUBDOMAIN_AGE_SECONDS`: Maximum token lifetime (default: `600`)

## Security

- All subdomains are randomly generated (12 characters, a-z0-9)
- Tokens automatically expire and are cleaned up
- WebSocket connections require Bearer token authentication
- CORS headers restrict API access

## Monitoring

The worker logs all tunnel connections and errors to Cloudflare Analytics. Monitor:
- Active tunnel connections
- Token creation rate
- Request forwarding success rate
- Error rates and types

## Production Checklist

- [ ] Set up custom domain `go.cyphernet.es`
- [ ] Configure KV namespace
- [ ] Set appropriate token expiration times
- [ ] Enable Cloudflare Analytics
- [ ] Set up alerting for high error rates
- [ ] Configure rate limiting if needed