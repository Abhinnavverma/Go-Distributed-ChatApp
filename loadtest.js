import http from 'k6/http';
import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { randomString } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

// 1. Configuration: How hard do we hit it?
export const options = {
  // Let's start with a "Smoke Test" (Gentle)
  // We will ramp up to 50 users over 10 seconds, stay there for 10s, then ramp down.
  stages: [
    { duration: '1m', target: 1000 }, 
    
    // Stay at 1,000 users for 30 seconds (The "Crush" phase)
    { duration: '30s', target: 1000 },
    
    // Ramp down to 0
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'], 
  },
};

const BASE_URL = 'http://localhost';

export default function () {
  // 2. Setup: Create a unique identity for this VU
  const username = `user_${randomString(8)}`;
  const password = 'password123';
  const payload = JSON.stringify({ username, password });
  const params = { headers: { 'Content-Type': 'application/json' } };

  // 3. Register
  let res = http.post(`${BASE_URL}/register`, payload, params);
  check(res, { 'Register status is 201': (r) => r.status === 201 });

  // 4. Login
  res = http.post(`${BASE_URL}/login`, payload, params);
  check(res, { 'Login status is 200': (r) => r.status === 200 });

  // Extract the Token (Backend sends "access_token")
  // If this fails, the test explodes.
  let token;
  try {
    token = res.json('access_token');
  } catch(e) {
    console.error("Login failed, no JSON returned");
    return;
  }

  // 5. The WebSocket Attack
  const wsUrl = `ws://localhost/ws?token=${token}`;

  const response = ws.connect(wsUrl, {}, function (socket) {
    socket.on('open', function open() {
      // console.log(`VU ${username} connected`);
      
      // Send a message
      socket.send('I DRINK YOUR MILKSHAKE!');
      
      // Keep connection open for a short bit to simulate reading
      sleep(1); 
      
      socket.close();
    });

    socket.on('message', function (message) {
      // Optional: Verify we got our message back
      // console.log(`Received: ${message}`);
    });

    socket.on('close', function () {
      // console.log(`VU ${username} disconnected`);
    });

    socket.on('error', function (e) {
      if (e.error() != "websocket: close sent") {
        console.error('WebSocket error: ', e.error());
      }
    });
  });

  check(response, { 'WebSocket connected (101)': (r) => r && r.status === 101 });
  
  // Random sleep between users to mimic natural traffic
  sleep(1); 
}