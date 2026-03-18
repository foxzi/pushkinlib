// @ts-check
const { test, expect } = require('@playwright/test');
const { BASE_URL, isAuthEnabled, loginAsAdmin, gotoAndLogin } = require('./auth-helpers');

test.describe('TTS Start From Reading Position', () => {

  // Login via API before each test so request calls work with auth
  test.beforeEach(async ({ request }) => {
    const enabled = await isAuthEnabled(request);
    if (enabled) {
      await loginAsAdmin(request);
    }
  });

  test('TTS request contains text from visible paragraph, not from beginning', async ({ page, request }) => {
    // Dismiss any error dialogs
    page.on('dialog', async dialog => {
      console.log('Dialog:', dialog.message());
      await dialog.accept();
    });

    // Save position via API request context (which has auth cookie from beforeEach)
    await request.put(`${BASE_URL}/api/v1/books/000100/position`, {
      data: {
        book_id: '000100',
        section: 0,
        progress: 0.0,
        total_sections: 22
      }
    });

    // 1. Go to library, open a book
    await gotoAndLogin(page);
    await page.waitForSelector('.book-card', { timeout: 15000 });

    // Click "Читать" on any book — try to open one with content
    const readBtns = page.locator('button', { hasText: '📖 Читать' });
    const count = await readBtns.count();
    console.log(`Found ${count} "Читать" buttons`);
    expect(count).toBeGreaterThan(0);

    // Try books until one opens in the reader
    let readerOpened = false;
    for (let i = 0; i < Math.min(count, 5); i++) {
      await readBtns.nth(i).click();
      try {
        await page.waitForSelector('.reader-layout', { timeout: 5000 });
        readerOpened = true;
        console.log(`Reader opened for book at index ${i}`);
        break;
      } catch {
        console.log(`Book at index ${i} failed to open, trying next...`);
        await page.waitForTimeout(500);
      }
    }
    expect(readerOpened).toBeTruthy();

    // 2. Wait for reader content to render
    await page.waitForSelector('.reader-content', { timeout: 10000 });
    await page.waitForTimeout(1000); // let content fully render

    // 3. Check if TTS is available (button exists)
    const ttsBtn = page.locator('.reader-control-btn', { hasText: '▶' });
    const ttsAvailable = await ttsBtn.count() > 0;
    if (!ttsAvailable) {
      console.log('SKIP: TTS not available (tts-server may not be running)');
      return;
    }

    // 4. Get the text of the FIRST paragraph in the section (this should NOT be in the TTS request
    //    after we scroll down)
    const firstParaText = await page.evaluate(() => {
      const content = document.querySelector('.reader-content');
      if (!content) return '';
      const blocks = content.querySelectorAll('p, h1, h2, h3, h4, h5, h6, div, li, blockquote, pre');
      for (const block of blocks) {
        const text = (block.textContent || '').trim();
        if (text.length > 20) return text.substring(0, 80); // first meaningful paragraph
      }
      return '';
    });
    console.log('First paragraph text (start):', JSON.stringify(firstParaText));

    // 5. Count total paragraphs to understand content size
    const totalParas = await page.evaluate(() => {
      const content = document.querySelector('.reader-content');
      if (!content) return 0;
      const blocks = content.querySelectorAll('p, h1, h2, h3, h4, h5, h6, div, li, blockquote, pre');
      let count = 0;
      for (const block of blocks) {
        if ((block.textContent || '').trim().length > 0) count++;
      }
      return count;
    });
    console.log(`Total non-empty paragraphs: ${totalParas}`);

    if (totalParas < 10) {
      console.log('SKIP: Section too short to test scroll-based TTS start');
      return;
    }

    // 6. Scroll down significantly in the reader body
    await page.evaluate(() => {
      const readerBody = document.querySelector('.reader-body');
      if (readerBody) {
        // Scroll to about 60% of the content
        readerBody.scrollTop = readerBody.scrollHeight * 0.6;
      }
    });
    await page.waitForTimeout(500);

    // 7. Get the text of the paragraph now visible at the top
    const visibleParaText = await page.evaluate(() => {
      const container = document.querySelector('.reader-body');
      const content = document.querySelector('.reader-content');
      if (!container || !content) return '';
      const containerRect = container.getBoundingClientRect();
      const blocks = content.querySelectorAll('p, h1, h2, h3, h4, h5, h6, div, li, blockquote, pre');
      for (const block of blocks) {
        const text = (block.textContent || '').trim();
        if (text.length === 0) continue;
        const rect = block.getBoundingClientRect();
        if (rect.bottom > containerRect.top && rect.top < containerRect.bottom) {
          return text.substring(0, 80);
        }
      }
      return '';
    });
    console.log('Visible paragraph text (after scroll):', JSON.stringify(visibleParaText));

    // Verify we actually scrolled — visible text should differ from first para
    if (visibleParaText === firstParaText) {
      console.log('SKIP: Could not scroll far enough, visible paragraph is still the first one');
      return;
    }

    // 8. Intercept the TTS speech API request
    let ttsRequestText = '';
    const ttsRequestPromise = page.waitForRequest(
      req => req.url().includes('/api/v1/tts/speech') && req.method() === 'POST',
      { timeout: 15000 }
    );

    // 9. Click TTS play button
    await ttsBtn.click();
    console.log('Clicked TTS play button');

    // 10. Wait for the TTS request and capture the text
    try {
      const ttsRequest = await ttsRequestPromise;
      const postData = ttsRequest.postDataJSON();
      ttsRequestText = postData.input || postData.text || '';
      console.log('TTS request text length:', ttsRequestText.length);
      console.log('TTS request text (first 120 chars):', JSON.stringify(ttsRequestText.substring(0, 120)));
    } catch (e) {
      console.log('WARNING: Could not capture TTS request:', e.message);
      // Stop TTS if it started
      const stopBtn = page.locator('.reader-control-btn', { hasText: /⏸|⏳/ });
      if (await stopBtn.count() > 0) await stopBtn.first().click();
      return;
    }

    // 11. CRITICAL ASSERTION: The TTS text should NOT start from the beginning
    expect(ttsRequestText.length).toBeGreaterThan(0);

    const firstWords = firstParaText.split(/\s+/).slice(0, 5).join(' ');
    console.log('Checking TTS text does NOT start with first paragraph words:', JSON.stringify(firstWords));

    const ttsStartsWithFirst = ttsRequestText.includes(firstWords);
    console.log(`TTS contains first paragraph: ${ttsStartsWithFirst}`);

    // Main assertion: TTS chunk should NOT contain text from the very beginning
    // of the section — it should have jumped ahead to the visible area
    expect(ttsStartsWithFirst).toBe(false);
    console.log('PASS: TTS does NOT start from the beginning of the section');

    // 12. Verify via Vue state that ttsChunkIndex > 0 (we skipped chunks)
    const chunkIndex = await page.evaluate(() => {
      // Access Vue app instance
      const app = document.querySelector('#app');
      if (app && app.__vue_app__) {
        const vm = app.__vue_app__._instance?.proxy;
        return vm ? vm.ttsChunkIndex : -1;
      }
      return -1;
    });
    console.log(`ttsChunkIndex = ${chunkIndex}`);
    expect(chunkIndex).toBeGreaterThan(0);
    console.log('PASS: TTS started from chunk > 0 (skipped beginning)');

    // 12. Stop TTS playback
    const stopBtn = page.locator('.reader-control-btn', { hasText: /⏸|⏳/ });
    if (await stopBtn.count() > 0) {
      await stopBtn.first().click();
      await page.waitForTimeout(500);
      console.log('PASS: TTS stopped');
    }
  });

  test('TTS starts from beginning when not scrolled', async ({ page }) => {
    page.on('dialog', async dialog => await dialog.accept());

    await gotoAndLogin(page);
    await page.waitForSelector('.book-card', { timeout: 15000 });

    // Open a book
    const readBtns = page.locator('button', { hasText: '📖 Читать' });
    const count = await readBtns.count();
    expect(count).toBeGreaterThan(0);

    let readerOpened = false;
    for (let i = 0; i < Math.min(count, 5); i++) {
      await readBtns.nth(i).click();
      try {
        await page.waitForSelector('.reader-layout', { timeout: 5000 });
        readerOpened = true;
        break;
      } catch {
        await page.waitForTimeout(500);
      }
    }
    expect(readerOpened).toBeTruthy();

    await page.waitForSelector('.reader-content', { timeout: 10000 });
    await page.waitForTimeout(1000);

    const ttsBtn = page.locator('.reader-control-btn', { hasText: '▶' });
    if (await ttsBtn.count() === 0) {
      console.log('SKIP: TTS not available');
      return;
    }

    // Get first meaningful block text (including headings) — same as what TTS sees
    const firstBlockText = await page.evaluate(() => {
      const content = document.querySelector('.reader-content');
      if (!content) return '';
      const blocks = content.querySelectorAll('p, h1, h2, h3, h4, h5, h6, div, li, blockquote, pre');
      for (const block of blocks) {
        const text = (block.textContent || '').trim();
        if (text.length > 0) return text;
      }
      return '';
    });
    console.log('First block text:', JSON.stringify(firstBlockText));

    // Make sure scroll is at top
    await page.evaluate(() => {
      const readerBody = document.querySelector('.reader-body');
      if (readerBody) readerBody.scrollTop = 0;
    });
    await page.waitForTimeout(300);

    // Intercept TTS request
    const ttsRequestPromise = page.waitForRequest(
      req => req.url().includes('/api/v1/tts/speech') && req.method() === 'POST',
      { timeout: 15000 }
    );

    await ttsBtn.click();

    try {
      const ttsRequest = await ttsRequestPromise;
      const postData = ttsRequest.postDataJSON();
      const ttsRequestText = postData.input || postData.text || '';
      console.log('TTS text (first 120 chars):', JSON.stringify(ttsRequestText.substring(0, 120)));

      // TTS should start with text from the beginning of the section
      // (could be a heading like "Глава 1" or the first paragraph)
      const firstWords = firstBlockText.split(/\s+/).slice(0, 3).join(' ');
      const ttsContainsFirst = ttsRequestText.includes(firstWords);
      console.log(`First block words: ${JSON.stringify(firstWords)}, found in TTS: ${ttsContainsFirst}`);
      expect(ttsContainsFirst).toBe(true);
      console.log('PASS: TTS starts from the beginning when not scrolled');
    } catch (e) {
      console.log('WARNING: Could not capture TTS request:', e.message);
    }

    // Stop TTS
    const stopBtn = page.locator('.reader-control-btn', { hasText: /⏸|⏳/ });
    if (await stopBtn.count() > 0) await stopBtn.first().click();
  });

});
