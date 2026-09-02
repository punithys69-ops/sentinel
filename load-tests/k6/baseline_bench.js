import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const admitted = new Counter('admitted_requests');
const rejected = new Counter('rejected_requests');

export const options = {
    scenarios: {
        baseline: {
            executor: 'constant-arrival-rate',
            rate: 1000,
            timeUnit: '1s',
            duration: '30s',
            preAllocatedVUs: 200,
            maxVUs: 500,
        },
    },
    thresholds: {},
};

export default function () {
    const res = http.get('http://localhost:8080/api/test');

    if (res.status === 200) {
        admitted.add(1);
    } else if (res.status === 429) {
        rejected.add(1);
    }

    check(res, {
        'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    });
}
