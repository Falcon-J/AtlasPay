const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const html = fs.readFileSync(`${__dirname}/index.html`, 'utf8');
const script = html.match(/<script>([\s\S]*?)<\/script>/)[1].replace(/\s*init\(\);\s*$/, '');

function fixture({ hostname = 'localhost', offline = false, states = ['completed'], sagaError = 0, losePost = false, postStatus = 200, invalidPost = false } = {}) {
    const elements = new Map();
    const calls = [];
    let sagaReads = 0;
    const element = id => {
        if (!elements.has(id)) elements.set(id, {
            style: {}, children: [], classList: { add() {}, remove() {} },
            appendChild(item) { this.children.push(item); }, replaceChildren() { this.children = []; }
        });
        return elements.get(id);
    };
    const context = vm.createContext({
        window: { location: { hostname } },
        document: { getElementById: element, createElement: () => ({}) },
        URLSearchParams, AbortController, setTimeout(fn, ms) { if (ms < 1000) queueMicrotask(fn); }, clearTimeout() {}, setInterval() {},
        fetch: async (url, options = {}) => {
            calls.push({ url, options });
            if (offline) throw new Error('Connection refused');
            if (losePost && url.endsWith('/api/orders') && options.method === 'POST') throw new Error('Response lost after acceptance');
            let data = {};
            let status = 200;
            if (url.endsWith('/api/auth/login')) data = { access_token: 'test', user: { id: 'user' } };
            if (url.includes('/api/inventory/')) data = { item: { quantity: 12, reserved_qty: 2 } };
            if (url.endsWith('/api/orders')) {
                status = postStatus;
                if (!invalidPost) data = { order: { id: 'order-1', status: 'pending', total_price: 73, currency: 'USD' } };
            }
            if (url.endsWith('/api/orders/order-1')) data = { order: { id: 'order-1', status: 'confirmed', total_price: 73, currency: 'USD' } };
            if (url.endsWith('/saga')) {
                status = sagaError || (sagaReads === 0 && states[0] === 'not-started' ? 404 : 200);
                data = { status: states[Math.min(sagaReads++, states.length - 1)], step_logs: [{ step_name: 'ReserveInventory', status: 'completed' }] };
            }
            return { ok: status < 400, status, json: async () => ({ data }) };
        }
    });
    vm.runInContext(script, context);
    return { element, calls, setSagaError: status => { sagaError = status; }, run: code => vm.runInContext(code, context) };
}

(async () => {
    const live = fixture({ states: ['not-started', 'running', 'completed'] });
    await live.run('init()');
    assert.equal(live.run('activeMode'), 'live');
    assert.equal(live.element('restock-button').disabled, true);
    await live.run('buyConsole()');
    assert.match(live.element('order-result').textContent, /order-1.*USD 73.*Saga completed/);
    assert.equal(live.element('saga-steps').children[0].textContent, 'ReserveInventory: completed');
    assert.equal(live.run('revenue'), 0, 'Backend must not invent fixed simulation revenue');
    assert.equal(live.run('inventoryCount'), 10, 'Backend stock comes from API, not animation');
    assert.ok(live.calls.filter(call => call.url.endsWith('/saga')).length >= 3);
    for (const call of live.calls.filter(call => /order-1/.test(call.url))) assert.equal(call.options.headers.Authorization, 'Bearer test');
    await live.run('restock()');
    assert.equal(live.calls.filter(call => call.url.includes('/restock')).length, 0);

    const compensated = fixture({ states: ['compensated'] });
    await compensated.run('init()');
    compensated.element('glitch-toggle').checked = true;
    await compensated.run('buyConsole()');
    assert.match(compensated.element('order-result').textContent, /Saga compensated/);
    assert.equal(JSON.parse(compensated.calls.find(call => call.url.endsWith('/api/orders')).options.body).items[0].sku, 'FAIL-PAYMENT-001');
    assert.equal(compensated.run('revenue'), 0);

    const denied = fixture({ sagaError: 403 });
    await denied.run('init()');
    await denied.run('buyConsole()');
    assert.equal(denied.run('activeMode'), 'offline');
    assert.equal(denied.run('demoOrderSequence'), 0);
    assert.equal(denied.run('isProcessing'), false);
    assert.equal(denied.element('checkout-button').disabled, true);
    denied.setSagaError(0);
    await denied.run('activateDemoMode()');
    await denied.run('retryBackend()');
    assert.equal(denied.run('activeMode'), 'live');
    assert.match(denied.element('order-result').textContent, /order-1.*Saga completed/);
    assert.equal(denied.calls.filter(call => call.url.endsWith('/api/orders')).length, 1, 'Retry must inspect the accepted order without creating another');
    assert.equal(denied.calls.filter(call => call.url.endsWith('/api/auth/register')).length, 1, 'Retry preserves the order owner session');
    assert.equal(denied.calls.filter(call => call.url.includes('/api/inventory/')).at(-1).url, 'http://localhost:8080/api/inventory/LAPTOP-001');

    for (const failure of [{ losePost: true }, { postStatus: 500 }, { invalidPost: true }]) {
        const unknown = fixture(failure);
        await unknown.run('init()');
        unknown.run("lastBackendOrderId = 'previous-order'");
        await unknown.run('buyConsole()');
        assert.match(unknown.element('order-result').textContent, /Checkout outcome unknown/);
        assert.equal(unknown.run('lastBackendOrderId'), null);
        await unknown.run('retryBackend()');
        assert.equal(unknown.run('activeMode'), 'live');
        assert.equal(unknown.element('checkout-button').disabled, true);
        await unknown.run('refreshBackendOrder()');
        await unknown.run('buyConsole()');
        assert.equal(unknown.calls.filter(call => call.url.endsWith('/api/orders')).length, 1, 'Unknown POST outcome must never trigger another order on reconnect');
    }
    const rejected = fixture({ postStatus: 400 });
    await rejected.run('init()');
    await rejected.run('buyConsole()');
    await rejected.run('retryBackend()');
    assert.equal(rejected.run('backendOrderPending'), false, 'Definite rejection permits a later checkout');
    assert.equal(rejected.element('checkout-button').disabled, false);

    const pending = fixture({ states: ['running'] });
    await pending.run('init()');
    await pending.run('buyConsole()');
    assert.equal(pending.run('activeMode'), 'live');
    assert.match(pending.element('order-result').textContent, /Saga running/);
    await pending.run('buyConsole()');
    assert.equal(pending.calls.filter(call => call.url.endsWith('/api/orders')).length, 1, 'Timeout must not submit a replacement');

    const offline = fixture({ offline: true });
    await offline.run('init()');
    assert.equal(offline.run('activeMode'), 'offline');
    assert.equal(offline.run('demoOrderSequence'), 0);
    assert.equal(offline.element('simulation-button').hidden, false);
    await offline.run('activateDemoMode()');
    assert.equal(offline.run('activeMode'), 'demo', 'Simulation requires explicit selection');

    const demo = fixture({ hostname: 'atlas-pay-two.vercel.app' });
    await demo.run('init()');
    await demo.run('buyConsole()');
    assert.equal(demo.run('revenue'), 499);
    assert.equal(demo.run('inventoryCount'), 9);
    demo.element('glitch-toggle').checked = true;
    await demo.run('buyConsole()');
    assert.equal(demo.run('revenue'), 499);
    assert.equal(demo.run('inventoryCount'), 9);
    await demo.run('restock()');
    assert.equal(demo.run('inventoryCount'), 19);
    assert.equal(demo.calls.length, 0);
    console.log('Backend persisted outcomes, pending/denied/offline boundaries and simulation checks passed.');
})().catch(error => { console.error(error); process.exitCode = 1; });
