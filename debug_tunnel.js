#!/usr/bin/env node
const WebSocket = require('ws');
const crypto = require('crypto');

async function testTunnelConnection() {
  console.log('🔍 Testing tunnel connection...');

  // Step 1: Create a subdomain via the API
  console.log('\n1. Creating subdomain...');
  
  const subdomainRequest = {
    expires_in: 600 // 10 minutes
  };

  try {
    const subdomainResponse = await fetch('https://go.cyphernet.es/api/subdomain', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(subdomainRequest)
    });

    if (!subdomainResponse.ok) {
      throw new Error(`Subdomain creation failed: ${subdomainResponse.status} ${subdomainResponse.statusText}`);
    }

    const subdomainData = await subdomainResponse.json();
    console.log('✅ Subdomain created:', subdomainData);

    const { subdomain } = subdomainData;
    
    // Step 2: Generate a token (simulate what the Go server does)
    const tokenBytes = crypto.randomBytes(32);
    const token = tokenBytes.toString('hex');
    console.log('🔑 Generated token:', token);

    // Step 3: Try to establish WebSocket connection
    console.log('\n2. Attempting WebSocket tunnel connection...');
    
    const tunnelUrl = `wss://go.cyphernet.es/tunnel/${subdomain}`;
    console.log('🔗 Connecting to:', tunnelUrl);

    const ws = new WebSocket(tunnelUrl, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    });

    ws.on('open', () => {
      console.log('✅ WebSocket connection established!');
      
      // Test sending a message
      const testMessage = {
        id: 'test-123',
        method: 'GET',
        url: '/',
        headers: {},
        body: ''
      };
      
      console.log('📤 Sending test message...');
      ws.send(JSON.stringify(testMessage));
    });

    ws.on('message', (data) => {
      console.log('📥 Received message:', data.toString());
    });

    ws.on('error', (error) => {
      console.error('❌ WebSocket error:', error);
    });

    ws.on('close', (code, reason) => {
      console.log('🔌 WebSocket closed:', code, reason.toString());
    });

    // Keep connection alive for testing
    setTimeout(() => {
      console.log('\n3. Closing connection...');
      ws.close();
    }, 5000);

  } catch (error) {
    console.error('❌ Error:', error.message);
  }
}

testTunnelConnection();