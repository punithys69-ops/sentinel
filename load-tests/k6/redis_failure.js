import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const status200 = new Counter('status_200');
const status429 = new Counter('status_429');
const status500 = new Counter('status_500');
const statusOther = new Counter('status_other');

export const options = {
    scenarios: {
        redis_failure: {
            executor: 'constant-arrival-rate',
            rate: 200,
            timeUnit: '1s',
            duration: '20s',
            preAllocatedVUs: 50,
            maxVUs: 200,
        },
    },
    thresholds: {},
};

export default function () {
    const res = http.get('http://localhost:8080/api/test');

    if (res.status === 200) {
        status200.add(1);
    } else if (res.status === 429) {
        status429.add(1);
    } else if (res.status === 500) {
        status500.add(1);
    } else {
        statusOther.add(1);
    }

    check(res, {
        'status is 200, 429, or 500': (r) =>
            r.status === 200 || r.status === 429 || r.status === 500,
    });
}
