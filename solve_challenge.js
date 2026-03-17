// Vercel v2 challenge solver
// Usage: node solve_challenge.js <token>
// Outputs the solution to stdout
const token = process.argv[2];
if (!token) { console.error('Usage: node solve_challenge.js <token>'); process.exit(1); }

const fs = require('fs');
const path = require('path');
const vm = require('vm');

const jsCode = fs.readFileSync(path.join(__dirname, 'challenge.v2.min.js'), 'utf-8');
const wasmBuffer = fs.readFileSync(path.join(__dirname, 'challenge.v2.wasm'));

const fakePort = {
    onmessage: null,
    postMessage: (msg) => {
        if (msg && msg.type && msg.type.includes('response')) {
            if (msg.success) {
                process.stdout.write(JSON.stringify(msg));
                process.exit(0);
            } else {
                process.stderr.write('Challenge failed: ' + JSON.stringify(msg) + '\n');
                process.exit(1);
            }
        }
    }
};

const sandbox = {
    console: { log: () => {}, error: (...args) => process.stderr.write(args.join(' ') + '\n'), warn: () => {} },
    setTimeout, clearTimeout, setInterval, clearInterval,
    TextEncoder, TextDecoder,
    Uint8Array, Uint8ClampedArray, Int8Array, Int16Array, Int32Array, Float32Array, Float64Array,
    ArrayBuffer, SharedArrayBuffer, DataView,
    Map, Set, WeakMap, WeakRef, FinalizationRegistry,
    Error, TypeError, RangeError, URIError, SyntaxError,
    Promise, Object, Array, String, Number, Boolean, BigInt, Symbol,
    Math, JSON, Date, Proxy,
    NaN, Infinity, undefined,
    isNaN, isFinite, parseInt, parseFloat,
    decodeURIComponent, encodeURIComponent, decodeURI, encodeURI,
    RegExp, Reflect, WebAssembly, Response,
    performance: { now: performance.now.bind(performance) },
    crypto: require('crypto').webcrypto,
    SharedWorkerGlobalScope: {},
    atob: (s) => Buffer.from(s, 'base64').toString('binary'),
    btoa: (s) => Buffer.from(s, 'binary').toString('base64'),
    fetch: async (url, opts) => {
        // If this is the challenge submission (POST), intercept and extract solution
        if (opts && opts.method === 'POST') {
            const solution = opts.headers['x-vercel-challenge-solution'];
            const token = opts.headers['x-vercel-challenge-token'];
            const version = opts.headers['x-vercel-challenge-version'];
            // Output the solution and exit
            process.stdout.write(JSON.stringify({ solution, token, version }));
            process.exit(0);
        }
        // Otherwise return the WASM file
        const resp = new Response(wasmBuffer, {
            status: 200,
            headers: { 'Content-Type': 'application/wasm' }
        });
        return resp;
    },
};
sandbox.self = sandbox;
sandbox.globalThis = sandbox;

const context = vm.createContext(sandbox);
vm.runInContext(jsCode, context, { timeout: 5000 });

// The JS sets self.onmessage
if (typeof sandbox.onmessage !== 'function') {
    console.error('Failed: onmessage not set');
    process.exit(1);
}

// First message: provide the port (simulates SharedWorker connect)
sandbox.onmessage({ data: { port: fakePort } });

if (!fakePort.onmessage) {
    console.error('Failed: port.onmessage not set');
    process.exit(1);
}

// Second message through port: solve request
fakePort.onmessage({
    data: {
        type: 'solve-request',
        token: token,
        version: '2',
    }
});

// Timeout
setTimeout(() => { console.error('Timeout'); process.exit(1); }, 25000);
