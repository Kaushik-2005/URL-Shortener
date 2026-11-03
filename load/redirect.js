import http from 'k6/http';
import { check, sleep } from 'k6';

const baseURL = (__ENV.BASE_URL || 'http://app:8080')
    .replace(/\/+$/, '');

const shortCode = __ENV.SHORT_CODE;
const expectedDestination = __ENV.EXPECTED_DESTINATION;

if (!shortCode || !expectedDestination) {
    throw new Error('SHORT_CODE and EXPECTED_DESTINATION are required');
}

export const options = {
    vus: 5,
    duration: '20s',
    maxRedirects: 0,
    summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'max'],
    thresholds: {
        http_req_failed: ['rate==0'],
        checks: ['rate==1'],
    },
};

export default function () {
    const response = http.get(
        `${baseURL}/${encodeURIComponent(shortCode)}`,
        { timeout: '10s' },
    );

    check(response, {
        'status is 302': (r) => r.status === 302,
        'destination is correct': (r) =>
            r.headers.Location === expectedDestination,
    });

    sleep(0.1);
}