// Dashboard helper using Playwright (headless Chrome)
// Solves Vercel challenge natively, completes onboarding, gets API keys
// Usage: node dashboard_helper.js <session-token>
// Outputs JSON: {"success":true,"apiKey":"uuid"} or {"success":false,"error":"msg"}

const sessionToken = process.argv[2];
if (!sessionToken) {
    console.log(JSON.stringify({ success: false, error: 'Usage: node dashboard_helper.js <session-token>' }));
    process.exit(1);
}

const DASHBOARD = 'https://dashboard.exa.ai';

async function main() {
    const { chromium } = require('playwright');

    let browser;
    try {
        browser = await chromium.launch({ headless: true });
        const context = await browser.newContext({
            userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
        });

        // Inject the session cookie
        await context.addCookies([{
            name: 'next-auth.session-token',
            value: sessionToken,
            domain: '.exa.ai',
            path: '/',
            httpOnly: true,
            secure: true,
            sameSite: 'Lax',
        }]);

        const page = await context.newPage();

        // Navigate to dashboard - Playwright's real Chrome will solve Vercel challenge
        process.stderr.write('Navigating to dashboard...\n');
        await page.goto(DASHBOARD + '/', { waitUntil: 'domcontentloaded', timeout: 30000 });

        // Wait for Vercel challenge to complete (it auto-redirects after solving)
        // Keep checking until we're past the challenge page
        for (let i = 0; i < 15; i++) {
            const title = await page.title();
            const url = page.url();
            process.stderr.write('  Check ' + (i+1) + ': title="' + title + '" url=' + url.substring(0, 60) + '\n');
            if (!title.includes('Vercel') && !title.includes('Security') && !title.includes('Checking')) {
                break;
            }
            await page.waitForTimeout(2000);
        }

        process.stderr.write('Page loaded, URL: ' + page.url() + '\n');

        // Complete onboarding via API call from the page context
        const onboardResult = await page.evaluate(async () => {
            const resp = await fetch('/api/onboarding/complete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    codingTool: 'claude',
                    framework: 'mcp',
                    useCase: 'coding-agent',
                    prompt: 'using in coding agent',
                    latencyProfile: 'auto',
                    contentType: 'compact',
                }),
            });
            return { status: resp.status, body: await resp.text() };
        });

        if (onboardResult.status !== 200) {
            console.log(JSON.stringify({ success: false, error: 'Onboarding returned ' + onboardResult.status + ': ' + onboardResult.body.substring(0, 200) }));
            return;
        }

        // Get API keys
        const keysResult = await page.evaluate(async () => {
            const resp = await fetch('/api/get-api-keys');
            return { status: resp.status, body: await resp.text() };
        });

        if (keysResult.status !== 200) {
            console.log(JSON.stringify({ success: false, error: 'Get API keys returned ' + keysResult.status + ': ' + keysResult.body.substring(0, 200) }));
            return;
        }

        const keysData = JSON.parse(keysResult.body);
        if (!keysData.apiKeys || keysData.apiKeys.length === 0) {
            console.log(JSON.stringify({ success: false, error: 'No API keys in response' }));
            return;
        }

        console.log(JSON.stringify({ success: true, apiKey: keysData.apiKeys[0].id }));
    } catch (e) {
        console.log(JSON.stringify({ success: false, error: e.message }));
    } finally {
        if (browser) await browser.close();
    }
}

main();
