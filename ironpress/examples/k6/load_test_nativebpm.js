import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '5s', target: 5 },   // Ramp-up to 5 VUs
    { duration: '15s', target: 15 }, // Keep at 15 VUs
    { duration: '5s', target: 0 },   // Cool down
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],   // Less than 5% errors
    http_req_duration: ['p(95)<1500'], // 95% of tasks must complete in under 1.5s
  },
};

const BASE_URL = 'http://localhost:8080/api/v1'; // NativeBPM REST API base
const HEADERS = {
  'Content-Type': 'application/json',
  'Authorization': 'Bearer nativebpm-api-auth-token-123'
};

export default function () {
  // 1. Fetch active tasks for invoice generation
  const listUrl = `${BASE_URL}/tasks?status=ACTIVE`;
  const listRes = http.get(listUrl, { headers: HEADERS });

  check(listRes, {
    'list status is 200': (r) => r.status === 200,
  });

  if (listRes.status === 200) {
    const tasks = JSON.parse(listRes.body);
    const invoiceTasks = tasks ? tasks.filter(t => t.activity_id === 'generate_invoice') : [];

    if (invoiceTasks.length > 0) {
      const task = invoiceTasks[0];

      // 2. Claim the task
      const claimUrl = `${BASE_URL}/tasks/${task.id}/claim`;
      const claimPayload = JSON.stringify({
        assignee: `k6-worker-${__VU}`
      });
      const claimRes = http.post(claimUrl, claimPayload, { headers: HEADERS });

      check(claimRes, {
        'claim status is 200': (r) => r.status === 200,
      });

      if (claimRes.status === 200) {
        // Simulate PDF rendering delay
        sleep(0.05);

        // 3. Complete the task with generated PDF payload
        const completeUrl = `${BASE_URL}/tasks/${task.id}/complete`;
        const completePayload = JSON.stringify({
          variables: {
            invoicePdfBase64: 'JVBERi0xLjQKJd...[mocked-pdf-base64-bytes]...',
            generationTime: new Date().toISOString()
          }
        });

        const completeRes = http.post(completeUrl, completePayload, { headers: HEADERS });
        check(completeRes, {
          'complete status is 200': (r) => r.status === 200,
        });
      }
    }
  }

  sleep(0.1); // Worker poll interval pacing
}
