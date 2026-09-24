import http from 'k6/http';
import { check } from 'k6';

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
  <h1>NativeBPM Ironpress WASM Warmup Benchmark</h1>
  <p>In-process WebAssembly execution via Wazero JIT runtime with pre-compilation cache.</p>
  <p>Testing zero-fork zero-disk in-memory PDF compilation throughput.</p>
</body>
</html>
`;

export const options = {
  scenarios: {
    warmup_load: {
      executor: 'constant-vus',
      vus: __ENV.TEST_VUS ? parseInt(__ENV.TEST_VUS) : 20,
      duration: __ENV.TEST_DURATION ? __ENV.TEST_DURATION : '20s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'], // Less than 1% errors
    http_req_duration: ['p(95)<300'], // 95% under 300ms
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:9099';

export default function () {
  const url = `${BASE_URL}/convert`;

  const data = {
    file: http.file(htmlData, 'index.html', 'text/html'),
    'page-size': 'a4',
    landscape: 'false',
    margin: '12',
    header: 'NativeBPM Warmup Benchmark',
    footer: 'Page {page} of {pages}',
  };

  const res = http.post(url, data);

  check(res, {
    'status is 200': (r) => r.status === 200,
    'content is PDF': (r) => r.headers['Content-Type'] === 'application/pdf',
    'body is not empty': (r) => r.body && r.body.length > 0,
  });
}
