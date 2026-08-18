import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

const checkoutCompleted = new Rate('checkout_completed');
const checkoutLatency = new Trend('checkout_latency', true);
const checkoutAttempts = new Counter('checkout_attempts');
const ordersAccepted = new Rate('orders_accepted');

const targetRPM = Number(__ENV.TARGET_RPM || '12500');
const duration = __ENV.DURATION || '1m';
const baseURL = __ENV.BASE_URL || 'http://host.docker.internal:8080';
const loadSKU = __ENV.LOAD_SKU || 'LOAD-001';
const skuCount = Math.max(1, Number(__ENV.SKU_COUNT || '1'));

function skuFor(index) {
    return skuCount === 1 ? loadSKU : `${loadSKU}-${index % skuCount}`;
}

export const options = {
    scenarios: {
        checkout: {
            executor: 'constant-arrival-rate',
            rate: targetRPM,
            timeUnit: '1m',
            duration,
            preAllocatedVUs: Number(__ENV.PREALLOCATED_VUS || '250'),
            maxVUs: Number(__ENV.MAX_VUS || '1000'),
        },
    },
    thresholds: {
        http_req_failed: ['rate<0.01'],
        checkout_completed: ['rate>0.95'],
    },
};

export function setup() {
    const stamp = Date.now();
    const auth = http.post(`${baseURL}/api/auth/register`, JSON.stringify({
        email: `load-${stamp}@example.com`,
        password: 'loadPass123',
        first_name: 'Load',
        last_name: 'Test',
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
    const startedAt = Date.now();
    checkoutAttempts.add(1);
    const headers = {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${data.token}`,
    };
    const orderResponse = http.post(`${baseURL}/api/orders/`, JSON.stringify({
        items: [{ sku: skuFor(__VU + __ITER), quantity: 1 }],
    }), { headers, tags: { name: 'POST /api/orders' } });

    const created = check(orderResponse, {
        'checkout order accepted': (r) => r.status === 201,
    });
    ordersAccepted.add(created);
    if (!created) {
        checkoutCompleted.add(false);
        return;
    }

    const orderId = JSON.parse(orderResponse.body).data.order.id;
    let completed = false;
    for (let attempt = 0; attempt < Number(__ENV.POLL_ATTEMPTS || '60'); attempt += 1) {
        sleep(0.1);
        const saga = http.get(`${baseURL}/api/orders/${orderId}/saga`, {
            headers,
            tags: {
                name: 'GET /api/orders/{id}/saga',
                // The worker creates in-memory saga state after the order is
                // committed, so an initial 404 is expected eventual consistency.
                expected_response: 'false',
            },
        });
        if (saga.status !== 200) {
            continue;
        }
        const status = JSON.parse(saga.body).data.status;
        if (status === 'completed') {
            completed = true;
            break;
        }
        if (status === 'failed' || status === 'compensated') {
            break;
        }
    }

    checkoutCompleted.add(completed);
    checkoutLatency.add(Date.now() - startedAt);
    check({ completed }, { 'checkout saga completed': (value) => value.completed });
}

export function handleSummary(data) {
    return {
        stdout: JSON.stringify({
            target_rpm: targetRPM,
            duration,
            load_sku: loadSKU,
            sku_count: skuCount,
            checkout_attempts: data.metrics.checkout_attempts.values.count,
            orders_accepted_rate: data.metrics.orders_accepted.values.rate,
            completed_rate: data.metrics.checkout_completed.values.rate,
            latency_p95_ms: data.metrics.checkout_latency.values['p(95)'],
            http_failed_rate: data.metrics.http_req_failed.values.rate,
        }, null, 2),
    };
}
