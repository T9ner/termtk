// Nod Visual Feature Test
// Usage: node test-features.cjs

const { chromium } = require('playwright');
const path = require('path');

const BASE = 'http://localhost:5174';
const SCREENSHOT_DIR = path.join(__dirname, 'screenshots');

async function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function screenshot(page, name) {
  await page.screenshot({ 
    path: path.join(SCREENSHOT_DIR, `${name}.png`),
    fullPage: false 
  });
  console.log(`  📸 ${name}.png`);
}

async function main() {
  const fs = require('fs');
  if (!fs.existsSync(SCREENSHOT_DIR)) fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

  console.log('\n🚀 Starting Nod Visual Feature Test\n');
  
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ 
    viewport: { width: 1280, height: 800 },
    colorScheme: 'dark'
  });
  const page = await context.newPage();

  try {
    // TEST 1: Initial Load
    console.log('1️⃣  Testing: App Load');
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await sleep(2000);
    await screenshot(page, '01_initial_load');

    // Check if we need to register
    const registerInput = await page.$('input[placeholder*="username" i], input[placeholder*="name" i], input[type="text"]');
    const hasRegisterBtn = await page.$('button:has-text("Get Started"), button:has-text("Register"), button:has-text("Start")');
    
    if (registerInput && hasRegisterBtn) {
      console.log('  -> Registration screen detected');
      await screenshot(page, '02_registration_screen');
      await registerInput.fill('TestUser_Visual');
      await screenshot(page, '03_registration_filled');
      await hasRegisterBtn.click();
      await sleep(2000);
      await screenshot(page, '04_after_registration');
    } else {
      console.log('  -> Already registered, skipping');
      await screenshot(page, '02_main_screen');
    }

    // TEST 2: Main Interface
    console.log('\n2️⃣  Testing: Main Interface');
    await sleep(1000);
    await screenshot(page, '05_main_interface');
    const sidebar = await page.$('.sidebar');
    console.log(`  -> Sidebar: ${sidebar ? 'FOUND' : 'MISSING'}`);
    const gearBtn = await page.$('.btn-settings, button[title="Settings"]');
    console.log(`  -> Settings button: ${gearBtn ? 'FOUND' : 'MISSING'}`);

    // TEST 3: Settings Modal
    console.log('\n3️⃣  Testing: Settings Modal');
    if (gearBtn) {
      await gearBtn.click();
      await sleep(500);
      await screenshot(page, '06_settings_modal');
      const profileSection = await page.$('.settings-section');
      console.log(`  -> Settings panel: ${profileSection ? 'FOUND' : 'MISSING'}`);
      const usernameInput = await page.$('.settings-input');
      console.log(`  -> Username input: ${usernameInput ? 'FOUND' : 'MISSING'}`);
      const toggleSwitch = await page.$('.toggle-switch');
      console.log(`  -> Toggle switch: ${toggleSwitch ? 'FOUND' : 'MISSING'}`);
      const relayStatus = await page.$('.settings-status');
      console.log(`  -> Relay status: ${relayStatus ? 'FOUND' : 'MISSING'}`);

      if (usernameInput) {
        console.log('\n  Testing: Username Change');
        await usernameInput.fill('');
        await usernameInput.fill('NodTester');
        await screenshot(page, '07_username_change');
        const saveBtn = await page.$('.btn-save');
        if (saveBtn) {
          await saveBtn.click();
          await sleep(1000);
          await screenshot(page, '08_username_saved');
        }
      }
      const closeBtn = await page.$('.btn-close, button:has-text("\\u2715")');
      if (closeBtn) await closeBtn.click();
      await sleep(500);
    } else {
      console.log('  MISSING: Settings button not found');
    }

    // TEST 4: Find People
    console.log('\n4️⃣  Testing: Find People Modal');
    const findBtn = await page.$('button:has-text("Find"), .btn-find');
    if (findBtn) {
      await findBtn.click();
      await sleep(1000);
      await screenshot(page, '09_find_people_modal');
      const allTab = await page.$('button:has-text("All Users")');
      console.log(`  -> All Users tab: ${allTab ? 'FOUND' : 'MISSING'}`);
      const onlineTab = await page.$('button:has-text("Online")');
      console.log(`  -> Online tab: ${onlineTab ? 'FOUND' : 'MISSING'}`);
      if (allTab) {
        await allTab.click();
        await sleep(1500);
        await screenshot(page, '10_all_users_tab');
      }
      const closeFindBtn = await page.$('.find-close, button:has-text("\\u2715"), .btn-close');
      if (closeFindBtn) await closeFindBtn.click();
      await sleep(500);
    }

    // TEST 5: Contact Management
    console.log('\n5️⃣  Testing: Contact Management');
    const contacts = await page.$$('.contact-item');
    console.log(`  -> Contacts found: ${contacts.length}`);
    if (contacts.length > 0) {
      await contacts[0].hover();
      await sleep(500);
      await screenshot(page, '11_contact_hover');
      const contactMenuBtn = await page.$('.btn-contact-menu');
      console.log(`  -> Contact menu button: ${contactMenuBtn ? 'FOUND' : 'MISSING'}`);
      if (contactMenuBtn) {
        await contactMenuBtn.click();
        await sleep(500);
        await screenshot(page, '12_contact_dropdown');
        const deleteOption = await page.$('.contact-dropdown-item.danger');
        console.log(`  -> Delete option: ${deleteOption ? 'FOUND' : 'MISSING'}`);
        if (deleteOption) {
          await deleteOption.click();
          await sleep(500);
          await screenshot(page, '13_delete_confirmation');
          const cancelBtn = await page.$('.btn-cancel');
          if (cancelBtn) await cancelBtn.click();
          await sleep(300);
        }
      }
      await contacts[0].click();
      await sleep(1000);
      await screenshot(page, '14_chat_open');
    }

    // TEST 6: Chat Features
    console.log('\n6️⃣  Testing: Chat Features');
    const messages = await page.$$('.message-bubble-wrap, .message-bubble');
    console.log(`  -> Messages visible: ${messages.length}`);
    if (messages.length > 0) {
      await messages[0].hover();
      await sleep(500);
      const reactBtn = await page.$('.btn-react-trigger');
      console.log(`  -> Reaction trigger: ${reactBtn ? 'FOUND' : 'MISSING'}`);
      if (reactBtn) {
        await reactBtn.click();
        await sleep(500);
        await screenshot(page, '15_reaction_picker');
        const emojiButtons = await page.$$('.reaction-picker-btn');
        console.log(`  -> Emoji buttons: ${emojiButtons.length}`);
        if (emojiButtons.length > 0) {
          await emojiButtons[0].click();
          await sleep(1000);
          await screenshot(page, '16_reaction_added');
        }
      }
      console.log('\n  Testing: Context Menu');
      await messages[0].click({ button: 'right' });
      await sleep(500);
      const contextMenu = await page.$('.msg-context-menu');
      console.log(`  -> Context menu: ${contextMenu ? 'FOUND' : 'MISSING'}`);
      if (contextMenu) {
        await screenshot(page, '17_context_menu');
        await page.click('body');
        await sleep(300);
      }
      const msgStatus = await page.$('.msg-status');
      console.log(`  -> Message status: ${msgStatus ? 'FOUND' : 'MISSING'}`);
      const encBadge = await page.$('.encrypted-badge');
      console.log(`  -> Encryption badge: ${encBadge ? 'FOUND' : 'N/A (unencrypted)'}`);
    }

    // TEST 7: Toast
    console.log('\n7️⃣  Testing: Toast Container');
    const toastContainer = await page.$('.toast-container');
    console.log(`  -> Toast container: ${toastContainer ? 'FOUND' : 'MISSING'}`);

    // TEST 8: Mobile Responsive
    console.log('\n8️⃣  Testing: Mobile Responsive');
    await screenshot(page, '18_desktop_layout');
    await page.setViewportSize({ width: 375, height: 812 });
    await sleep(500);
    await screenshot(page, '19_mobile_layout');
    const backBtn = await page.$('.btn-back');
    const backBtnVisible = backBtn ? await backBtn.isVisible() : false;
    console.log(`  -> Back button visible: ${backBtnVisible ? 'YES' : 'NO'}`);
    if (backBtnVisible) {
      await backBtn.click();
      await sleep(500);
      await screenshot(page, '20_mobile_contacts');
    }
    await page.setViewportSize({ width: 1280, height: 800 });
    await sleep(500);

    // FINAL
    await screenshot(page, '21_final_overview');
    console.log('\n============================');
    console.log('Visual test complete!');
    console.log(`Screenshots: ${SCREENSHOT_DIR}`);
    console.log('============================\n');
    
  } catch (err) {
    console.error('\nTest failed:', err.message);
    await screenshot(page, 'error_state');
  } finally {
    await browser.close();
  }
}

main();
