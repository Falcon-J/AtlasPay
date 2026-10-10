const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

async function check(hostname) {
    const elements = new Map();
    const calls = [];
    const element = id => {
        if (!elements.has(id)) elements.set(id, { style: {}, classList: { add() {}, remove() {} }, appendChild() {} });
        return elements.get(id);
    };
    const context = vm.createContext({
        window: { location: { hostname }, ATLASPAY_API_BASE: '' },
        document: { getElementById: element, createElement: () => ({}) },
        URLSearchParams, AbortController, setTimeout, clearTimeout, setInterval() {},
        fetch: async (url, options = {}) => {
            calls.push({ url, options });
            let data = {};
            if (url.endsWith('/api/auth/login')) data = { access_token: 'test', user: { id: 'user' } };
            if (url.includes('/api/inventory/')) data = { item: { quantity: 12, reserved_qty: 2 } };
            return { ok: true, status: 200, json: async () => ({ data }) };
        }
    });
    const html = fs.readFileSync(`${__dirname}/index.html`, 'utf8');
    const script = html.match(/<script>([\s\S]*?)<\/script>/)[1].replace(/\s*init\(\);\s*$/, '');
    vm.runInContext(script, context);
    await vm.runInContext('init()', context);
    if (hostname === 'localhost') {
        assert.equal(vm.runInContext('activeMode', context), 'live');
        assert.equal(element('restock-button').disabled, true);
        assert.equal(vm.runInContext('inventoryCount', context), 10);
        await vm.runInContext('apiClient.checkout(false)', context);
        await vm.runInContext('apiClient.checkout(true)', context);
        const orders = calls.filter(call => call.url.endsWith('/api/orders'));
        assert.equal(JSON.parse(orders[0].options.body).items[0].sku, 'LAPTOP-001');
        assert.equal(JSON.parse(orders[1].options.body).items[0].sku, 'FAIL-PAYMENT-001');
        await vm.runInContext('restock()', context);
        assert.equal(calls.filter(call => call.url.includes('/restock')).length, 0);
    } else {
        assert.equal(vm.runInContext('activeMode', context), 'demo');
        assert.equal(calls.length, 0);
        assert.equal(element('restock-button').disabled, false);
        await vm.runInContext('restock()', context);
        assert.equal(vm.runInContext('inventoryCount', context), 20);
    }
}
Promise.all([check('localhost'), check('atlas-pay-two.vercel.app')])
    .then(() => console.log('Demo modes and operator-seeded backend checks passed.'))
    .catch(error => { console.error(error); process.exitCode = 1; });
