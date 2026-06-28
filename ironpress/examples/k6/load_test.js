import http from 'k6/http';
import { check, sleep } from 'k6';

const htmlData = `
<!DOCTYPE html>
<html>
<head>
<style>
  body {
    font-family: sans-serif;
    color: #222;
    padding: 15px;
  }
  h1 { color: #d32f2f; }
  p { font-size: 14px; line-height: 1.5; }
</style>
</head>
<body>
  <h1>K6 Load Test Document</h1>
  <p>This is a lightweight document converted during concurrent load testing of the NativeBPM ironpress HTTP server wrapper.</p>
  <p>Testing response latencies, throughput, and memory scaling parameters under parallel PDF generations.</p>
</body>
</html>
`;

export const options = {
  stages: [
    { duration: '5s', target: 10 },  // Ramp-up to 10 VUs
    { duration: '15s', target: 20 }, // Sustained 20 VUs
    { duration: '5s', target: 0 },   // Ramp-down
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],  // Less than 1% errors
    http_req_duration: ['p(95)<1000'], // 95% of requests must complete under 1s
  },
};

export default function () {
  const url = 'http://localhost:8080/convert';

  const data = {
    file: http.file(htmlData, 'index.html', 'text/html'),
    'page-size': 'a4',
    landscape: 'false',
    margin: '12',
    header: 'K6 Load Test',
    footer: 'Page {page} of {pages}',
  };

  const res = http.post(url, data);

  check(res, {
    'status is 200': (r) => r.status === 200,
    'content is PDF': (r) => r.headers['Content-Type'] === 'application/pdf',
    'body is not empty': (r) => r.body && r.body.length > 0,
  });

  sleep(0.1); // pacing interval
}
