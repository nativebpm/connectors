import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '5s', target: 5 },   // Ramp-up to 5 concurrent workers
    { duration: '15s', target: 15 }, // Scale to 15 concurrent workers
    { duration: '5s', target: 0 },   // Cool down
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],   // Less than 5% HTTP errors
    http_req_duration: ['p(95)<1500'], // 95% of tasks must process in under 1.5s
  },
};

const BASE_URL = 'http://localhost:8080/engine-rest';

export default function () {
  // 1. Fetch and Lock task (simulate worker long polling)
  const fetchUrl = `${BASE_URL}/external-task/fetchAndLock`;
  const fetchPayload = JSON.stringify({
    workerId: `k6-worker-${__VU}`,
    maxTasks: 1,
    usePriority: true,
    topics: [
      {
        topicName: 'generate_invoice_pdf',
        lockDuration: 30000,
        variables: ['invoiceId', 'customerName', 'amount'],
      },
    ],
  });

  const headers = { 'Content-Type': 'application/json' };
  const fetchRes = http.post(fetchUrl, fetchPayload, { headers });

  check(fetchRes, {
    'fetch status is 200': (r) => r.status === 200,
  });

  if (fetchRes.status === 200) {
    const tasks = JSON.parse(fetchRes.body);
    if (tasks && tasks.length > 0) {
      const task = tasks[0];

      // Simulate local PDF generation delay
      sleep(0.05); 

      // 2. Complete the task, sending generated PDF back in base64
      const completeUrl = `${BASE_URL}/external-task/${task.id}/complete`;
      const completePayload = JSON.stringify({
        workerId: `k6-worker-${__VU}`,
        variables: {
          invoicePdfBase64: {
            value: 'JVBERi0xLjQKJd...[mocked-pdf-base64-bytes]...',
            type: 'String'
          }
        }
      });

      const completeRes = http.post(completeUrl, completePayload, { headers });
      check(completeRes, {
        'complete status is 204': (r) => r.status === 204,
      });
    }
  }

  sleep(0.1); // Worker poll interval pacing
}
