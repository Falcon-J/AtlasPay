import http from 'k6/http';
import { check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const submitAttempts = new Counter('submit_attempts');
const acceptedOrders = new Rate('accepted_orders');
const submitLatency = new Trend('submit_latency', true);
const targetRPM = Number(__ENV.TARGET_RPM || '12500');
const duration = __ENV.DURATION || '10s';
const baseURL = __ENV.BASE_URL || 'http://host.docker.internal:8080';
const loadSKU = __ENV.LOAD_SKU || 'SUBMIT-LOAD-001';
const skuCount = Math.max(1, Number(__ENV.SKU_COUNT || '1'));

function skuFor(index) {
    return skuCount === 1 ? loadSKU : `${loadSKU}-${index % skuCount}`;
}

export const options = {
    scenarios: {
        order_submit: {
            executor: 'constant-arrival-rate',
            rate: targetRPM,
            timeUnit: '1m',
            duration,
            preAllocatedVUs: Number(__ENV.PREALLOCATED_VUS || '100'),
            maxVUs: Number(__ENV.MAX_VUS || '500'),
        },
    },
    thresholds: {
        http_req_failed: ['rate<0.01'],
        accepted_orders: ['rate>0.99'],
    },
};

export function setup() {
    const stamp = Date.now();
    const auth = http.post(`${baseURL}/api/auth/register`, JSON.stringify({
        email: `submit-load-${stamp}@example.com`,
        password: 'loadPass123',
        first_name: 'Submit',
        last_name: 'Load',
    }), { headers: { 'Content-Type': 'application/json' } });
    check(auth, { 'load user registered': (r) => r.status === 201 });
    const token = JSON.parse(auth.body).data.access_token;
    for (let i = 0; i < skuCount; i += 1) {
        const restock = http.post(`${baseURL}/api/inventory/restock`, JSON.stringify({
            sku: skuFor(i),
            quantity: 1000000,
        }), {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${token}`,
            },
        });
        check(restock, { 'load inventory seeded': (r) => r.status === 200 });
    }
    return { token };
}

export default function (data) {
    submitAttempts.add(1);
    const startedAt = Date.now();
    const response = http.post(`${baseURL}/api/orders/`, JSON.stringify({
        items: [{ sku: skuFor(__VU + __ITER), quantity: 1 }],
    }), {
        headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${data.token}`,
        },
        tags: { name: 'POST /api/orders' },
    });
    const accepted = check(response, { 'order accepted': (r) => r.status === 201 });
    acceptedOrders.add(accepted);
    submitLatency.add(Date.now() - startedAt);
}

export function handleSummary(data) {
    return {
        stdout: JSON.stringify({
            target_rpm: targetRPM,
            duration,
            load_sku: loadSKU,
            sku_count: skuCount,
            submit_attempts: data.metrics.submit_attempts.values.count,
            accepted_rate: data.metrics.accepted_orders.values.rate,
            submit_latency_p95_ms: data.metrics.submit_latency.values['p(95)'],
            http_failed_rate: data.metrics.http_req_failed.values.rate,
        }, null, 2),
    };
}
