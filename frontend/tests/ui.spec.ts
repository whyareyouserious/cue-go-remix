import { test, expect, type Page } from '@playwright/test';
import { initialSettings } from '../src/types';

async function mockDesktop(page: Page, bounds?: { maxWidth: number; maxHeight: number }) {
  await page.addInitScript(({ settings, bounds }) => {
    const w = window as any;
    w.mockSettings = { ...settings, onboarded: true, chat: { ...settings.chat, hasKey: true } };
    w.mockCalls = [];
    w.mockBounds = bounds;
  }, { settings: initialSettings, bounds });
  await page.route('**/wails/runtime.js*', route => route.fulfill({ contentType: 'application/javascript', body: `
    export const Events = { On: (_, callback) => { window.cueEmit = data => callback({data}); return () => {}; } };
    export const Clipboard = { SetText: async text => { window.copied = text; } };
    export const Browser = { OpenURL: async url => { window.openedURL = url; } };
    export const Call = { ByName: async (name, ...args) => {
      const method = name.split('.').pop(); window.mockCalls.push({method,args});
      if(method === 'GetSettings') return window.mockSettings;
      if(method === 'SetOverlayLayout') return window.mockBounds;
      if(method === 'SaveSettings') { window.mockSettings = {...args[0]}; return window.mockSettings; }
      if(method === 'GetStatus') return {platform:'darwin',listening:false,busy:false,dataDir:'/tmp/cue-test',transcript:[],slides:[],warnings:[]};
      if(method.startsWith('List')) return [];
      if(method === 'Consent' && args[1] && !window.secondConsent) {
        window.secondConsent = true;
        window.cueEmit({type:'consent',id:'consent-2',text:'允许控制会话？',data:{caller:'测试助手',scope:'action'}});
      }
    }};
  ` }));
  await page.goto('/');
  await expect(page.locator('#connection-status')).toContainText('准备就绪');
}

test('从菜单条高度打开设置时不滚动页面，首次上报完整窗口高度', async ({ page }) => {
  await page.setViewportSize({ width: 700, height: 78 });
  await mockDesktop(page, { maxWidth: 1440, maxHeight: 900 });
  const toolbar = (await page.locator('#toolbar').boundingBox())!;
  await page.locator('#settings-toggle').click();
  // 原生窗口仍是收起高度时，聚焦弹窗按钮也不能滚动整张页面。
  expect(await page.evaluate(() => [window.scrollY, document.documentElement.scrollTop, document.body.scrollTop])).toEqual([0, 0, 0]);
  expect(await page.locator('#toolbar').boundingBox()).toEqual(toolbar);
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SetOverlayLayout').at(-1)?.args[1])).toBe(618);
  await page.setViewportSize({ width: 700, height: 618 });
  await expect(page.locator('#modal-close')).toBeFocused();
  await expect(page.locator('#save-settings')).toBeInViewport();
  expect(await page.locator('#toolbar').boundingBox()).toEqual(toolbar);
  await page.locator('#settings-toggle').click();
  await expect(page.locator('.modal')).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SetOverlayLayout').at(-1)?.args[1])).toBe(78);
});

for (const expanded of [false, true]) {
  test(`透明度弹层开关和调节时菜单条位置不变（${expanded ? '展开' : '收起'}）`, async ({ page }) => {
    await mockDesktop(page);
    if (expanded) await page.locator('#collapse-toggle').click();
    const toolbar = page.locator('#toolbar');
    const before = await toolbar.boundingBox();
    const panelBefore = expanded ? await page.locator('#panel').boundingBox() : null;
    await page.locator('#opacity-toggle').click();
    await expect(page.locator('#opacity-popover')).toBeVisible();
    // 记录整个淡入过程，防止只检查动画终点漏掉中途位移。
    const positions = await toolbar.evaluate(element => new Promise<{ x: number; y: number }[]>(resolve => {
      const samples: { x: number; y: number }[] = [];
      const start = performance.now();
      function sample(now: number) {
        const { x, y } = element.getBoundingClientRect();
        samples.push({ x, y });
        if (now - start < 220) requestAnimationFrame(sample);
        else resolve(samples);
      }
      requestAnimationFrame(sample);
    }));
    for (const position of positions) expect(position).toEqual({ x: before!.x, y: before!.y });
    if (panelBefore) expect(await page.locator('#panel').boundingBox()).toEqual(panelBefore);
    const popover = (await page.locator('#opacity-popover').boundingBox())!;
    expect(popover.y).toBeGreaterThanOrEqual(before!.y + before!.height);
    await expect.poll(() => page.evaluate(({ x, y, width, height }) => {
      const layout = (window as any).mockCalls.filter((call: any) => call.method === 'SetOverlayLayout').at(-1)?.args;
      return layout && layout[1] >= y + height && layout[2].some((region: any) =>
        region.x <= x && region.y <= y && region.x + region.width >= Math.floor(x + width) && region.y + region.height >= Math.floor(y + height));
    }, popover)).toBeTruthy();
    const slider = page.locator('#quick-opacity');
    await slider.focus();
    await slider.press('ArrowLeft');
    const value = Number(await slider.inputValue());
    await expect(page.locator('#opacity-value')).toHaveText(`${value}%`);
    await expect.poll(() => page.evaluate(() => (window as any).mockSettings.opacity)).toBe(value / 100);
    expect(await toolbar.boundingBox()).toEqual(before);
    await page.locator('#opacity-toggle').click();
    await expect(page.locator('#opacity-popover')).toBeHidden();
    expect(await toolbar.boundingBox()).toEqual(before);
  });
}

test('默认显示悬浮条，回答卡和转录历史按需展开', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/?preview=1');
  await expect(page.locator('#toolbar')).toBeVisible();
  await expect(page.locator('#workspace')).toBeHidden();
  expect(await page.locator('#toolbar').evaluate(el => el.getBoundingClientRect().height)).toBe(40);
  expect(await page.locator('#toolbar').evaluate(el => getComputedStyle(el).backgroundColor)).toBe('rgba(16, 12, 28, 0.7)');
  await page.screenshot({ path: 'test-results/preview-toolbar.png' });
  await page.locator('#collapse-toggle').click();
  await expect(page.locator('#preview-note')).toBeVisible();
  await expect(page.locator('#transcript-panel')).toBeHidden();
  expect(await page.locator('#panel').evaluate(el => el.getBoundingClientRect().width)).toBe(624);
  expect(await page.locator('#panel').evaluate(el => el.getBoundingClientRect().height)).toBeLessThan(320);
  await page.screenshot({ path: 'test-results/preview-expanded.png' });
  await page.locator('#sidebar-toggle').click();
  await expect(page.locator('#transcript-panel')).toBeVisible();
  await page.locator('#close-sidebar').click();
  await page.locator('#settings-toggle').click();
  await page.locator('[data-provider-value="custom"]').click();
  await page.locator('#endpoint-key').click();
  await expect(page.locator('.modal')).toBeVisible();
  await page.screenshot({ path: 'test-results/preview-settings.png' });
  for (const tab of ['speech', 'style', 'appearance']) {
    await page.locator(`[data-tab="${tab}"]`).click();
    await expect(page.locator('#settings-page')).toBeVisible();
  }
  await page.locator('[data-tab="speech"]').click();
  await page.locator('[data-settings-page="models"]').click();
  await expect(page.locator('#model-list')).toBeVisible();
  await page.locator('[data-tab="chat"]').click();
  await page.locator('[data-settings-page="access"]').click();
  await expect(page.locator('#caller-list')).toBeVisible();
  await page.locator('#modal-close').click();
  await page.setViewportSize({ width: 620, height: 700 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  await page.screenshot({ path: 'test-results/preview-compact.png' });
  expect(errors).toEqual([]);
});

test('更换中转地址不携带旧密钥，普通响应开关独立保存', async ({ page }) => {
  await mockDesktop(page);
  await page.locator('#settings-toggle').click();
  await page.locator('[data-provider-value="custom"]').click();
  await page.locator('#endpoint-key').click();
  await expect(page.locator('.modal')).toBeVisible();
  expect(await page.evaluate(() => (window as any).mockCalls.filter((c: any) => ['SaveSettings', 'Ask', 'Quit'].includes(c.method)).length)).toBe(0);
  await page.locator('#endpoint-url').fill('https://relay.example/api/v1');
  await page.locator('#endpoint-model').fill('my-fast-model');
  await page.locator('#endpoint-smart').fill('my-smart-model');
  await page.locator('#endpoint-stream').uncheck();
  await page.locator('#save-settings').click();
  const saved = await page.evaluate(() => (window as any).mockSettings);
  expect(saved.chat).toMatchObject({ provider: 'custom', baseUrl: 'https://relay.example/api/v1', model: 'my-fast-model', smartModel: 'my-smart-model', stream: false, apiKey: '', clearKey: true });
  expect(saved.speech.baseUrl).toBe(initialSettings.speech.baseUrl);
});

test('流式 Markdown 安全渲染，保留手工输入并允许针对转录提问', async ({ page }) => {
  await mockDesktop(page);
  await page.locator('#collapse-toggle').click();
  await page.locator('#question').fill('我正在写自己的问题');
  await page.evaluate(() => (window as any).cueEmit({ type: 'transcript', data: { id: 't1', channel: 'them', text: '怎样恢复会议？', time: Date.now() } }));
  await expect(page.locator('#question')).toHaveValue('我正在写自己的问题');
  await page.locator('#sidebar-toggle').click();
  await page.locator('[data-turn="t1"]').click();
  await page.locator('#send').click();
  const calls = await page.evaluate(() => (window as any).mockCalls);
  expect(calls.find((c: any) => c.method === 'Ask').args[0]).toEqual({ mode: 'answerThis', text: '怎样恢复会议？' });
  await page.evaluate(() => {
    const emit = (window as any).cueEmit;
    emit({ type: 'answer-start', id: 'a1', text: '怎样恢复会议？' });
    emit({ type: 'answer-token', id: 'a1', text: '**会议记录**会保存在本机。\n\n```go\nfmt.Println("Cue")\n```\n<img src=x onerror="window.xss=true"><script>window.xss=true</script><button data-model-download="large-v3">恶意操作</button><a href="https://example.com" data-mode="assist">外部链接</a>' });
    emit({ type: 'answer-done', id: 'a1' });
  });
  await expect(page.locator('.answer-content strong')).toHaveText('会议记录');
  await expect(page.locator('.answer-content img, .answer-content script, .answer-content [data-mode], .answer-content [data-model-download]')).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).xss)).toBeUndefined();
  await page.locator('.copy-code').click();
  expect(await page.evaluate(() => (window as any).copied)).toContain('fmt.Println');
  await page.screenshot({ path: 'test-results/answer-transcript.png' });
});

test('实时问题填入输入框，按 Enter 后才请求模型；答案展开且隐藏后回到工具条', async ({ page }) => {
  await mockDesktop(page);
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SetOverlayLayout').at(-1)?.args[1])).toBe(78);
  await page.evaluate(() => (window as any).cueEmit({ type: 'transcript', data: { id: 't1', channel: 'them', text: 'Go 如何处理并发？', time: Date.now() } }));
  await page.evaluate(() => (window as any).cueEmit({ type: 'transcript', data: { id: 't2', channel: 'them', text: '请举个例子。', time: Date.now() } }));
  await expect(page.locator('#workspace')).toBeHidden();
  expect(await page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'Ask').length)).toBe(0);
  await page.locator('#collapse-toggle').click();
  await expect(page.locator('#question')).toHaveValue('Go 如何处理并发？ 请举个例子。');
  await page.locator('#question').press('Enter');
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.find((c: any) => c.method === 'Ask')?.args[0])).toEqual({ mode: 'answerThis', text: 'Go 如何处理并发？ 请举个例子。' });
  await page.locator('#collapse-toggle').click();
  await page.evaluate(() => {
    (window as any).cueEmit({ type: 'answer-start', id: 'a2' });
    (window as any).cueEmit({ type: 'answer-token', id: 'a2', text: '使用 goroutine 和 channel。\n\n'.repeat(80) });
    (window as any).cueEmit({ type: 'answer-done', id: 'a2' });
  });
  await expect(page.locator('#workspace')).toBeVisible();
  await expect(page.locator('.answer-content')).toContainText('goroutine');
  expect(await page.locator('#messages').evaluate(el => el.getBoundingClientRect().height)).toBeLessThanOrEqual(300);
  await expect(page.locator('#toolbar')).toBeInViewport();
  await page.locator('#sidebar-toggle').click();
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SetOverlayLayout').at(-1)?.args[0])).toBe(1000);
  await page.locator('#collapse-toggle').click();
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SetOverlayLayout').at(-1)?.args[1])).toBe(78);
  const layout = await page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SetOverlayLayout').at(-1).args);
  expect(layout[0]).toBe(700);
  expect(layout[2]).toHaveLength(2);
});

test('连续授权请求不会被上一条响应关闭，Escape 拒绝当前请求', async ({ page }) => {
  await mockDesktop(page);
  await page.evaluate(() => (window as any).cueEmit({ type: 'consent', id: 'consent-1', text: '允许读取状态？', data: { caller: '测试助手', scope: 'read' } }));
  await page.locator('#consent-allow').click();
  await expect(page.locator('.consent-content')).toContainText('允许控制会话？');
  await page.keyboard.press('Escape');
  await expect(page.locator('.modal')).toHaveCount(0);
  const calls = await page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'Consent'));
  expect(calls.map((c: any) => c.args)).toEqual([['consent-1', true], ['consent-2', false]]);
});

test('900×480工作区内长回答、历史关闭和设置保存均可到达', async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 480 });
  await mockDesktop(page, { maxWidth: 900, maxHeight: 480 });
  await expect.poll(() => page.evaluate(() => document.documentElement.style.getPropertyValue('--screen-height'))).toBe('480px');
  await page.locator('#collapse-toggle').click();
  await page.evaluate(() => {
    const emit = (window as any).cueEmit;
    emit({ type: 'answer-start', id: 'small-screen' });
    emit({ type: 'answer-token', id: 'small-screen', text: '这是一段较长回答。\n\n'.repeat(100) });
    emit({ type: 'answer-done', id: 'small-screen' });
    emit({ type: 'interim', channel: 'them', text: '较长的实时转录文字。'.repeat(100) });
  });
  await page.locator('#sidebar-toggle').click();
  await expect(page.locator('#close-sidebar')).toBeInViewport();
  await expect(page.locator('#send')).toBeInViewport();
  expect(await page.locator('#transcript-panel').evaluate(el => el.getBoundingClientRect().right)).toBeLessThanOrEqual(900);
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SetOverlayLayout').at(-1)?.args[1])).toBeLessThanOrEqual(480);
  await page.locator('#close-sidebar').click();
  await page.locator('#settings-toggle').click();
  await expect(page.locator('#modal-close')).toBeInViewport();
  await expect(page.locator('#save-settings')).toBeInViewport();
  await page.locator('[data-provider-value="custom"]').click();
  await page.locator('#endpoint-model').fill('small-screen-model');
  await page.locator('#save-settings').click();
  await expect(page.locator('.modal')).toHaveCount(0);
  await page.screenshot({ path: 'test-results/small-screen.png' });
});

test('暂定字幕逐步填入且允许修正，最终句替换暂定文字并只在 Enter 后发送', async ({ page }) => {
  await mockDesktop(page);
  await page.locator('#collapse-toggle').click();
  await page.evaluate(() => {
    const emit = (window as any).cueEmit;
    emit({ type: 'status', data: { listening: true } });
    emit({ type: 'transcript', data: { id: 'live-1', channel: 'them', text: '先说明背景。', time: Date.now() } });
    emit({ type: 'interim', channel: 'them', text: '如' });
  });
  await expect(page.locator('#question')).toHaveValue('先说明背景。 如');
  await page.evaluate(() => (window as any).cueEmit({ type: 'interim', channel: 'them', text: '如何处里并发' }));
  await expect(page.locator('#question')).toHaveValue('先说明背景。 如何处里并发');
  await page.evaluate(() => (window as any).cueEmit({ type: 'interim', channel: 'them', text: '如何处理并发' }));
  await expect(page.locator('#question')).toHaveValue('先说明背景。 如何处理并发');
  await page.evaluate(() => {
    const emit = (window as any).cueEmit;
    emit({ type: 'transcript', data: { id: 'live-2', channel: 'them', text: '如何处理并发？', time: Date.now() } });
    emit({ type: 'interim', channel: 'them', text: '' });
  });
  await expect(page.locator('#question')).toHaveValue('先说明背景。 如何处理并发？');
  await expect(page.locator('#interim')).toBeEmpty();
  expect(await page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'Ask').length)).toBe(0);
  await page.locator('#question').press('Enter');
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.find((c: any) => c.method === 'Ask')?.args[0])).toEqual({ mode: 'answerThis', text: '先说明背景。 如何处理并发？' });
});

test('发送暂定问题后同一句不回填，下一句继续更新，手工编辑和麦克风文字受保护', async ({ page }) => {
  await mockDesktop(page);
  await page.locator('#collapse-toggle').click();
  await page.evaluate(() => (window as any).cueEmit({ type: 'interim', channel: 'them', text: '如何处理并发' }));
  await expect(page.locator('#question')).toHaveValue('如何处理并发');
  await page.locator('#question').press('Enter');
  await expect(page.locator('#question')).toHaveValue('');
  await page.evaluate(() => {
    const emit = (window as any).cueEmit;
    emit({ type: 'interim', channel: 'them', text: '如何处理并发请求' });
    emit({ type: 'transcript', data: { id: 'sent-1', channel: 'them', text: '如何处理并发请求？', time: Date.now() } });
  });
  await expect(page.locator('#question')).toHaveValue('');
  await page.evaluate(() => (window as any).cueEmit({ type: 'interim', channel: 'them', text: '再举一个例子' }));
  await expect(page.locator('#question')).toHaveValue('再举一个例子');
  await page.locator('#question').fill('我手动改好的问题');
  await page.evaluate(() => {
    const emit = (window as any).cueEmit;
    emit({ type: 'interim', channel: 'them', text: '再举一个详细的例子' });
    emit({ type: 'transcript', data: { id: 'sent-2', channel: 'them', text: '再举一个详细的例子。', time: Date.now() } });
    emit({ type: 'interim', channel: 'you', text: '这是我的麦克风声音' });
  });
  await expect(page.locator('#question')).toHaveValue('我手动改好的问题');
  await expect(page.locator('#interim')).toContainText('这是我的麦克风声音');
  expect(await page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'Ask').length)).toBe(1);
});

test('本地中文实时服务不需要 URL 或密钥并保存为流式', async ({ page }) => {
  await mockDesktop(page);
  await page.locator('#settings-toggle').click();
  await page.locator('[data-tab="speech"]').click();
  await page.getByRole('button', { name: '本地实时（中文）', exact: true }).click();
  await expect(page.locator('#endpoint-key')).toHaveCount(0);
  await expect(page.locator('#endpoint-url')).toHaveCount(0);
  await expect(page.locator('#settings-page')).toContainText('简体中文');
  await page.locator('#save-settings').click();
  expect(await page.evaluate(() => (window as any).mockSettings.speech)).toMatchObject({ provider: 'sherpa', stream: true, apiKey: '', baseUrl: '' });
});
