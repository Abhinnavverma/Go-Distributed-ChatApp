import ws from 'k6/ws';
import http from 'k6/http';
import { check, sleep } from 'k6';

// ⚙️ CONFIGURATION
export const options = {
  // 🟢 RAMP UP STRATEGY (The Fix for "Connection Reset")
  stages: [
    { duration: '10s', target: 50 },  // Warm up: 0 to 50 users in 10s
    { duration: '30s', target: 1000 }, // Ramp up: 50 to 500 users
    { duration: '10s', target: 0 },   // Cool down
  ],
};

const BASE_URL = 'http://localhost';
const WS_URL = 'ws://localhost/ws';

export default function () {
  // Generate unique IDs for this iteration
  const uniqueId = __VU + '_' + __ITER + '_' + Date.now();
  const senderUser = `sender_${uniqueId}`;
  const receiverUser = `receiver_${uniqueId}`;
  const password = 'password123';

  // 1. REGISTER Two Users (So we have a guaranteed partner)
  const regParams = { headers: { 'Content-Type': 'application/json' } };
  
  http.post(`${BASE_URL}/register`, JSON.stringify({ username: senderUser, password: password }), regParams);
  // We register a partner but don't log them in (simulates sending to offline user, or just DB load)
  const resRegB = http.post(`${BASE_URL}/register`, JSON.stringify({ username: receiverUser, password: password }), regParams);

  // 2. LOGIN (Sender)
  const resLogin = http.post(`${BASE_URL}/login`, JSON.stringify({ username: senderUser, password: password }), regParams);
  
  const isLoginSuccess = check(resLogin, { 'login status is 200': (r) => r.status === 200 });
  if (!isLoginSuccess) {
    console.error(`Login failed for ${senderUser}`);
    return;
  }

  const token = resLogin.json('access_token');
  const headers = { headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' } };

  // 3. SEARCH for Receiver to get their ID
  // (Since we just created them, we need their ID to start a chat)
  const resSearch = http.get(`${BASE_URL}/api/users/search?q=${receiverUser}`, headers);
  const users = resSearch.json();
  const targetId = users[0].id;

  // 4. CREATE CONVERSATION
  const resChat = http.post(`${BASE_URL}/api/conversations`, JSON.stringify({ target_id: targetId }), headers);
  const conversationId = resChat.json('conversation_id');

  // 5. WEBSOCKET SPAM
  const wsUrlWithToken = `${WS_URL}?token=${token}`;
  
  const resWs = ws.connect(wsUrlWithToken, {}, function (socket) {
    socket.on('open', function open() {
      // Send 5 messages per user session
      for (let i = 0; i < 5; i++) {
        socket.send(JSON.stringify({
          content: `K6 Load Test Message ${i}`,
          conversation_id: conversationId,
        }));
        // Tiny sleep to mimic human typing speed (and prevent local socket exhaustion)
        sleep(0.5); 
      }
      socket.close();
    });

    socket.on('error', function (e) {
      if (e.error() != 'websocket: close sent') {
        console.error('WS Error: ', e.error());
      }
    });
  });

  check(resWs, { 'websocket status is 101': (r) => r && r.status === 101 });
}