// Oma-Test: frischer Datenordner, kein ffmpeg – nur klicken, bis der erste Film läuft.
const { chromium } = require('playwright-core');
const S = process.env.S, BASE = process.env.BASE, MEDIA = process.env.MEDIA;
const t0 = Date.now();
const log = [];
const step = (msg) => { const s = ((Date.now() - t0) / 1000).toFixed(1); log.push(`${s} s  ${msg}`); console.log(`${s} s  ${msg}`); };
const shot = (page, name) => page.screenshot({ path: `${S}/shots/${name}.png` });

(async () => {
  const browser = await chromium.launch({ executablePath: process.env.CHROME, args: ['--autoplay-policy=no-user-gesture-required'] });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  const errors = [];
  page.on('pageerror', (e) => errors.push('pageerror: ' + e.message));
  page.on('console', (m) => m.type() === 'error' && errors.push('console: ' + m.text()));
  try {
    await page.goto(BASE + '/');
    await page.waitForURL('**/setup');
    step('Startseite leitet auf die Einrichtung um');
    if (await page.isVisible('#ffearly')) step('ffmpeg-Download läuft schon im Hintergrund');
    await shot(page, '1-willkommen');

    await page.fill('#serverName', 'Wohnzimmer');
    await page.click('[data-step="0"] [data-next]');
    await page.fill('#name', 'Oma');
    await page.fill('#password', 'geheim');
    await page.fill('#password2', 'geheim');
    await shot(page, '2-konto');
    await page.click('[data-step="1"] [data-next]');
    step('Name und Konto eingegeben');

    await page.fill('#manual', MEDIA);
    await page.click('#go');
    await page.waitForSelector('#pick:not([hidden])');
    await page.click('#pick');
    await page.waitForFunction(() => /Videos/.test(document.querySelector('#chosen small')?.textContent || ''));
    step('Ordner gewählt: ' + await page.textContent('#chosen small'));
    await shot(page, '3-ordner');
    await page.click('[data-step="2"] [data-next]');

    await page.waitForFunction(() => !/Prüfe/.test(document.querySelector('#fftext').textContent));
    step('ffmpeg-Status: ' + await page.textContent('#fftext'));
    await shot(page, '4-ffmpeg-fehlt');
    if (await page.isVisible('#ffget')) {
      await page.click('#ffget');
      step('„Automatisch installieren“ geklickt');
    }
    await page.waitForFunction(() => /installiert/.test(document.querySelector('#fftext').textContent), null, { timeout: 15 * 60 * 1000 });
    step('ffmpeg installiert');
    await shot(page, '5-ffmpeg-ok');
    await page.click('#finish');
    await page.waitForURL((u) => !u.pathname.startsWith('/setup'), { timeout: 30000 });
    step('Einrichtung fertig, weiter zur Bibliothek');

    await page.waitForSelector('.card', { timeout: 5 * 60 * 1000 });
    await page.waitForTimeout(1500);
    step('Bibliothek zeigt Titel');
    await shot(page, '6-bibliothek');
    const titles = await page.$$eval('.card', (cs) => cs.map((c) => c.innerText.replace(/\s+/g, ' ').trim()));
    step('Karten: ' + titles.join(' | '));

    await page.locator('.card', { hasText: 'Blade Runner' }).first().click();
    await page.waitForSelector('text=Abspielen', { timeout: 30000 });
    step('Detailseite offen');
    await shot(page, '7-detail');
    await page.click('text=Abspielen');
    await page.waitForFunction(() => { const v = document.querySelector('video'); return v && v.currentTime > 2 && !v.paused; }, null, { timeout: 120000 });
    step('Film läuft (currentTime > 2 s)');
    await shot(page, '8-wiedergabe');
  } catch (e) {
    step('FEHLER: ' + e.message.split('\n')[0]);
    await shot(page, 'fehler').catch(() => {});
    process.exitCode = 1;
  }
  if (errors.length) console.log('Browser-Fehler:\n' + errors.slice(0, 10).join('\n'));
  await browser.close();
})();
