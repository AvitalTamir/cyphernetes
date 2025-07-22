#!/usr/bin/env node
const WebSocket = require('ws');
const crypto = require('crypto');

async function testWebSocketTunnel() {
  console.log('🔍 Testing WebSocket tunnel connection...');

  // Step 1: Create a subdomain
  console.log('\n1. Creating subdomain...');
  
  try {
    const subdomainResponse = await fetch('https://go.cyphernet.es/api/subdomain', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ expires_in: 600 })
    });

    if (!subdomainResponse.ok) {
      throw new Error(`Subdomain creation failed: ${subdomainResponse.status}`);
    }

    const subdomainData = await subdomainResponse.json();
    console.log('✅ Subdomain created:', subdomainData);

    const { subdomain } = subdomainData;
    
    // Step 2: Generate a token
    const token = crypto.randomBytes(32).toString('hex');
    console.log('🔑 Generated token:', token);

    // Step 3: Connect via WebSocket
    console.log('\n2. Connecting to WebSocket tunnel...');
    
    const tunnelUrl = `wss://go.cyphernet.es/tunnel/${subdomain}`;
    console.log('🔗 Connecting to:', tunnelUrl);

    const ws = new WebSocket(tunnelUrl, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });

    ws.on('open', () => {
      console.log('✅ WebSocket tunnel established!');
      
      // Simulate incoming request
      console.log('\n3. Waiting for requests...');
      console.log(`📡 Test URL: https://${subdomain}.go.cyphernet.es/`);
    });

    ws.on('message', (data) => {
      const request = JSON.parse(data);
      console.log('📥 Received request:', request);
      
      // Send response
      const response = {
        id: request.id,
        status_code: 200,
        headers: {
          'Content-Type': 'text/html'
        },
        body: '<h1>Hello from WebSocket Tunnel!</h1>'
      };
      
      ws.send(JSON.stringify(response));
      console.log('📤 Sent response');
    });

    ws.on('error', (error) => {
      console.error('❌ WebSocket error:', error.message);
    });

    ws.on('close', (code, reason) => {
      console.log('🔌 WebSocket closed:', code, reason.toString());
    });

    // Keep running
    console.log('\n✨ Tunnel is ready! Press Ctrl+C to stop.');

  } catch (error) {
    console.error('❌ Error:', error.message);
  }
}

testWebSocketTunnel();