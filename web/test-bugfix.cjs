// Quick test: send a message and check typing/All Users
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const BASE = 'http://localhost:5174';
const SHOTS = path.join(__dirname, 'screenshots');
if (!fs.existsSync(SHOTS)) fs.mkdirSync(SHOTS, { recursive: true });

async function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }
async function shot(page, name) {
  await page.screenshot({ path: path.join(SHOTS, `${name}.png`) });
  console.log(`  📸 ${name}`);
}

async function main() {
  const browser = await chromium.launch({ headless: true });
  const page = await (await browser.newContext({ viewport: { width: 1280, height: 800 }, colorScheme: 'dark' })).newPage();

  try {
    console.log('\n🧪 Bug Fix Verification\n');

    await page.goto(BASE, { waitUntil: 'networkidle' });
    await sleep(2000);

    // Click the first contact to open chat
    const contact = await page.$('.contact-item');
    if (contact) {
      await contact.click();
      await sleep(1000);

      // TEST 1: Type something and send a message
      console.log('1️⃣  Sending a message...');
      const input = await page.$('.compose-input');
      if (input) {
        await input.fill('Hello from Playwright! 🚀');
        await sleep(500);
        await shot(page, 'test_message_typed');

        // Check chat header for typing status
        const headerStatus = await page.$eval('.chat-header-status', el => el.textContent);
        console.log(`  -> Chat header status: "${headerStatus}"`);

        // Press Enter to send
        await input.press('Enter');
        await sleep(1500);
        await shot(page, 'test_message_sent');

        // Check the latest message
        const lastMsg = await page.$$eval('.message-bubble', els => {
          const last = els[els.length - 1];
          return last ? last.textContent : null;
        });
        console.log(`  -> Last message: "${lastMsg}"`);
        console.log(`  -> Message sent: ${lastMsg && lastMsg.includes('Hello from Playwright') ? 'YES ✅' : 'NO ❌'}`);
      }

      // TEST 2: Check the All Users tab loading text
      console.log('\n2️⃣  Testing All Users tab loading text...');
      const findBtn = await page.$('button:has-text("Find"), .btn-find');
      if (findBtn) {
        await findBtn.click();
        await sleep(500);

        const allTab = await page.$('button:has-text("All Users")');
        if (allTab) {
          await allTab.click();
          // Quickly capture the loading state
          await sleep(100);
          const loadingText = await page.$eval('.find-empty', el => el.textContent).catch(() => null);
          console.log(`  -> Loading text: "${loadingText}"`);
          console.log(`  -> Shows "Loading users" (not "Searching"): ${loadingText && loadingText.includes('Loading users') ? 'YES ✅' : loadingText === null ? 'SKIPPED (too fast)' : 'NO ❌'}`);

          await sleep(2000);
          await shot(page, 'test_all_users_loaded');

          // Check what users are shown
          const userItems = await page.$$('.find-user');
          console.log(`  -> Users listed: ${userItems.length}`);
        }

        const closeBtn = await page.$('.find-close, button:has-text("✕"), .btn-close');
        if (closeBtn) await closeBtn.click();
      }

      // TEST 3: Verify typing indicator is NOT showing when not expected
      console.log('\n3️⃣  Checking typing indicator...');
      const typingIndicator = await page.$('.typing-indicator');
      const typingText = typingIndicator ? await typingIndicator.textContent() : '';
      console.log(`  -> Typing indicator text: "${typingText}"`);
      console.log(`  -> No false typing: ${typingText === '' ? 'YES ✅' : 'NO ❌'}`);

    } else {
      console.log('  No contacts found');
    }

    await shot(page, 'test_final');
    console.log('\n✅ Bug fix verification complete\n');

  } catch (err) {
    console.error('Test failed:', err.message);
    await shot(page, 'test_error');
  } finally {
    await browser.close();
  }
}

main();
