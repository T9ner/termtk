// Test Sprint 2 features: date dividers, last message preview, search
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
    console.log('\n🧪 Sprint 2 Feature Verification\n');

    await page.goto(BASE, { waitUntil: 'networkidle' });
    await sleep(2000);

    // TEST 1: Last message preview in sidebar
    console.log('1️⃣  Last message preview in sidebar...');
    const preview = await page.$('.contact-preview');
    if (preview) {
      const previewText = await preview.textContent();
      console.log(`  -> Preview text: "${previewText}"`);
      console.log(`  -> Has preview: YES ✅`);
    } else {
      console.log(`  -> No .contact-preview found ❌`);
    }
    await shot(page, 'sprint2_sidebar_preview');

    // Click first contact
    const contact = await page.$('.contact-item');
    if (contact) {
      await contact.click();
      await sleep(1500);

      // TEST 2: Date dividers
      console.log('\n2️⃣  Date dividers...');
      const dividers = await page.$$('.date-divider');
      console.log(`  -> Date dividers found: ${dividers.length}`);
      if (dividers.length > 0) {
        const dividerText = await dividers[0].textContent();
        console.log(`  -> First divider: "${dividerText.trim()}"`);
        console.log(`  -> Has dividers: YES ✅`);
      }
      await shot(page, 'sprint2_date_dividers');

      // TEST 3: Ctrl+F search
      console.log('\n3️⃣  Chat search (Ctrl+F)...');
      await page.keyboard.down('Control');
      await page.keyboard.press('f');
      await page.keyboard.up('Control');
      await sleep(500);

      const searchBar = await page.$('.chat-search-bar');
      if (searchBar) {
        console.log(`  -> Search bar opened: YES ✅`);
        const searchInput = await page.$('.chat-search-input');
        if (searchInput) {
          await searchInput.fill('Hello');
          await sleep(300);
          
          const countEl = await page.$('.chat-search-count');
          if (countEl) {
            const countText = await countEl.textContent();
            console.log(`  -> Match count: "${countText}"`);
          }
          
          // Check for highlighted text
          const marks = await page.$$('mark');
          console.log(`  -> Highlighted matches: ${marks.length}`);
          
          await shot(page, 'sprint2_search_highlight');
        }

        // Close search
        const closeBtn = await page.$('.chat-search-close');
        if (closeBtn) await closeBtn.click();
        await sleep(300);
      } else {
        console.log(`  -> Search bar NOT found ❌`);
      }

      // Send a new message to test message preview updates
      console.log('\n4️⃣  Sending test message...');
      const input = await page.$('.compose-input');
      if (input) {
        await input.fill('Sprint 2 test message!');
        await input.press('Enter');
        await sleep(1500);
        
        // Check the sidebar preview updated
        const updatedPreview = await page.$('.contact-preview');
        if (updatedPreview) {
          const text = await updatedPreview.textContent();
          console.log(`  -> Updated preview: "${text}"`);
        }
      }
      await shot(page, 'sprint2_message_sent');
    }

    await shot(page, 'sprint2_final');
    console.log('\n✅ Sprint 2 verification complete\n');

  } catch (err) {
    console.error('Test failed:', err.message);
    await shot(page, 'sprint2_error');
  } finally {
    await browser.close();
  }
}

main();
