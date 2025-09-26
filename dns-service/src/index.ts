import { TunnelDurableObject } from './tunnel-durable-object';
import { NOTEBOOK_HTML, ASSETS } from './static-assets';

export interface Env {
  SUBDOMAINS: KVNamespace;
  TUNNEL_DOMAIN: string;
  MAX_SUBDOMAIN_AGE_SECONDS: string;
  TUNNEL: DurableObjectNamespace;
}

interface SubdomainData {
  clientIP: string;
  createdAt: number;
  expiresAt: number;
  tunnel_url?: string;
  token?: string;
  registered_at?: number;
}

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    const url = new URL(request.url);
    
    // CORS headers for API endpoints
    const corsHeaders = {
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type, Authorization',
    };

    // Handle CORS preflight
    if (request.method === 'OPTIONS') {
      return new Response(null, { headers: corsHeaders });
    }

    try {
      // API endpoint to create subdomain
      if (url.pathname === '/api/subdomain' && request.method === 'POST') {
        return await this.handleCreateSubdomain(request, env, corsHeaders);
      }

      // HTTP endpoint to register tunnel endpoint
      if (url.pathname.startsWith('/register/')) {
        return await this.handleTunnelRegistration(request, env);
      }

      // WebSocket tunnel endpoint
      if (url.pathname.startsWith('/tunnel/')) {
        return await this.handleWebSocketTunnel(request, env);
      }

      // Handle subdomain routing (e.g., abc123.go.cyphernet.es)
      const subdomain = this.extractSubdomain(url.hostname, env.TUNNEL_DOMAIN);
      if (subdomain) {
        return await this.handleSubdomainRequest(request, subdomain, env);
      }

      // Default response
      return new Response('Cyphernetes DNS Service', {
        headers: { 'Content-Type': 'text/plain', ...corsHeaders },
      });
    } catch (error) {
      console.error('Worker error:', error);
      return new Response('Internal Server Error', { 
        status: 500,
        headers: corsHeaders 
      });
    }
  },

  async handleCreateSubdomain(request: Request, env: Env, corsHeaders: Record<string, string>): Promise<Response> {
    try {
      const body = await request.json() as { expires_in?: number, token?: string };
      const expiresIn = body.expires_in || parseInt(env.MAX_SUBDOMAIN_AGE_SECONDS);
      
      // Generate random subdomain
      const subdomain = this.generateRandomSubdomain();
      
      // Get client IP
      const clientIP = request.headers.get('CF-Connecting-IP') || 'unknown';
      
      // Store subdomain mapping
      const data: SubdomainData = {
        clientIP,
        createdAt: Date.now(),
        expiresAt: Date.now() + (expiresIn * 1000),
        token: body.token, // Store token if provided
      };
      
      // Store in KV with TTL
      await env.SUBDOMAINS.put(subdomain, JSON.stringify(data), {
        expirationTtl: expiresIn,
      });
      
      console.log('📝 Created subdomain:', subdomain, 'with token:', data.token ? 'yes' : 'no');
      
      return new Response(JSON.stringify({
        subdomain,
        expires_at: new Date(data.expiresAt).toISOString(),
        expires_in: expiresIn,
      }), {
        headers: {
          'Content-Type': 'application/json',
          ...corsHeaders,
        },
      });
    } catch (error) {
      return new Response(JSON.stringify({ error: 'Failed to create subdomain' }), {
        status: 400,
        headers: {
          'Content-Type': 'application/json',
          ...corsHeaders,
        },
      });
    }
  },

  async handleTunnelRegistration(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    const subdomain = url.pathname.split('/')[2];
    
    console.log('📝 Tunnel registration request:', { subdomain });
    
    if (!subdomain) {
      return new Response('Invalid registration path', { status: 400 });
    }

    // Verify subdomain exists
    const subdomainData = await env.SUBDOMAINS.get(subdomain);
    if (!subdomainData) {
      console.log('❌ Subdomain not found:', subdomain);
      return new Response('Subdomain not found', { status: 404 });
    }

    // Get the tunnel endpoint from request body
    const { tunnel_url, token } = await request.json() as { tunnel_url: string, token: string };
    
    if (!tunnel_url || !token) {
      return new Response('Missing tunnel_url or token', { status: 400 });
    }

    // Store tunnel endpoint info
    const tunnelData = {
      ...JSON.parse(subdomainData),
      tunnel_url,
      token,
      registered_at: Date.now()
    };

    await env.SUBDOMAINS.put(subdomain, JSON.stringify(tunnelData));
    console.log('✅ Tunnel registered:', subdomain, '→', tunnel_url);

    return new Response(JSON.stringify({ success: true }), {
      headers: { 'Content-Type': 'application/json' }
    });
  },

  async handleWebSocketTunnel(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    const subdomain = url.pathname.split('/')[2];
    
    console.log('🔌 WebSocket tunnel request:', { subdomain });
    
    if (!subdomain) {
      return new Response('Invalid tunnel path', { status: 400 });
    }

    // Verify subdomain exists
    const subdomainData = await env.SUBDOMAINS.get(subdomain);
    if (!subdomainData) {
      console.log('❌ Subdomain not found:', subdomain);
      return new Response('Subdomain not found', { status: 404 });
    }

    // Get or create a Durable Object instance for this subdomain
    const id = env.TUNNEL.idFromName(subdomain);
    const tunnelObject = env.TUNNEL.get(id);
    
    // Forward the WebSocket request to the Durable Object
    return tunnelObject.fetch(request);
  },


  async handleSubdomainRequest(request: Request, subdomain: string, env: Env): Promise<Response> {
    // Check if subdomain exists
    const subdomainData = await env.SUBDOMAINS.get(subdomain);
    if (!subdomainData) {
      return new Response('Subdomain not found', { status: 404 });
    }

    const data = JSON.parse(subdomainData) as SubdomainData;
    
    // Check if expired
    if (Date.now() > data.expiresAt) {
      await env.SUBDOMAINS.delete(subdomain);
      return new Response('Subdomain expired', { status: 410 });
    }

    const url = new URL(request.url);
    
    // Serve the main HTML page for root requests
    if (url.pathname === '/' || url.pathname === '') {
      // Serve the full notebook HTML
      const html = NOTEBOOK_HTML;
      return new Response(html, {
        headers: { 'Content-Type': 'text/html' },
      });
    }
    
    // Serve static assets directly from embedded base64 content
    if (url.pathname.startsWith('/assets/')) {
      const assetPath = url.pathname.slice(1); // Remove leading slash
      const base64Content = ASSETS && ASSETS[assetPath];
      
      if (base64Content) {
        console.log('📦 Serving embedded asset:', assetPath);
        
        // Decode base64 content
        const binaryString = atob(base64Content);
        const bytes = new Uint8Array(binaryString.length);
        for (let i = 0; i < binaryString.length; i++) {
          bytes[i] = binaryString.charCodeAt(i);
        }
        
        // Set appropriate content type and headers
        const headers: Record<string, string> = {
          'Cache-Control': 'public, max-age=86400', // 24 hours
          'ETag': `"${assetPath}"`,
        };
        
        if (assetPath.endsWith('.js')) {
          headers['Content-Type'] = 'application/javascript; charset=utf-8';
        } else if (assetPath.endsWith('.css')) {
          headers['Content-Type'] = 'text/css; charset=utf-8';
        }
        
        return new Response(bytes, { headers });
      } else {
        return new Response('Asset not found', { 
          status: 404,
          headers: { 'Content-Type': 'text/plain' }
        });
      }
    }
    
    // Handle favicon and logo
    if (url.pathname === '/favicon.ico' || url.pathname === '/logo.png') {
      const assetPath = url.pathname.slice(1); // Remove leading slash
      const base64Content = ASSETS && ASSETS[assetPath];
      
      if (base64Content) {
        console.log('🖼️ Serving embedded icon:', assetPath);
        
        // Decode base64 content
        const binaryString = atob(base64Content);
        const bytes = new Uint8Array(binaryString.length);
        for (let i = 0; i < binaryString.length; i++) {
          bytes[i] = binaryString.charCodeAt(i);
        }
        
        // Set appropriate content type
        const headers: Record<string, string> = {
          'Cache-Control': 'public, max-age=86400', // 24 hours
          'ETag': `"${assetPath}"`,
        };
        
        if (assetPath === 'favicon.ico') {
          headers['Content-Type'] = 'image/x-icon';
        } else if (assetPath === 'logo.png') {
          headers['Content-Type'] = 'image/png';
        }
        
        return new Response(bytes, { headers });
      } else {
        return new Response('', { status: 204 }); // No content if not found
      }
    }

    // Forward API and other requests through Durable Object tunnel  
    try {
      console.log('🔄 Forwarding request through tunnel:', request.url);
      
      // Get the Durable Object for this subdomain
      const id = env.TUNNEL.idFromName(subdomain);
      const tunnelObject = env.TUNNEL.get(id);
      
      // Create a new request with the tunnel path
      const tunnelUrl = new URL(request.url);
      tunnelUrl.pathname = `/forward/${subdomain}${tunnelUrl.pathname}`;
      const tunnelRequest = new Request(tunnelUrl.toString(), request);
      
      // Forward to Durable Object
      const response = await tunnelObject.fetch(tunnelRequest);
      
      // Check if tunnel is connected
      if (response.status === 503) {
        return new Response(`
          <html>
            <head>
              <title>Cyphernetes - Tunnel Not Connected</title>
              <style>
                body { font-family: system-ui; padding: 40px; text-align: center; }
                .error { color: #ef4444; }
                .info { color: #6b7280; margin-top: 20px; }
              </style>
            </head>
            <body>
              <h1>🔌 Tunnel Not Connected</h1>
              <p class="error">The notebook server is not currently connected to this tunnel.</p>
              <p class="info">Subdomain: ${subdomain}.${env.TUNNEL_DOMAIN}</p>
              <p class="info">Please ensure the notebook server is running and sharing is active.</p>
            </body>
          </html>
        `, {
          status: 503,
          headers: { 'Content-Type': 'text/html' },
        });
      }
      
      return response;
    } catch (error) {
      console.error('Tunnel request failed:', error);
      return new Response(`
        <html>
          <head>
            <title>Cyphernetes - Tunnel Error</title>
            <style>
              body { font-family: system-ui; padding: 40px; text-align: center; }
              .error { color: #ef4444; }
            </style>
          </head>
          <body>
            <h1>❌ Tunnel Error</h1>
            <p class="error">Failed to forward request: ${error.message}</p>
          </body>
        </html>
      `, {
        status: 502,
        headers: { 'Content-Type': 'text/html' },
      });
    }
  },

  generateRandomSubdomain(): string {
    const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
    let result = '';
    for (let i = 0; i < 12; i++) {
      result += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return result;
  },

  extractSubdomain(hostname: string, baseDomain: string): string | null {
    if (!hostname.endsWith(baseDomain)) {
      return null;
    }
    
    const subdomain = hostname.slice(0, -(baseDomain.length + 1));
    return subdomain || null;
  },
};

// Export the Durable Object
export { TunnelDurableObject };