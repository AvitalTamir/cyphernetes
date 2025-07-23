export class TunnelDurableObject {
  private state: DurableObjectState;
  private sessions: Map<string, WebSocket> = new Map();
  private pendingRequests: Map<string, {
    resolve: (response: Response) => void;
    reject: (error: Error) => void;
    timeout: number;
  }> = new Map();
  private streamingResponses: Map<string, {
    controller: ReadableStreamDefaultController<Uint8Array>;
    headers?: Headers;
    status?: number;
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
      
      // Handle different response types
      if (response.type === 'chunk') {
        this.handleStreamingChunk(response);
      } else if (response.type === 'end') {
        this.handleStreamingEnd(response);
      } else if (response.type === 'complete') {
        this.handleCompleteResponse(response);
      }
    } catch (error) {
      console.error('Failed to parse tunnel response:', error);
    }
  }

  private handleCompleteResponse(response: any): void {
    const pending = this.pendingRequests.get(response.id);
    if (!pending) return;
    
    clearTimeout(pending.timeout);
    this.pendingRequests.delete(response.id);
    
    // Create response
    const headers = new Headers(response.headers || {});
    const responseObj = new Response(response.body || '', {
      status: response.status_code || 200,
      headers,
    });
    
    pending.resolve(responseObj);
  }

  private handleStreamingChunk(response: any): void {
    const pending = this.pendingRequests.get(response.id);
    
    if (!pending && !this.streamingResponses.has(response.id)) {
      return;
    }
    
    // If this is the first chunk, create the streaming response
    if (pending && response.headers) {
      clearTimeout(pending.timeout);
      this.pendingRequests.delete(response.id);
      
      const headers = new Headers(response.headers || {});
      const status = response.status_code || 200;
      
      // Create a readable stream for the response
      const stream = new ReadableStream<Uint8Array>({
        start: (controller) => {
          // Store the controller for later chunks
          this.streamingResponses.set(response.id, {
            controller,
            headers,
            status,
          });
        },
        cancel: () => {
          // Clean up when stream is cancelled
          this.streamingResponses.delete(response.id);
        }
      });
      
      const responseObj = new Response(stream, {
        status,
        headers,
      });
      
      pending.resolve(responseObj);
    }
    
    // Send chunk data to the stream
    const streaming = this.streamingResponses.get(response.id);
    if (streaming && response.body) {
      const encoder = new TextEncoder();
      streaming.controller.enqueue(encoder.encode(response.body));
    }
  }

  private handleStreamingEnd(response: any): void {
    const streaming = this.streamingResponses.get(response.id);
    if (streaming) {
      // Close the stream
      streaming.controller.close();
      this.streamingResponses.delete(response.id);
    }
  }
}