export class TunnelDurableObject {
  private state: DurableObjectState;
  private sessions: Map<string, WebSocket> = new Map();
  private pendingRequests: Map<string, {
    resolve: (response: Response) => void;
    reject: (error: Error) => void;
    timeout: number;
  }> = new Map();

  constructor(state: DurableObjectState) {
    this.state = state;
  }

  async fetch(request: Request): Promise<Response> {
    const url = new URL(request.url);
    
    // Handle WebSocket upgrade
    if (request.headers.get('Upgrade') === 'websocket') {
      const pair = new WebSocketPair();
      const [client, server] = Object.values(pair);
      
      // Accept the WebSocket with larger message limit
      server.accept();
      
      // Store the connection
      const subdomain = url.pathname.split('/').pop() || '';
      this.sessions.set(subdomain, server);
      
      // Handle messages
      server.addEventListener('message', (event) => {
        this.handleTunnelResponse(event.data as string);
      });
      
      server.addEventListener('close', () => {
        this.sessions.delete(subdomain);
        // Clean up any pending requests
        this.pendingRequests.forEach((pending, id) => {
          if (id.startsWith(subdomain)) {
            clearTimeout(pending.timeout);
            pending.reject(new Error('Tunnel connection closed'));
            this.pendingRequests.delete(id);
          }
        });
      });
      
      return new Response(null, {
        status: 101,
        webSocket: client,
      });
    }
    
    // Handle HTTP request forwarding
    if (url.pathname.startsWith('/forward/')) {
      const parts = url.pathname.split('/');
      const subdomain = parts[2];
      const connection = this.sessions.get(subdomain);
      
      if (!connection) {
        return new Response('No active tunnel connection', { status: 503 });
      }
      
      // Remove the /forward/{subdomain} prefix and restore original path
      const pathParts = parts.slice(3);
      const originalPath = pathParts.length > 0 ? '/' + pathParts.join('/') : '/';
      const forwardUrl = new URL(request.url);
      forwardUrl.pathname = originalPath;
      const forwardRequest = new Request(forwardUrl.toString(), request);
      
      // Forward the request
      return await this.forwardRequest(subdomain, forwardRequest, connection);
    }
    
    return new Response('Invalid request path', { status: 400 });
  }
  
  private async forwardRequest(subdomain: string, request: Request, connection: WebSocket): Promise<Response> {
    const requestId = `${subdomain}_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    
    try {
      // Read request data
      const url = new URL(request.url);
      const headers: Record<string, string> = {};
      request.headers.forEach((value, key) => {
        headers[key] = value;
      });
      
      let body: string | undefined;
      if (request.method !== 'GET' && request.method !== 'HEAD') {
        body = await request.text();
      }
      
      // Create response promise
      const responsePromise = new Promise<Response>((resolve, reject) => {
        const timeout = setTimeout(() => {
          this.pendingRequests.delete(requestId);
          reject(new Error('Request timeout'));
        }, 30000);
        
        this.pendingRequests.set(requestId, {
          resolve,
          reject,
          timeout,
        });
      });
      
      // Send request through tunnel
      connection.send(JSON.stringify({
        id: requestId,
        method: request.method,
        url: url.pathname + url.search,
        headers,
        body,
      }));
      
      return await responsePromise;
    } catch (error) {
      this.pendingRequests.delete(requestId);
      throw error;
    }
  }
  
  private handleTunnelResponse(data: string): void {
    try {
      const response = JSON.parse(data);
      const pending = this.pendingRequests.get(response.id);
      
      if (!pending) {
        return;
      }
      
      clearTimeout(pending.timeout);
      this.pendingRequests.delete(response.id);
      
      // Create response
      const headers = new Headers(response.headers || {});
      const responseObj = new Response(response.body || '', {
        status: response.status_code || 200,
        headers,
      });
      
      pending.resolve(responseObj);
    } catch (error) {
      console.error('Failed to parse tunnel response:', error);
    }
  }
}