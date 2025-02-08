import http from 'k6/http';
import { check } from 'k6';

export let options = {
  stages: [
    { target: 100, duration: '1m30s' },
    { target: 200, duration: '30s' },
    { target: 0, duration: '30s' },
  ],
  thresholds: {
    http_req_duration: [
      {
        threshold: 'p(99) < 200',
      },
    ],
  },
  ext: {
    loadimpact: {
      name: 'Configured k6-operator test',
    }
  }
};

export default function () {
  const result = http.get('https://test-api.k6.io/public/crocodiles/');
  check(result, {
    'http response status code is 200': result.status === 200,
  });
}
