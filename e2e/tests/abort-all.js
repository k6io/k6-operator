import http from 'k6/http';
import { check } from 'k6';
import exec from 'k6/execution';

export let options = {
  stages: [
    { target: 200, duration: '30s' },
    { target: 0, duration: '30s' },
  ],
};

export default function () {
  const result = http.get('https://test-api.k6.io/public/crocodiles/');
  check(result, {
    'http response status code is 200': result.status === 200,
  });

  // abort at some random point in the 2nd half of the test
  if (exec.scenario.progress > 0.5) {
    const rnd = Math.floor(Math.random() * 100);
    if (rnd > 50) {
      exec.test.abort();
    }
  }
}
