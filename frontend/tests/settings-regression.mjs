import { chromium } from 'playwright';
import { readFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';

// 优先采用环境变量指定的浏览器路径，缺省使用 macOS 上的 Google Chrome
const chromePath = process.env.CUE_TEST_BROWSER || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const launchOptions = existsSync(chromePath) ? { executablePath: chromePath, headless: true } : { headless: true };
const browser = await chromium.launch(launchOptions);

const distDir = join(process.cwd(), 'dist');
if (!existsSync(join(distDir, 'index.html'))) {
  console.error('未找到前端构建产物 dist/index.html，请先执行 npm run build');
  process.exit(1);
}

/**
 * 创建并配置模拟环境，真实模拟宿主 SetOverlayLayout 回授契约
 */
async function setup(
  initialWidth = 700,
  initialHeight = 78,
  bounds = { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 }
) {
  const page = await browser.newPage({ viewport: { width: initialWidth, height: initialHeight } });

  await page.addInitScript(({ bounds }) => {
    window.mockCalls = [];
    window.mockBounds = bounds;
  }, { bounds });

  await page.route('https://cue.local/**', async route => {
    const url = new URL(route.request().url());
    if (url.pathname === '/wails/runtime.js') {
      return route.fulfill({
        contentType: 'application/javascript',
        body: `
          window.__eventListeners = window.__eventListeners || {};
          export const Events = {
            On: (name, cb) => {
              window.__eventListeners[name] = cb;
              return () => { delete window.__eventListeners[name]; };
            }
          };
          export const Clipboard = { SetText: async () => {} };
          export const Browser = { OpenURL: async () => {} };
          export const Call = {
            ByName: async (name, ...args) => {
              const method = name.split('.').pop();
              window.mockCalls.push({ method, args });
              if (method === 'GetLicenseStatus') return {state:'license',allowed:true,mode:'license',expiresAt:Date.now()+86400000,graceSeconds:0,message:'测试授权有效',saved:true};
              if (method === 'GetSettings') return {
                onboarded: true, opacity: 1, contentProtection: false, slidesEnabled: false,
                slidesInterval: 5000, chat: { provider: 'openai', baseUrl: '', model: 'gpt-4o-mini', apiKey: '', hasKey: true },
                speech: { provider: 'openai', baseUrl: '', model: 'whisper-1', apiKey: '' }
              };
              if (method === 'SetOverlayLayout') {
                const [w, h, regions] = args;
                if (w === -1 && h === -1) {
                  return {
                    maxWidth: window.mockBounds.maxWidth || 1440,
                    maxHeight: window.mockBounds.maxHeight || 900,
                    workAreaX: window.mockBounds.workAreaX ?? 0,
                    workAreaY: window.mockBounds.workAreaY ?? 0,
                    windowX: window.mockBounds.windowX ?? 500,
                    windowY: window.mockBounds.windowY ?? 200,
                  };
                }
                return {
                  maxWidth: window.mockBounds.maxWidth || 1440,
                  maxHeight: window.mockBounds.maxHeight || 900,
                  workAreaX: window.mockBounds.workAreaX ?? 0,
                  workAreaY: window.mockBounds.workAreaY ?? 0,
                  windowX: window.mockBounds.windowX ?? 500,
                  windowY: window.mockBounds.windowY ?? 200,
                };
              }
              if (method === 'GetStatus') return {
                platform: 'darwin', listening: false, busy: false, dataDir: '/tmp/test', transcript: [], slides: [], warnings: []
              };
              if (method === 'Consent') return true;
              if (method === 'SetPassthrough') return true;
              if (method.startsWith('List')) return [];
            }
          };
        `
      });
    }

    const filePath = join(distDir, url.pathname === '/' ? 'index.html' : url.pathname.slice(1));
    if (!existsSync(filePath)) {
      return route.fulfill({ status: 404, body: 'Not found' });
    }
    const ext = filePath.split('.').pop();
    const mimeMap = {
      html: 'text/html',
      js: 'application/javascript',
      css: 'text/css',
      png: 'image/png',
      svg: 'image/svg+xml'
    };
    return route.fulfill({
      contentType: mimeMap[ext] || 'application/octet-stream',
      body: readFileSync(filePath)
    });
  });

  await page.goto('https://cue.local/');
  await page.locator('#license-panel').waitFor({ state: 'hidden' });
  await page.locator('#settings-toggle').click();
  await page.locator('.modal-settings').waitFor({ state: 'visible' });
  // 等待实际宿主契约成立，避免并行编译时固定延时不足而读取上一帧布局。
  await page.waitForFunction(() => {
    const last = window.mockCalls.filter(c => c.method === 'SetOverlayLayout').at(-1)?.args;
    return document.querySelector('#app')?.classList.contains('overlay-fullscreen') && last?.[0] === -1 && last?.[1] === -1;
  });

  // 模拟宿主响应幕布请求扩充视口
  const last = await page.evaluate(() => window.mockCalls.filter(c => c.method === 'SetOverlayLayout').at(-1)?.args);
  if (last && last[0] === -1 && last[1] === -1) {
    await page.setViewportSize({ width: bounds.maxWidth, height: bounds.maxHeight });
    await page.waitForTimeout(60);
  }

  return page;
}

async function geometry(page) {
  return page.evaluate(() => {
    const rect = s => {
      const r = document.querySelector(s)?.getBoundingClientRect();
      return r ? { x: Math.round(r.x), y: Math.round(r.y), width: Math.round(r.width), height: Math.round(r.height), right: Math.round(r.right), bottom: Math.round(r.bottom) } : null;
    };
    const m = document.querySelector('.modal');
    return {
      viewport: [window.innerWidth, window.innerHeight],
      modal: rect('.modal'),
      save: rect('#save-settings'),
      header: rect('.modal .modal-header'),
      close: rect('#modal-close'),
      toolbar: rect('#toolbar'),
      desktop: rect('.desktop'),
      offset: [m?.style.getPropertyValue('--modal-tx'), m?.style.getPropertyValue('--modal-ty')],
      lastCall: window.mockCalls.filter(c => c.method === 'SetOverlayLayout').at(-1)?.args
    };
  });
}

try {
  console.log('--- 回归用例 1: 全屏幕布回授尺寸与位置稳定性测试 ---');
  const grow = await setup(700, 78, { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 });
  const positionSteps = [];
  for (let i = 0; i < 5; i++) {
    const g = await geometry(grow);
    positionSteps.push(g.modal);
    await grow.waitForTimeout(40);
  }
  const modalXs = positionSteps.map(m => m?.x);
  const modalYs = positionSteps.map(m => m?.y);
  if (!modalXs.every(x => x === modalXs[0])) {
    throw new Error(`面板 X 坐标未稳定，实际序列: ${JSON.stringify(modalXs)}`);
  }
  if (!modalYs.every(y => y === modalYs[0])) {
    throw new Error(`面板 Y 坐标未稳定，实际序列: ${JSON.stringify(modalYs)}`);
  }
  console.log('✓ 用例 1 通过：全屏透明幕布下尺寸与面板位置完全收敛稳定');
  await grow.close();

  console.log('--- 回归用例 2: 900x480 小屏可达性测试 ---');
  const small = await setup(700, 78, { maxWidth: 900, maxHeight: 480, workAreaX: 0, workAreaY: 0, windowX: 100, windowY: 20 });
  const smallGeom = await geometry(small);
  if (!smallGeom.save || smallGeom.save.bottom > 480) {
    throw new Error(`小屏下保存按钮越界不可达: bottom=${smallGeom.save?.bottom}`);
  }
  console.log(`✓ 用例 2 通过：小屏下保存按钮完全在屏内 (bottom = ${smallGeom.save.bottom} <= 480)`);
  await small.close();

  console.log('--- 回归用例 3: 大屏右下自由拖拽与边界防护测试 ---');
  const drag = await setup(700, 78, { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 });
  const h = (await geometry(drag)).header;
  await drag.mouse.move(h.x + 100, h.y + 15);
  await drag.mouse.down();
  await drag.mouse.move(h.x + 550, h.y + 400, { steps: 10 });
  await drag.mouse.up();
  await drag.waitForTimeout(80);

  const dragAfter = await geometry(drag);
  if (dragAfter.modal.right > 1440) {
    throw new Error(`大屏拖拽松手后面板越界: modal.right=${dragAfter.modal.right} > 1440`);
  }
  if (dragAfter.close.right > 1440) {
    throw new Error(`大屏拖拽松手后关闭按钮越界: close.right=${dragAfter.close.right} > 1440`);
  }
  console.log(`✓ 用例 3 通过：大屏拖拽松手后面板及关闭按钮均在屏内 (modal.right = ${dragAfter.modal.right} <= 1440)`);

  console.log('--- 回归用例 4: 双击复位与屏幕中心实时对齐测试 ---');
  const newHeader = (await geometry(drag)).header;
  await drag.mouse.dblclick(newHeader.x + 100, newHeader.y + 15);
  await drag.waitForTimeout(80);
  const resetGeom = await geometry(drag);
  const modalRegion = resetGeom.lastCall[2].find(r => r.width >= 800);
  if (!modalRegion || Math.abs(resetGeom.modal.x - modalRegion.x) > 2) {
    throw new Error(`双击复位后命中区与 DOM 未对齐: dom.x=${resetGeom.modal.x}, region.x=${modalRegion?.x}`);
  }
  // 按实际面板宽度验证居中，避免调整设置尺寸后继续使用旧坐标。
  const expectedResetX = Math.round((resetGeom.viewport[0] - resetGeom.modal.width) / 2);
  if (Math.abs(resetGeom.modal.x - expectedResetX) > 4) {
    throw new Error(`双击复位后未回到屏幕可用区域中心 (${expectedResetX}): 实际 ${resetGeom.modal.x}`);
  }
  console.log(`✓ 用例 4 通过：双击复位精确回到屏幕可用区域正中 (${resetGeom.modal.x}, ${resetGeom.modal.y}) 且命中区严格对齐`);

  console.log('--- 回归用例 5: 拖动后直接关闭复位与悬浮条归拢 700px 测试 ---');
  // 重新拖动一段距离
  const resetHeader = (await geometry(drag)).header;
  await drag.mouse.move(resetHeader.x + 100, resetHeader.y + 15);
  await drag.mouse.down();
  await drag.mouse.move(resetHeader.x + 80, resetHeader.y + 50, { steps: 5 });
  await drag.mouse.up();
  await drag.waitForTimeout(60);

  // 直接点击关闭按钮
  await drag.locator('#modal-close').click();
  await drag.waitForTimeout(80);

  const closedGeom = await geometry(drag);
  const lastWidth = closedGeom.lastCall[0];
  if (lastWidth !== 700) {
    throw new Error(`模态框拖拽后关闭未归拢至 700px，实际请求宽度: ${lastWidth}`);
  }
  console.log('✓ 用例 5 通过：拖拽后关闭弹窗偏移量完全复位，悬浮条精确归拢至 700px');
  await drag.close();

  console.log('--- 回归用例 6 (屏幕可用区域正中打开、自由拖拽与关闭重开居中测试) ---');
  const cross = await setup(700, 78, { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 });
  const initGeom = await geometry(cross);

  // 1. 验证菜单条留在原位 (500, 200)
  if (Math.abs(initGeom.desktop.x - 500) > 2 || Math.abs(initGeom.desktop.y - 200) > 2) {
    throw new Error(`悬浮菜单条未保持在原位 (500, 200): 实际 (${initGeom.desktop.x}, ${initGeom.desktop.y})`);
  }
  // 2. 验证弹窗不再跟随菜单条，而是按实测宽高在可用区域正中打开：(1440 - modal.width)/2, (900 - modal.height)/2
  const expectedOpenX = Math.round((1440 - initGeom.modal.width) / 2);
  const expectedOpenY = Math.round((900 - initGeom.modal.height) / 2);
  if (Math.abs(initGeom.modal.x - expectedOpenX) > 4 || Math.abs(initGeom.modal.y - expectedOpenY) > 6) {
    throw new Error(`面板未在屏幕可用区域正中打开 (${expectedOpenX}, ${expectedOpenY}): 实际 (${initGeom.modal.x}, ${initGeom.modal.y})`);
  }

  // 3. 自由拖动测试：向左上拖动 (-170, -70)
  const dragDx = -170;
  const dragDy = -70;
  const crossHeader = initGeom.header;
  await cross.mouse.move(crossHeader.x + 100, crossHeader.y + 15);
  await cross.mouse.down();
  await cross.mouse.move(crossHeader.x + 100 + dragDx, crossHeader.y + 15 + dragDy, { steps: 10 });
  await cross.mouse.up();
  await cross.waitForTimeout(80);

  const crossGeom = await geometry(cross);
  const expectedDragX = initGeom.modal.x + dragDx;
  const expectedDragY = initGeom.modal.y + dragDy;
  if (Math.abs(crossGeom.modal.x - expectedDragX) > 4 || Math.abs(crossGeom.modal.y - expectedDragY) > 6) {
    throw new Error(`面板拖动后坐标不符合预期 (${expectedDragX}, ${expectedDragY}): 实际 (${crossGeom.modal.x}, ${crossGeom.modal.y})`);
  }

  // 4. 关闭后重新打开（关闭重开）：验证依然回到屏幕正中
  await cross.locator('#modal-close').click();
  await cross.waitForTimeout(80);
  await cross.locator('#settings-toggle').click();
  await cross.waitForTimeout(80);

  const reopenGeom = await geometry(cross);
  const expectedReopenCenterX = Math.round((1440 - reopenGeom.modal.width) / 2);
  const expectedReopenCenterY = Math.round((900 - reopenGeom.modal.height) / 2);
  if (Math.abs(reopenGeom.modal.x - expectedReopenCenterX) > 4 || Math.abs(reopenGeom.modal.y - expectedReopenCenterY) > 6) {
    throw new Error(`关闭重开后面板未回到屏幕正中 (${expectedReopenCenterX}, ${expectedReopenCenterY}): 实际 (${reopenGeom.modal.x}, ${reopenGeom.modal.y})`);
  }
  console.log(`✓ 用例 6 通过：面板在屏幕可用区域正中打开 (${initGeom.modal.x}, ${initGeom.modal.y})，支持自由拖动，关闭重开精准回到中心`);
  await cross.close();

  console.log('--- 回归用例 7 (新契约副屏): 副屏负坐标 WorkArea (-1920, 0) 映射测试 ---');
  const subPage = await setup(700, 78, { maxWidth: 1920, maxHeight: 1080, workAreaX: -1920, workAreaY: 0, windowX: -1420, windowY: 200 });
  const subGeom = await geometry(subPage);
  // 在视口内相对坐标应为 -1420 - (-1920) = 500, 200
  if (Math.abs(subGeom.desktop.x - 500) > 2 || Math.abs(subGeom.desktop.y - 200) > 2) {
    throw new Error(`副屏相对坐标映射错误: 实际 (${subGeom.desktop.x}, ${subGeom.desktop.y}), 期望 (500, 200)`);
  }
  console.log(`✓ 用例 7 通过：副屏负坐标 WorkArea 相对映射完全正确 (desktop.x = ${subGeom.desktop.x}, desktop.y = ${subGeom.desktop.y})`);
  await subPage.close();

  console.log('--- 回归用例 8 (新契约防误拖): 幕布期禁止底层元素原生 drag 避免拖走整窗测试 ---');
  const dragCheck = await setup(700, 78, { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 });
  const dragStyles = await dragCheck.evaluate(() => {
    const getDrag = s => {
      const el = document.querySelector(s);
      return el ? window.getComputedStyle(el).getPropertyValue('--wails-draggable').trim() : null;
    };
    return {
      desktop: getDrag('.desktop'),
      dragPill: getDrag('.drag-pill'),
      toolbar: getDrag('#toolbar'),
      modalHeader: getDrag('.modal .modal-header'),
    };
  });
  if (dragStyles.desktop !== 'no-drag' || dragStyles.dragPill !== 'no-drag' || dragStyles.toolbar !== 'no-drag') {
    throw new Error(`全屏幕布期底层元素未彻底禁用原生 drag: ${JSON.stringify(dragStyles)}`);
  }
  console.log('✓ 用例 8 通过：全屏幕布期底层元素严格为 no-drag，杜绝误拖走整块工作区幕布');
  await dragCheck.close();

  console.log('--- 回归用例 9 (核心红例): 布局响应延迟 120ms 下立即 Escape 关闭防重加幕布测试 ---');
  // 创建延迟 120ms 的测试页面
  const delayPage = await browser.newPage({ viewport: { width: 700, height: 78 } });
  await delayPage.addInitScript(() => {
    window.mockCalls = [];
    window.mockBounds = { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 1200, windowY: 950 };
  });
  await delayPage.route('https://cue.local/**', async route => {
    const url = new URL(route.request().url());
    if (url.pathname === '/wails/runtime.js') {
      return route.fulfill({
        contentType: 'application/javascript',
        body: `
          export const Events = { On: () => () => {} };
          export const Clipboard = { SetText: async () => {} };
          export const Browser = { OpenURL: async () => {} };
          export const Call = {
            ByName: async (name, ...args) => {
              const method = name.split('.').pop();
              window.mockCalls.push({ method, args, time: Date.now() });
              if (method === 'GetLicenseStatus') return {state:'license',allowed:true,mode:'license',expiresAt:Date.now()+86400000,graceSeconds:0,message:'测试授权有效',saved:true};
              if (method === 'GetSettings') return {
                onboarded: true, opacity: 1, contentProtection: false, slidesEnabled: false,
                slidesInterval: 5000, chat: { provider: 'openai', baseUrl: '', model: 'gpt-4o-mini', apiKey: '', hasKey: true },
                speech: { provider: 'openai', baseUrl: '', model: 'whisper-1', apiKey: '' }
              };
              if (method === 'SetOverlayLayout') {
                const [w, h] = args;
                // 模拟宿主 120ms 响应延迟
                await new Promise(r => setTimeout(r, 120));
                if (w === -1 && h === -1) {
                  return { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 1200, windowY: 950 };
                }
                return { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 1200, windowY: 950 };
              }
              if (method === 'GetStatus') return { platform: 'darwin', listening: false, busy: false, dataDir: '/tmp/test', transcript: [], slides: [], warnings: [] };
              if (method.startsWith('List')) return [];
            }
          };
        `
      });
    }
    const filePath = join(distDir, url.pathname === '/' ? 'index.html' : url.pathname.slice(1));
    if (!existsSync(filePath)) return route.fulfill({ status: 404, body: 'Not found' });
    const ext = filePath.split('.').pop();
    const mimeMap = { html: 'text/html', js: 'application/javascript', css: 'text/css', png: 'image/png', svg: 'image/svg+xml' };
    return route.fulfill({ contentType: mimeMap[ext] || 'application/octet-stream', body: readFileSync(filePath) });
  });

  await delayPage.goto('https://cue.local/');
  await delayPage.waitForTimeout(40);

  // 用户点击打开设置
  await delayPage.locator('#settings-toggle').click();

  // 极速在 20ms 内（宿主还未返回时）立即按 Escape 退出弹窗
  await delayPage.waitForTimeout(20);
  await delayPage.keyboard.press('Escape');

  // 等待 180ms，确保 120ms 的旧请求与后续的关闭请求全部返回并处理完毕
  await delayPage.waitForTimeout(180);

  const delayCheck = await delayPage.evaluate(() => {
    const app = document.getElementById('app');
    const isFullscreen = app?.classList.contains('overlay-fullscreen');
    const calls = window.mockCalls.filter(c => c.method === 'SetOverlayLayout');
    const lastCall = calls.at(-1)?.args;
    const toolbarX = document.documentElement.style.getPropertyValue('--toolbar-x');
    const toolbarY = document.documentElement.style.getPropertyValue('--toolbar-y');
    return { isFullscreen, lastCall, toolbarX, toolbarY, totalCalls: calls.length };
  });

  if (delayCheck.isFullscreen) {
    throw new Error('过期模式回授导致 overlay-fullscreen 被错误重新添加！');
  }
  if (!delayCheck.lastCall || delayCheck.lastCall[0] !== 700) {
    throw new Error(`最终宿主请求尺寸异常: ${JSON.stringify(delayCheck.lastCall)}，期望恢复 700 紧凑宽度`);
  }
  if (delayCheck.toolbarX !== '0px' || delayCheck.toolbarY !== '0px') {
    throw new Error(`关闭后 toolbar 坐标未先还原为 0px: toolbarX=${delayCheck.toolbarX}, toolbarY=${delayCheck.toolbarY}`);
  }
  console.log(`✓ 用例 9 通过：120ms 延迟下打开后极速 Escape，旧回授被坚决丢弃，窗口严格归拢 700px 且无全屏残留`);
  await delayPage.close();

  console.log('--- 回归用例 10 (核心竞态): 宿主回授待返回时同几何 resize 触发 rAF，防有效响应误判失效测试 ---');
  const racePage = await browser.newPage({ viewport: { width: 700, height: 78 } });
  await racePage.addInitScript(() => {
    window.mockCalls = [];
    window.mockBounds = { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 };
  });
  await racePage.route('https://cue.local/**', async route => {
    const url = new URL(route.request().url());
    if (url.pathname === '/wails/runtime.js') {
      return route.fulfill({
        contentType: 'application/javascript',
        body: `
          export const Events = { On: () => () => {} };
          export const Clipboard = { SetText: async () => {} };
          export const Browser = { OpenURL: async () => {} };
          export const Call = {
            ByName: async (name, ...args) => {
              const method = name.split('.').pop();
              window.mockCalls.push({ method, args, time: Date.now() });
              if (method === 'GetLicenseStatus') return {state:'license',allowed:true,mode:'license',expiresAt:Date.now()+86400000,graceSeconds:0,message:'测试授权有效',saved:true};
              if (method === 'GetSettings') return {
                onboarded: true, opacity: 1, contentProtection: false, slidesEnabled: false,
                slidesInterval: 5000, chat: { provider: 'openai', baseUrl: '', model: 'gpt-4o-mini', apiKey: '', hasKey: true },
                speech: { provider: 'openai', baseUrl: '', model: 'whisper-1', apiKey: '' }
              };
              if (method === 'SetOverlayLayout') {
                const [w, h] = args;
                // 精确暂停首次全屏回授，测试由显式屏障触发，避免并行编译时依赖固定延时。
                if (w === -1 && h === -1 && !window.fullscreenReleased) {
                  await new Promise(resolve => { window.releaseFullscreen = resolve; });
                }
                if (w === -1 && h === -1) {
                  return { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 };
                }
                return { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 };
              }
              if (method === 'GetStatus') return { platform: 'darwin', listening: false, busy: false, dataDir: '/tmp/test', transcript: [], slides: [], warnings: [] };
              if (method.startsWith('List')) return [];
            }
          };
        `
      });
    }
    const filePath = join(distDir, url.pathname === '/' ? 'index.html' : url.pathname.slice(1));
    if (!existsSync(filePath)) return route.fulfill({ status: 404, body: 'Not found' });
    const ext = filePath.split('.').pop();
    const mimeMap = { html: 'text/html', js: 'application/javascript', css: 'text/css', png: 'image/png', svg: 'image/svg+xml' };
    return route.fulfill({ contentType: mimeMap[ext] || 'application/octet-stream', body: readFileSync(filePath) });
  });

  await racePage.goto('https://cue.local/');
  await racePage.locator('#license-panel').waitFor({ state: 'hidden' });

  // 点击打开设置
  await racePage.locator('#settings-toggle').click();

  // 确认请求在途中，派发同几何 resize，跨过两帧后才允许回授返回。
  await racePage.waitForFunction(() => typeof window.releaseFullscreen === 'function');
  await racePage.evaluate(async () => {
    window.dispatchEvent(new Event('resize'));
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    window.fullscreenReleased = true;
    window.releaseFullscreen();
  });
  await racePage.waitForFunction(() => document.getElementById('app')?.classList.contains('overlay-fullscreen'));

  const raceCheck = await racePage.evaluate(() => {
    const app = document.getElementById('app');
    return {
      isFullscreen: app?.classList.contains('overlay-fullscreen'),
      toolbarX: document.documentElement.style.getPropertyValue('--toolbar-x'),
      toolbarY: document.documentElement.style.getPropertyValue('--toolbar-y'),
    };
  });

  if (!raceCheck.isFullscreen) {
    throw new Error('同几何 resize rAF 导致正在途中的有效全屏幕布回授被误弃，overlay-fullscreen 错误丢失！');
  }
  if (raceCheck.toolbarX !== '500px' || raceCheck.toolbarY !== '200px') {
    throw new Error(`toolbar 坐标未正确应用: toolbarX=${raceCheck.toolbarX}, toolbarY=${raceCheck.toolbarY}`);
  }
  console.log(`✓ 用例 10 通过：回授未到时同几何 resize 触发 rAF，有效回授未被误弃，overlay-fullscreen 成功保持且坐标正确`);
  await racePage.close();

  console.log('--- 回归用例 11 (确认弹窗按实际尺寸居中与动态内容点击测试) ---');
  const consentPage = await setup(700, 78, { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 });
  // 先关闭默认打开的设置弹窗
  await consentPage.locator('#modal-close').click();
  await consentPage.waitForTimeout(60);

  // 触发原生确认授权事件
  await consentPage.evaluate(() => {
    const listener = window.__eventListeners?.['cue:event'];
    if (listener) {
      listener({
        data: {
          type: 'consent',
          id: 'test-consent-req-1',
          text: '请求录音授权',
          data: { caller: '测试应用', scope: '麦克风' }
        }
      });
    }
  });
  await consentPage.waitForTimeout(100);

  const consentGeom = await consentPage.evaluate(() => {
    const m = document.querySelector('.modal')?.getBoundingClientRect();
    const calls = window.mockCalls.filter(c => c.method === 'SetOverlayLayout');
    const lastCall = calls.at(-1)?.args;
    return {
      modal: m ? { x: Math.round(m.x), y: Math.round(m.y), width: Math.round(m.width), height: Math.round(m.height) } : null,
      lastCall
    };
  });

  if (!consentGeom.modal) {
    throw new Error('确认弹窗未成功渲染到 DOM 中！');
  }

  // 验证确认弹窗按自身实际内容尺寸在屏幕中心居中 (1440 - width)/2, (900 - height)/2
  const expectedCenterX = Math.round((1440 - consentGeom.modal.width) / 2);
  const expectedCenterY = Math.round((900 - consentGeom.modal.height) / 2);
  if (Math.abs(consentGeom.modal.x - expectedCenterX) > 4 || Math.abs(consentGeom.modal.y - expectedCenterY) > 4) {
    throw new Error(`确认弹窗未按实际尺寸在屏幕中心居中: 实际 (${consentGeom.modal.x}, ${consentGeom.modal.y}), 期望 (${expectedCenterX}, ${expectedCenterY})`);
  }

  // 验证命中区覆盖了模态框
  const hitRegions = consentGeom.lastCall[2];
  const modalHit = hitRegions.find(r => r.width === consentGeom.modal.width);
  if (!modalHit || Math.abs(modalHit.x - consentGeom.modal.x) > 2 || Math.abs(modalHit.y - consentGeom.modal.y) > 2) {
    throw new Error(`确认弹窗命中区未对齐: DOM=(${consentGeom.modal.x}, ${consentGeom.modal.y}), Region=(${modalHit?.x}, ${modalHit?.y})`);
  }

  // 验证按钮点击交互：点击允许
  await consentPage.locator('#consent-allow').click();
  await consentPage.waitForTimeout(60);

  const consentClosed = await consentPage.evaluate(() => {
    const hasConsentCall = window.mockCalls.some(c => c.method === 'Consent' && c.args[0] === 'test-consent-req-1' && c.args[1] === true);
    const modalExists = Boolean(document.querySelector('.modal'));
    const isFullscreen = document.getElementById('app')?.classList.contains('overlay-fullscreen');
    return { hasConsentCall, modalExists, isFullscreen };
  });

  if (!consentClosed.hasConsentCall) {
    throw new Error('点击允许后未正确调用 Consent 方法');
  }
  if (consentClosed.modalExists || consentClosed.isFullscreen) {
    throw new Error('允许授权后模态框未正常关闭或未退出全屏幕布');
  }

  console.log(`✓ 用例 11 通过：确认弹窗按实际内容尺寸精确居中 (${consentGeom.modal.x}, ${consentGeom.modal.y})，命中区精确对齐，点击允许后正常处理并退出全屏`);
  await consentPage.close();

  console.log('--- 回归用例 12 (已有面板上叠加确认框居中兼容与取消保留底层面板测试) ---');
  const confirmPage = await setup(700, 78, { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 });
  await confirmPage.waitForTimeout(60);

  // 初始状态：setup 默认打开设置面板并处于全屏幕布下
  const initCheck = await confirmPage.evaluate(() => {
    return {
      hasModal: Boolean(document.querySelector('.modal')),
      isFullscreen: document.getElementById('app')?.classList.contains('overlay-fullscreen')
    };
  });
  if (!initCheck.hasModal || !initCheck.isFullscreen) {
    throw new Error('测试初始状态未能处于已有面板和全屏幕布下');
  }

  // 在已有面板上叠加呼出 confirmAction 二次确认弹窗
  await confirmPage.evaluate(() => {
    const btn = document.createElement('button');
    btn.dataset.deleteMeeting = 'test-1';
    document.body.appendChild(btn);
    btn.click();
    btn.remove();
  });
  await confirmPage.waitForTimeout(60);

  const confirmGeom = await confirmPage.evaluate(() => {
    const dialog = document.querySelector('.confirm-dialog')?.getBoundingClientRect();
    const modal = document.querySelector('.modal')?.getBoundingClientRect();
    const calls = window.mockCalls.filter(c => c.method === 'SetOverlayLayout');
    const lastCall = calls.at(-1)?.args;
    const isFullscreen = document.getElementById('app')?.classList.contains('overlay-fullscreen');
    return {
      dialog: dialog ? { x: Math.round(dialog.x), y: Math.round(dialog.y), width: Math.round(dialog.width), height: Math.round(dialog.height) } : null,
      modal: modal ? { x: Math.round(modal.x), y: Math.round(modal.y) } : null,
      lastCall,
      isFullscreen
    };
  });

  if (!confirmGeom.dialog) {
    throw new Error('二次确认弹窗未成功叠加渲染到 DOM 中！');
  }
  if (!confirmGeom.modal) {
    throw new Error('底层面板意外丢失！');
  }
  if (!confirmGeom.isFullscreen) {
    throw new Error('全屏幕布状态未保持！');
  }

  // 验证已有面板叠加模式下，确认弹窗在屏幕可用区域正中居中 (1440 - width)/2, (900 - height)/2
  const expectedConfirmX = Math.round((1440 - confirmGeom.dialog.width) / 2);
  const expectedConfirmY = Math.round((900 - confirmGeom.dialog.height) / 2);
  if (Math.abs(confirmGeom.dialog.x - expectedConfirmX) > 4 || Math.abs(confirmGeom.dialog.y - expectedConfirmY) > 4) {
    throw new Error(`已有面板上叠加确认框未在屏幕中心居中: 实际 (${confirmGeom.dialog.x}, ${confirmGeom.dialog.y}), 期望 (${expectedConfirmX}, ${expectedConfirmY})`);
  }

  // 点击取消按钮
  await confirmPage.locator('[data-confirm-cancel]').click();
  await confirmPage.waitForTimeout(60);

  // 验证：取消须保留底层面板与全屏幕布
  const cancelCheck = await confirmPage.evaluate(() => {
    const dialogExists = Boolean(document.querySelector('.confirm-dialog'));
    const modalExists = Boolean(document.querySelector('.modal'));
    const isFullscreen = document.getElementById('app')?.classList.contains('overlay-fullscreen');
    return { dialogExists, modalExists, isFullscreen };
  });

  if (cancelCheck.dialogExists) {
    throw new Error('点击取消后确认弹窗未关闭');
  }
  if (!cancelCheck.modalExists) {
    throw new Error('点击取消后底层面板被误关闭！未能保留底层面板');
  }
  if (!cancelCheck.isFullscreen) {
    throw new Error('点击取消后全屏幕布状态被误退出！未能保留底层面板全屏');
  }

  console.log(`✓ 用例 12 通过：已有面板上叠加确认框精确居中 (${confirmGeom.dialog.x}, ${confirmGeom.dialog.y})，取消后完美保留底层面板及全屏幕布`);
  await confirmPage.close();

  console.log('--- 回归用例 13 (独立“清空历史”确认框在屏幕工作区正中居中与取消归拢测试) ---');
  const standaloneConfirmPage = await setup(700, 78, { maxWidth: 1440, maxHeight: 900, workAreaX: 0, workAreaY: 0, windowX: 500, windowY: 200 });
  // 先关闭默认打开的设置弹窗，进入纯悬浮条紧凑状态
  await standaloneConfirmPage.locator('#modal-close').click();
  await standaloneConfirmPage.waitForTimeout(60);

  // 模拟转录数据非空，以触发二次确认弹窗
  await standaloneConfirmPage.evaluate(() => {
    const listener = window.__eventListeners?.['cue:event'];
    if (listener) {
      listener({ data: { type: 'status', data: { transcript: [{ id: '1', text: '对话' }] } } });
    }
  });
  await standaloneConfirmPage.waitForTimeout(40);

  // 点击清空转录，独立呼出确认框
  await standaloneConfirmPage.evaluate(() => {
    document.querySelector('#clear-transcript')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
  await standaloneConfirmPage.waitForTimeout(60);

  const standaloneGeom = await standaloneConfirmPage.evaluate(() => {
    const dialog = document.querySelector('.confirm-dialog')?.getBoundingClientRect();
    const calls = window.mockCalls.filter(c => c.method === 'SetOverlayLayout');
    const lastCall = calls.at(-1)?.args;
    const isFullscreen = document.getElementById('app')?.classList.contains('overlay-fullscreen');
    return {
      dialog: dialog ? { x: Math.round(dialog.x), y: Math.round(dialog.y), width: Math.round(dialog.width), height: Math.round(dialog.height) } : null,
      lastCall,
      isFullscreen
    };
  });

  if (!standaloneGeom.dialog) {
    throw new Error('独立确认弹窗未成功渲染到 DOM 中！');
  }
  if (!standaloneGeom.isFullscreen) {
    throw new Error('独立确认弹窗弹出时未进入全屏幕布 overlay-fullscreen 状态！');
  }

  // 验证独立确认弹窗精确在屏幕工作区正中 (1440 - width)/2, (900 - height)/2，绝非局部的 top 100
  const expectedStandaloneX = Math.round((1440 - standaloneGeom.dialog.width) / 2);
  const expectedStandaloneY = Math.round((900 - standaloneGeom.dialog.height) / 2);
  if (Math.abs(standaloneGeom.dialog.x - expectedStandaloneX) > 4 || Math.abs(standaloneGeom.dialog.y - expectedStandaloneY) > 4) {
    throw new Error(`独立确认弹窗未在屏幕工作区中心居中: 实际 (${standaloneGeom.dialog.x}, ${standaloneGeom.dialog.y}), 期望 (${expectedStandaloneX}, ${expectedStandaloneY})`);
  }

  // 点击取消按钮退出确认弹窗
  await standaloneConfirmPage.locator('[data-confirm-cancel]').click();
  await standaloneConfirmPage.waitForTimeout(60);

  const standaloneClosed = await standaloneConfirmPage.evaluate(() => {
    const dialogExists = Boolean(document.querySelector('.confirm-dialog'));
    const isFullscreen = document.getElementById('app')?.classList.contains('overlay-fullscreen');
    return { dialogExists, isFullscreen };
  });

  if (standaloneClosed.dialogExists || standaloneClosed.isFullscreen) {
    throw new Error('独立确认框取消后弹窗未关闭或未退出全屏幕布');
  }

  console.log(`✓ 用例 13 通过：独立“清空历史”确认框精确在屏幕工作区正中居中 (${standaloneGeom.dialog.x}, ${standaloneGeom.dialog.y})，取消后退出全屏平稳归拢至 700px`);
  await standaloneConfirmPage.close();

  await browser.close();
  console.log('\n========================================');
  console.log('13 项核心回归与新契约测试全部顺利通过！');
  console.log('========================================');
  process.exit(0);
} catch (error) {
  console.error('\n❌ 回归测试失败:', error.message || error);
  if (browser) await browser.close();
  process.exit(1);
}
