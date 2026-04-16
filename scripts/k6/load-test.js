import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const orderSuccessRate = new Rate('order_success_rate');
const orderLatency = new Trend('order_latency', true);
const paymentLatency = new Trend('payment_latency', true);
const totalOrders = new Counter('total_orders');

// Test configuration
export const options = {
    summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
    stages: [
        // Ramp up to 100 VUs over 1 minute
        { duration: '1m', target: 100 },
        // Stay at 100 VUs for 3 minutes
        { duration: '3m', target: 100 },
        // Ramp up to 500 VUs over 2 minutes (stress test)
        { duration: '2m', target: 500 },
        // Stay at 500 VUs for 2 minutes
        { duration: '2m', target: 500 },
        // Ramp down to 0
        { duration: '1m', target: 0 },
    ],
    thresholds: {
        // 95th percentile response time under 200ms
        'http_req_duration{scenario:default}': ['p(95)<200'],
        // Error rate under 1%
        'http_req_failed': ['rate<0.01'],
        // Order success rate above 95%
        'order_success_rate': ['rate>0.95'],
        // Order latency p95 under 500ms
        'order_latency': ['p(95)<500'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
let authToken = '';

// Setup function - runs once before all VUs
export function setup() {
    const email = `test-${Date.now()}@example.com`;

    // Register a test user
    const registerRes = http.post(`${BASE_URL}/api/auth/register`, JSON.stringify({
        email: email,
        password: 'testpassword123',
        first_name: 'Load',
        last_name: 'Test'
    }), {
        headers: { 'Content-Type': 'application/json' }
    });

    if (registerRes.status === 201 || registerRes.status === 409) {
        if (registerRes.status === 201) {
            const body = JSON.parse(registerRes.body);
            return { token: body.data.access_token };
        }

        // Login
        const loginRes = http.post(`${BASE_URL}/api/auth/login`, JSON.stringify({
            email: email,
            password: 'testpassword123'
        }), {
            headers: { 'Content-Type': 'application/json' }
        });

        if (loginRes.status === 200) {
            const body = JSON.parse(loginRes.body);
            return { token: body.data.access_token };
        }
    }

    // Fallback - try admin login
    const adminLoginRes = http.post(`${BASE_URL}/api/auth/login`, JSON.stringify({
        email: 'admin@atlaspay.com',
        password: 'admin123'
    }), {
        headers: { 'Content-Type': 'application/json' }
    });

    if (adminLoginRes.status === 200) {
        const body = JSON.parse(adminLoginRes.body);
        return { token: body.data.access_token };
    }

    return { token: '' };
}

// Main test function
export default function(data) {
    const headers = {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${data.token}`
    };

    // Test scenarios
    group('Health Check', () => {
        const res = http.get(`${BASE_URL}/health`);
        check(res, {
            'health check status is 200': (r) => r.status === 200,
            'health check is healthy': (r) => JSON.parse(r.body).status === 'healthy'
        });
    });

    group('Create Order', () => {
        const startTime = Date.now();
        
        const orderData = {
            items: [
                { sku: 'LAPTOP-001', quantity: 1 },
                { sku: 'HEADPHONES-001', quantity: 2 }
            ]
        };

        const res = http.post(`${BASE_URL}/api/orders`, JSON.stringify(orderData), { headers });
        
        const latency = Date.now() - startTime;
        orderLatency.add(latency);
        totalOrders.add(1);

        const success = check(res, {
            'order created successfully': (r) => r.status === 201,
            'order has id': (r) => {
                if (r.status === 201) {
                    const body = JSON.parse(r.body);
                    return body.data && body.data.order && body.data.order.id;
                }
                return false;
            },
            'order status is pending': (r) => {
                if (r.status === 201) {
                    const body = JSON.parse(r.body);
                    return body.data.order.status === 'pending';
                }
                return false;
            }
        });

        orderSuccessRate.add(success);

        if (res.status === 201) {
            const body = JSON.parse(res.body);
            const orderId = body.data.order.id;

            // Get order details
            sleep(0.5);
            
            group('Get Order', () => {
                const getRes = http.get(`${BASE_URL}/api/orders/${orderId}`, {
                    headers,
                    tags: { name: 'GET /api/orders/{id}' },
                });
                check(getRes, {
                    'get order status is 200': (r) => r.status === 200,
                    'order data matches': (r) => {
                        const getData = JSON.parse(r.body);
                        return getData.data.order.id === orderId;
                    }
                });
            });
        }
    });

    group('List Orders', () => {
        const res = http.get(`${BASE_URL}/api/orders?page=1&page_size=10`, { headers });
        check(res, {
            'list orders status is 200': (r) => r.status === 200,
            'orders list is array': (r) => {
                const body = JSON.parse(r.body);
                return Array.isArray(body.data.orders);
            }
        });
    });

    group('Check Inventory', () => {
        const res = http.get(`${BASE_URL}/api/inventory/LAPTOP-001`, { headers });
        check(res, {
            'inventory check status is 200': (r) => r.status === 200,
            'inventory has quantity': (r) => {
                const body = JSON.parse(r.body);
                return body.data.item.quantity >= 0;
            }
        });
    });

    // Simulate real user behavior with random sleep
    sleep(Math.random() * 2 + 1);
}

// Teardown function
export function teardown(data) {
    console.log('Load test completed');
}

// Handle summary
export function handleSummary(data) {
    return {
        'stdout': textSummary(data, { indent: ' ', enableColors: true }),
        'results/summary.json': JSON.stringify(data, null, 2),
    };
}

function textSummary(data, opts) {
    const metrics = data.metrics;
    const value = (metricName, fieldName, fallback = 0) => {
        const metric = metrics[metricName];
        if (!metric || !metric.values || metric.values[fieldName] === undefined || metric.values[fieldName] === null) {
            return fallback;
        }
        return metric.values[fieldName];
    };
    const fixed = (metricName, fieldName, decimals = 2) => value(metricName, fieldName).toFixed(decimals);

    let output = '\n========== LOAD TEST RESULTS ==========\n\n';
    
    output += `Total Requests: ${value('http_reqs', 'count')}\n`;
    output += `Failed Requests: ${(value('http_req_failed', 'rate') * 100).toFixed(2)}%\n`;
    output += `Avg Response Time: ${fixed('http_req_duration', 'avg')}ms\n`;
    output += `P95 Response Time: ${fixed('http_req_duration', 'p(95)')}ms\n`;
    output += `P99 Response Time: ${fixed('http_req_duration', 'p(99)')}ms\n`;
    output += `Total Orders: ${value('total_orders', 'count')}\n`;
    output += `Order Success Rate: ${(value('order_success_rate', 'rate') * 100).toFixed(2)}%\n`;
    
    output += '\n========================================\n';
    
    return output;
}
