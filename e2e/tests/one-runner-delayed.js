import http from 'k6/http';
import { check, sleep } from 'k6';
import exec from 'k6/execution';

export let options = {
  scenarios: {
    default: {
      executor: 'per-vu-iterations',
      vus: 10,
      iterations: 10,
      maxDuration: '1m30s'
    }
  }
};

export default function () {
  // delay the 1st instance from executing to simulate a delay
  const instanceId = exec.test.options.tags["instance_id"];
  if (instanceId == 1 && exec.scenario.iterationInInstance == 0) {
    sleep(60);
  }


  const result = http.get('https://test-api.k6.io/public/crocodiles/');
  check(result, {
    'http response status code is 200': result.status === 200,
  });
}
