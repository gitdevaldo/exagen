// Dashboard helper: solves Vercel challenge, completes onboarding, gets API keys
// Usage: node dashboard_helper.js <session-token>
// The session-token is the next-auth.session-token cookie value from auth callback
// Outputs JSON: {"success":true,"apiKey":"uuid"} or {"success":false,"error":"msg"}
const sessionToken = process.argv[2];
if (!sessionToken) {
    console.log(JSON.stringify({ success: false, error: 'Usage: node dashboard_helper.js <session-token>' }));
    process.exit(1);
}

const DASHBOARD = 'https://dashboard.exa.ai';

async function main() {
    try {
        // Step 1: Visit dashboard to trigger Vercel challenge
        let resp = await fetch(DASHBOARD + '/', {
            headers: {
                'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
                'Cookie': 'next-auth.session-token=' + sessionToken,
            },
        });

        // If we get 429, solve the Vercel challenge
        if (resp.status === 429) {
            const challengeToken = resp.headers.get('x-vercel-challenge-token');
            if (!challengeToken) {
                console.log(JSON.stringify({ success: false, error: 'No challenge token in 429 response' }));
                process.exit(1);
            }

            // Solve challenge using WASM
            const solution = await solveChallenge(challengeToken);

            // Submit solution
            const submitResp = await fetch(DASHBOARD + '/.well-known/vercel/security/request-challenge', {
                method: 'POST',
                headers: {
                    'X-Vercel-Challenge-Token': challengeToken,
                    'X-Vercel-Challenge-Solution': solution,
                    'X-Vercel-Challenge-Version': '2',
                    'Accept': '*/*',
                    'Origin': DASHBOARD,
                    'Referer': DASHBOARD + '/.well-known/vercel/security/static/challenge.v2.min.js',
                },
            });

            if (submitResp.status !== 204 && submitResp.status !== 200) {
                console.log(JSON.stringify({ success: false, error: 'Challenge submit returned ' + submitResp.status }));
                process.exit(1);
            }

            // Extract _vcrcs cookie from Set-Cookie header
            const cookies = submitResp.headers.getSetCookie?.() || [];
            let vcrcsCookie = '';
            for (const c of cookies) {
                if (c.startsWith('_vcrcs=')) {
                    vcrcsCookie = c.split(';')[0].replace('_vcrcs=', '');
                    break;
                }
            }

            // Step 2: Complete onboarding
            const onboardResp = await fetch(DASHBOARD + '/api/onboarding/complete', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Accept': '*/*',
                    'Cookie': 'next-auth.session-token=' + sessionToken + (vcrcsCookie ? '; _vcrcs=' + vcrcsCookie : ''),
                    'Referer': DASHBOARD + '/onboarding',
                    'Origin': DASHBOARD,
                },
                body: JSON.stringify({
                    codingTool: 'claude',
                    framework: 'mcp',
                    useCase: 'coding-agent',
                    prompt: 'using in coding agent',
                    latencyProfile: 'auto',
                    contentType: 'compact',
                }),
            });

            if (onboardResp.status !== 200) {
                const body = await onboardResp.text();
                console.log(JSON.stringify({ success: false, error: 'Onboarding returned ' + onboardResp.status + ': ' + body.substring(0, 200) }));
                process.exit(1);
            }

            // Step 3: Get API keys
            const keysResp = await fetch(DASHBOARD + '/api/get-api-keys', {
                headers: {
                    'Accept': '*/*',
                    'Cookie': 'next-auth.session-token=' + sessionToken + (vcrcsCookie ? '; _vcrcs=' + vcrcsCookie : ''),
                    'Referer': DASHBOARD + '/',
                },
            });

            if (keysResp.status !== 200) {
                const body = await keysResp.text();
                console.log(JSON.stringify({ success: false, error: 'Get API keys returned ' + keysResp.status + ': ' + body.substring(0, 200) }));
                process.exit(1);
            }

            const keysData = await keysResp.json();
            if (!keysData.apiKeys || keysData.apiKeys.length === 0) {
                console.log(JSON.stringify({ success: false, error: 'No API keys in response' }));
                process.exit(1);
            }

            console.log(JSON.stringify({ success: true, apiKey: keysData.apiKeys[0].id }));
        } else if (resp.status === 200 || resp.status === 302 || resp.status === 307) {
            // No Vercel challenge, proceed directly
            // Step 2: Complete onboarding
            const onboardResp = await fetch(DASHBOARD + '/api/onboarding/complete', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Accept': '*/*',
                    'Cookie': 'next-auth.session-token=' + sessionToken,
                    'Referer': DASHBOARD + '/onboarding',
                    'Origin': DASHBOARD,
                },
                body: JSON.stringify({
                    codingTool: 'claude',
                    framework: 'mcp',
                    useCase: 'coding-agent',
                    prompt: 'using in coding agent',
                    latencyProfile: 'auto',
                    contentType: 'compact',
                }),
            });

            if (onboardResp.status !== 200) {
                const body = await onboardResp.text();
                console.log(JSON.stringify({ success: false, error: 'Onboarding returned ' + onboardResp.status }));
                process.exit(1);
            }

            // Step 3: Get API keys
            const keysResp = await fetch(DASHBOARD + '/api/get-api-keys', {
                headers: {
                    'Accept': '*/*',
                    'Cookie': 'next-auth.session-token=' + sessionToken,
                    'Referer': DASHBOARD + '/',
                },
            });

            if (keysResp.status !== 200) {
                console.log(JSON.stringify({ success: false, error: 'Get API keys returned ' + keysResp.status }));
                process.exit(1);
            }

            const keysData = await keysResp.json();
            if (!keysData.apiKeys || keysData.apiKeys.length === 0) {
                console.log(JSON.stringify({ success: false, error: 'No API keys in response' }));
                process.exit(1);
            }

            console.log(JSON.stringify({ success: true, apiKey: keysData.apiKeys[0].id }));
        } else {
            console.log(JSON.stringify({ success: false, error: 'Unexpected dashboard status ' + resp.status }));
        }
    } catch (e) {
        console.log(JSON.stringify({ success: false, error: e.message }));
    }
}

// WASM solver
async function solveChallenge(token) {
    const fs = require('fs');
    const path = require('path');
    const vm = require('vm');

    const jsCode = fs.readFileSync(path.join(__dirname, 'challenge.v2.min.js'), 'utf-8');
    const wasmBuffer = fs.readFileSync(path.join(__dirname, 'challenge.v2.wasm'));

    return new Promise((resolve, reject) => {
        const fakePort = {
            onmessage: null,
            postMessage: () => {},
        };

        const sandbox = {
            console: { log: () => {}, error: () => {}, warn: () => {} },
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
                if (opts && opts.method === 'POST') {
                    // Intercept the challenge submission to get the solution
                    resolve(opts.headers['x-vercel-challenge-solution']);
                    return { ok: true, status: 204, headers: { get: () => null, has: () => false, getSetCookie: () => [] } };
                }
                return new Response(wasmBuffer, { status: 200, headers: { 'Content-Type': 'application/wasm' } });
            },
        };
        sandbox.self = sandbox;
        sandbox.globalThis = sandbox;

        let onmsgHandler = null;
        Object.defineProperty(sandbox, 'onmessage', {
            get: () => onmsgHandler,
            set: (v) => { onmsgHandler = v; },
        });

        const context = vm.createContext(sandbox);
        vm.runInContext(jsCode, context, { timeout: 5000 });

        if (!onmsgHandler) { reject(new Error('onmessage not set')); return; }

        onmsgHandler({ data: { port: fakePort } });

        if (!fakePort.onmessage) { reject(new Error('port.onmessage not set')); return; }

        fakePort.onmessage({
            data: { type: 'solve-request', token, version: '2' }
        });

        setTimeout(() => reject(new Error('WASM solver timeout')), 20000);
    });
}

main();
