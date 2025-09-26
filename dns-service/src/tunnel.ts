export interface TunnelManager {
  connections: Map<string, WebSocket>;
  pendingRequests: Map<string, {
    resolve: (response: Response) => void;
    reject: (error: Error) => void;
    timeout: number;
  }>;
}

export class HttpTunnelHandler {
  private connections = new Map<string, WebSocket>();
  private pendingRequests = new Map<string, {
    resolve: (response: Response) => void;
    reject: (error: Error) => void;
    timeout: number;
  }>();

  handleTunnelConnection(subdomain: string, ws: WebSocket): void {
    console.log('📡 Setting up tunnel connection for:', subdomain);
    this.connections.set(subdomain, ws);
    console.log('📊 Active connections:', this.connections.size);

    ws.addEventListener('message', (event) => {
      console.log('📨 Received message from tunnel:', subdomain);
      this.handleTunnelResponse(event.data as string);
    });

    ws.addEventListener('close', () => {
      console.log('🔌 Tunnel connection closed:', subdomain);
      this.connections.delete(subdomain);
      // Reject all pending requests for this tunnel
      this.pendingRequests.forEach((pending, requestId) => {
        if (requestId.startsWith(subdomain)) {
          clearTimeout(pending.timeout);
          pending.reject(new Error('Tunnel connection closed'));
          this.pendingRequests.delete(requestId);
        }
      });
    });

    ws.addEventListener('error', (error) => {
      console.error(`❌ Tunnel error for ${subdomain}:`, error);
    });
    
    console.log('✅ Tunnel connection setup complete for:', subdomain);
  }

  async forwardRequest(subdomain: string, request: Request): Promise<Response> {
    const connection = this.connections.get(subdomain);
    if (!connection) {
      throw new Error('No active tunnel connection');
    }

    // Generate unique request ID
    const requestId = `${subdomain}_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

    try {
      // Read the body within this request context
      const url = new URL(request.url);
      const headers: Record<string, string> = {};
      request.headers.forEach((value, key) => {
        headers[key] = value;
      });

      let body: string | undefined;
      if (request.method !== 'GET' && request.method !== 'HEAD') {
        body = await request.text();
      }

      const tunnelRequest = {
        id: requestId,
        method: request.method,
        url: url.pathname + url.search,
        headers,
        body,
      };

      // Create promise for response before sending
      const responsePromise = new Promise<Response>((resolve, reject) => {
        const timeout = setTimeout(() => {
          this.pendingRequests.delete(requestId);
          reject(new Error('Request timeout'));
        }, 30000); // 30 second timeout

        this.pendingRequests.set(requestId, {
          resolve,
          reject,
          timeout,
        });
      });

      // Send request through tunnel
      connection.send(JSON.stringify(tunnelRequest));

      // Wait for response
      return await responsePromise;
    } catch (error) {
      // Clean up on error
      this.pendingRequests.delete(requestId);
      throw error;
    }
  }

  private handleTunnelResponse(data: string): void {
    try {
      const response = JSON.parse(data);
      const pending = this.pendingRequests.get(response.id);
      
      if (!pending) {
        console.warn('Received response for unknown request:', response.id);
        return;
      }

      clearTimeout(pending.timeout);
      this.pendingRequests.delete(response.id);

      // Create Response object with proper body handling
      const headers = new Headers(response.headers || {});
      const body = response.body || '';
      
      // Create a new Response with the body as a string
      const responseObj = new Response(body, {
        status: response.statusCode || response.status_code || 200,
        headers,
      });

      pending.resolve(responseObj);
    } catch (error) {
      console.error('Failed to parse tunnel response:', error);
    }
  }

  getActiveConnections(): string[] {
    return Array.from(this.connections.keys());
  }

  isConnected(subdomain: string): boolean {
    return this.connections.has(subdomain);
  }
}

// Global tunnel manager instance
export const tunnelManager = new HttpTunnelHandler();