import { test, expect, type Page } from '@playwright/test';
import { initialSettings } from '../src/types';

async function mockDesktop(page: Page, bounds?: { maxWidth: number; maxHeight: number }) {
  await page.addInitScript(({ settings, bounds }) => {
    const w = window as any;
    const chat = { ...settings.chat, hasKey: true };
    const speech = { ...settings.speech, hasKey: true };
    w.mockSettings = { ...settings, onboarded: true, chat, speech,
      chatChannels: [{ ...chat, id: 'chat-main', name: '测试聊天账号', status: 'untested' }],
      speechChannels: [{ ...speech, id: 'speech-main', name: '测试语音账号', status: 'untested' }],
      activeChatChannelId: 'chat-main', activeSpeechChannelId: 'speech-main',
    };
    // 仅为隔离的桌面契约夹具：完整虚构密钥由后端 mock 持有，设置响应与 DOM 保持脱敏。
    w.mockKeys = { chat: { 'chat-main': 'fake-stored-chat-key' }, speech: { 'speech-main': 'fake-stored-speech-key' } };
    w.mockCalls = [];
    w.mockBounds = bounds;
    w.mockModels = [];
  }, { settings: initialSettings, bounds });
  await page.route('**/wails/runtime.js*', route => route.fulfill({ contentType: 'application/javascript', body: `
    export const Events = { On: (_, callback) => { window.cueEmit = data => callback({data}); return () => {}; } };
    export const Clipboard = { SetText: async text => { window.copied = text; } };
    export const Browser = { OpenURL: async url => { window.openedURL = url; } };
    export const Call = { ByName: async (name, ...args) => {
      if (!name.startsWith('cue-go-remix/internal/controller.App.')) throw new Error('Wails 绑定名称不匹配：' + name);
      const method = name.split('.').pop(); window.mockCalls.push({method,args:structuredClone(args)});
      if (/license|trial/i.test(method)) throw new Error('无卡密版不应请求激活或试用：' + method);
      if(method === 'GetSettings') return structuredClone(window.mockSettings);
      if(method === 'SetOverlayLayout') return window.mockBounds;
      if(method === 'SaveSettings') {
        const next = structuredClone(args[0]);
        for (const kind of ['chat', 'speech']) {
          const field = kind + 'Channels';
          const old = window.mockSettings[field] || [];
          const previousKeys = window.mockKeys[kind];
          const keys = {};
          next[field] = (next[field] || []).map(channel => {
            const previous = old.find(item => item.id === channel.id);
            const same = previous?.provider === channel.provider && previous?.baseUrl === channel.baseUrl;
            const key = channel.clearKey ? '' : channel.apiKey || (same ? previousKeys[channel.id] || '' : '');
            keys[channel.id] = key;
            return {...channel, apiKey:'', hasKey:!!key, clearKey:false};
          });
          window.mockKeys[kind] = keys;
          const activeID = next[kind === 'chat' ? 'activeChatChannelId' : 'activeSpeechChannelId'];
          const active = next[field].find(channel => channel.id === activeID);
          next[kind] = active ? {
            provider:active.provider, baseUrl:active.baseUrl, apiKey:'', hasKey:active.hasKey,
            chatProtocol:active.chatProtocol,
            clearKey:false, model:active.model, smartModel:active.smartModel || '', stream:active.stream,
          } : {provider:'',baseUrl:'',apiKey:'',hasKey:false,model:'',smartModel:'',stream:false};
          if(kind === 'speech' && activeID === 'local_sherpa') next.speech = {...next.speech,provider:'sherpa',model:'zipformer',stream:true};
          if(kind === 'speech' && activeID === 'local_whisper') next.speech = {...next.speech,provider:'local',model:next.whisperModel};
        }
        window.mockSettings = next;
        return structuredClone(next);
      }
      if(method === 'GetChannelKey') return window.mockKeys[args[0]][args[1]] || '';
      if(method === 'TestChannelEndpoint') return {success:true,latency:12,message:'隔离测试连接成功'};
      if(method === 'GetStatus') return {platform:'darwin',listening:false,busy:false,dataDir:'/tmp/cue-test',transcript:[],slides:[],warnings:[]};
      if(method === 'ListModels') return structuredClone(window.mockModels);
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

test('聊天协议在保存后生效，检测沿用渠道草稿且编辑不丢失密钥', async ({ page }) => {
  await mockDesktop(page);
  await page.locator('#settings-toggle').click();
  await page.locator('[data-edit-id="chat-main"]').click();
  await expect(page.locator('#new-channel-protocol')).toHaveValue('chat-completions');
  await page.locator('#new-channel-protocol').selectOption('responses');
  await page.locator('#add-channel-submit').click();
  await expect(page.locator('[data-channel-id="chat-main"] .mode-tag')).toHaveText('Responses');
  expect(await page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SaveSettings'))).toHaveLength(0);
  expect(await page.evaluate(() => (window as any).mockSettings.chat.chatProtocol)).toBeUndefined();

  await page.locator('[data-test-id="chat-main"]').click();
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'TestChannelEndpoint').at(-1)?.args[1]?.chatProtocol)).toBe('responses');
  await page.locator('[data-edit-id="chat-main"]').click();
  await expect(page.locator('#new-channel-protocol')).toHaveValue('responses');
  await page.locator('#new-channel-protocol').selectOption('chat-completions');
  await page.locator('#add-channel-cancel').click();
  await expect(page.locator('[data-channel-id="chat-main"] .mode-tag')).toHaveText('Responses');

  await page.locator('#save-settings').click();
  await expect(page.locator('.modal')).toHaveCount(0);
  const saved = await page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SaveSettings').at(-1).args[0]);
  expect(saved.chat.chatProtocol).toBe('responses');
  expect(saved.chatChannels[0].chatProtocol).toBe('responses');
  expect(saved.chatChannels[0].apiKey).toBe('');
  expect(saved.chatChannels[0].clearKey).toBe(false);
  expect(await page.evaluate(() => (window as any).mockKeys.chat['chat-main'])).toBe('fake-stored-chat-key');
  await page.locator('#settings-toggle').click();
  await page.locator('[data-edit-id="chat-main"]').click();
  await expect(page.locator('#new-channel-protocol')).toHaveValue('responses');
  await page.locator('[data-preset-id="anthropic"]').click();
  await expect(page.locator('#new-channel-protocol')).toHaveCount(0);
});

test('无卡密版直接进入工具条，模型配置后可直接提问', async ({ page }) => {
  await mockDesktop(page);
  await expect(page.locator('#toolbar')).toBeVisible();
  await expect(page.locator('#session-toggle')).toBeEnabled();
  await expect(page.locator('#license-panel, #license-status')).toHaveCount(0);
  await page.locator('#collapse-toggle').click();
  await page.locator('#question').fill('直接使用无卡密版');
  await page.locator('#send').click();
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.find((call: any) => call.method === 'Ask')?.args[0])).toEqual({ mode: 'ask', text: '直接使用无卡密版' });
  expect(await page.evaluate(() => (window as any).mockCalls.filter((call: any) => /license|trial/i.test(call.method)))).toEqual([]);
});

test('从菜单条高度打开设置时不滚动页面，首次上报完整窗口高度', async ({ page }) => {
  await page.setViewportSize({ width: 700, height: 78 });
  await mockDesktop(page, { maxWidth: 1440, maxHeight: 900 });
  const toolbar = (await page.locator('#toolbar').boundingBox())!;
  await page.locator('#settings-toggle').click();
  // 原生窗口仍是收起高度时，聚焦弹窗按钮也不能滚动整张页面。
  expect(await page.evaluate(() => [window.scrollY, document.documentElement.scrollTop, document.body.scrollTop])).toEqual([0, 0, 0]);
  expect(await page.locator('#toolbar').boundingBox()).toEqual(toolbar);

  // 验证首次上报契约：全屏透明幕布模式上报 -1，普通模式上报完整窗口高度
  const reportedArgs = await page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SetOverlayLayout').at(-1)?.args);
  const isFullscreenReq = reportedArgs[0] === -1 && reportedArgs[1] === -1;
  if (!isFullscreenReq) {
    expect(reportedArgs[1]).toBeGreaterThan(78);
  }
  const targetWidth = isFullscreenReq ? 1440 : reportedArgs[0];
  const targetHeight = isFullscreenReq ? 900 : reportedArgs[1];

  // 模拟宿主回授该尺寸
  await page.setViewportSize({ width: targetWidth, height: targetHeight });

  // 验证宿主回授后真实面板边界与操作项稳定可达
  const modalBox = (await page.locator('.modal').boundingBox())!;
  expect(modalBox.height).toBeGreaterThan(300);
  expect(modalBox.y + modalBox.height).toBeLessThanOrEqual(targetHeight);
  await expect(page.locator('#modal-close')).toBeFocused();
  await expect(page.locator('#save-settings')).toBeInViewport();
  expect(await page.locator('#toolbar').boundingBox()).toEqual(toolbar);

  // 关闭后窗口高度准确恢复收起高度
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
  await page.locator('#add-channel-btn').click();
  await page.locator('[data-preset-id="custom"]').click();
  await page.locator('#new-channel-key').click();
  await expect(page.locator('.modal')).toBeVisible();
  await page.screenshot({ path: 'test-results/preview-settings.png' });
  await page.locator('#add-channel-cancel').click();
  for (const tab of ['speech', 'style', 'appearance']) {
    await page.locator(`[data-tab="${tab}"]`).click();
    await expect(page.locator('#settings-page')).toBeVisible();
  }
  await page.locator('[data-tab="speech"]').click();
  await page.locator('#manage-local-models-btn').click();
  await expect(page.locator('.models-modal-overlay')).toBeVisible();
  await expect(page.locator('#model-list')).toBeVisible();
  await page.locator('#whisper-path').fill('/tmp/cue-test/whisper-server');
  await page.locator('#whisper-model').fill('small');
  await page.locator('#whisper-threads').fill('6');
  await page.locator('#models-modal-done').click();
  await expect(page.locator('.models-modal-overlay')).toHaveCount(0);
  await expect(page.locator('#speech-search')).toBeVisible();
  await page.locator('[data-activate-speech="local_whisper"]').first().click();
  await page.locator('[data-speech-id="local_whisper"]').getByRole('button', { name: '管理本地模型' }).click();
  await expect(page.locator('#whisper-path')).toHaveValue('/tmp/cue-test/whisper-server');
  await expect(page.locator('#whisper-model')).toHaveValue('small');
  await expect(page.locator('#whisper-threads')).toHaveValue('6');
  await page.locator('#models-modal-close').click();
  await expect(page.locator('.models-modal-overlay')).toHaveCount(0);
  await expect(page.locator('[data-activate-speech="local_whisper"]').first()).toBeChecked();
  await page.locator('[data-tab="chat"]').click();
  await page.locator('[data-settings-page="access"]').click();
  await expect(page.locator('#caller-list')).toBeVisible();
  await page.getByRole('button', { name: '返回密钥设置' }).click();
  await expect(page.locator('#channels-search')).toBeVisible();
  await page.locator('#modal-close').click();
  await page.setViewportSize({ width: 620, height: 700 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  await page.screenshot({ path: 'test-results/preview-compact.png' });
  expect(errors).toEqual([]);
});

test('模型管理子弹窗保留草稿与自动线程，关闭和确认按层级处理', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await mockDesktop(page);
  await page.evaluate(() => {
    // 只提供虚构的已安装模型，确认测试始终取消，不下载或删除模型。
    (window as any).mockModels = [{ id: 'fixture-base', bytes: 1024, sha256: '', installed: true, downloading: false, downloaded: 1024 }];
  });
  const calls = (method: string) => page.evaluate(name => (window as any).mockCalls.filter((call: any) => call.method === name), method);
  const overlay = page.locator('.models-modal-overlay');
  const whisperCard = page.locator('[data-speech-id="local_whisper"]');

  await page.locator('#settings-toggle').click();
  await page.locator('[data-tab="speech"]').click();
  await whisperCard.locator('input[data-activate-speech]').check();

  await test.step('完成仅保留草稿，零线程及新模型能重新显示', async () => {
    await page.locator('#manage-local-models-btn').click();
    await expect(overlay).toBeVisible();
    await page.locator('#whisper-path').fill('/tmp/cue-test/model-dialog/whisper-server');
    await page.locator('#whisper-model').fill('small');
    await page.locator('#whisper-threads').fill('0');
    await page.locator('#models-modal-done').click();
    await expect(overlay).toHaveCount(0);
    await expect(page.locator('#speech-search')).toBeVisible();
    await expect(whisperCard.locator('.model-badge-val')).toHaveText('small');
    expect(await calls('SaveSettings')).toEqual([]);

    const before = (await calls('ListModels')).length;
    await whisperCard.getByRole('button', { name: '管理本地模型' }).click();
    await expect(overlay).toHaveCount(1);
    await expect(page.locator('#model-list')).toHaveCount(1);
    await expect(page.locator('#speech-search')).toBeVisible();
    await expect.poll(async () => (await calls('ListModels')).length).toBe(before + 1);
    await expect(page.locator('#whisper-path')).toHaveValue('/tmp/cue-test/model-dialog/whisper-server');
    await expect(page.locator('#whisper-model')).toHaveValue('small');
    await expect(page.locator('#whisper-threads')).toHaveValue('0');
  });

  await test.step('Escape先取消删除确认，再仅关闭模型子窗', async () => {
    await page.locator('[data-model-delete="fixture-base"]').click();
    await expect(page.locator('.confirm-backdrop')).toBeVisible();
    // 确认按钮必须实际可点击，避免仅检测 DOM 可见却被模型遮罩覆盖。
    await page.locator('[data-confirm-cancel]').click({ trial: true });
    await page.keyboard.press('Escape');
    await expect(page.locator('.confirm-backdrop')).toHaveCount(0);
    await expect(overlay).toHaveCount(1);
    await expect(page.locator('.modal')).toHaveCount(1);
    expect(await calls('DeleteModel')).toEqual([]);

    await page.keyboard.press('Escape');
    await expect(overlay).toHaveCount(0);
    await expect(page.locator('.modal')).toBeVisible();
    await expect(page.locator('#speech-search')).toBeVisible();
    expect(await calls('SaveSettings')).toEqual([]);
  });

  await test.step('右上角关闭保留修改，底部保存后才生效', async () => {
    await page.locator('#manage-local-models-btn').click();
    await expect(page.locator('#whisper-threads')).toHaveValue('0');
    await page.locator('#whisper-model').fill('medium');
    await page.locator('#models-modal-close').click();
    await expect(overlay).toHaveCount(0);
    await expect(whisperCard.locator('.model-badge-val')).toHaveText('medium');
    expect(await calls('SaveSettings')).toEqual([]);
    await page.locator('#save-settings').click();
    await expect(page.locator('.modal')).toHaveCount(0);
    const savedCalls = await calls('SaveSettings');
    expect(savedCalls).toHaveLength(1);
    expect(savedCalls[0].args[0]).toMatchObject({
      whisperPath: '/tmp/cue-test/model-dialog/whisper-server', whisperModel: 'medium', whisperThreads: 0,
      activeSpeechChannelId: 'local_whisper',
    });
    await page.locator('#settings-toggle').click();
    await page.locator('[data-tab="speech"]').click();
    await page.locator('#manage-local-models-btn').click();
    await expect(page.locator('#whisper-path')).toHaveValue('/tmp/cue-test/model-dialog/whisper-server');
    await expect(page.locator('#whisper-model')).toHaveValue('medium');
    await expect(page.locator('#whisper-threads')).toHaveValue('0');
    await page.locator('#models-modal-close').click();
    await page.locator('#modal-close').click();
    await expect(page.locator('.models-modal-overlay, .confirm-backdrop')).toHaveCount(0);
  });
  expect(errors).toEqual([]);
});

test('更换中转地址不携带旧密钥，普通响应开关独立保存', async ({ page }) => {
  await mockDesktop(page);
  await page.locator('#settings-toggle').click();
  await page.locator('[data-edit-id="chat-main"]').click();
  await expect(page.locator('#new-channel-key')).toHaveValue('');
  await expect(page.locator('#channel-key-hint')).toContainText('已保存');
  await page.locator('[data-preset-id="custom"]').click();
  await page.locator('#new-channel-key').click();
  await expect(page.locator('.modal')).toBeVisible();
  expect(await page.evaluate(() => (window as any).mockCalls.filter((c: any) => ['SaveSettings', 'Ask', 'Quit'].includes(c.method)).length)).toBe(0);
  await page.locator('#new-channel-url').fill('https://relay.example/api/v1');
  await page.locator('#new-channel-model').fill('my-fast-model');
  await page.locator('#new-channel-smart').fill('my-smart-model');
  await page.locator('#new-channel-stream').uncheck();
  await page.locator('#add-channel-submit').click();
  await expect(page.locator('.add-channel-overlay')).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).mockSettings.chat.baseUrl)).toBe(initialSettings.chat.baseUrl);
  expect(await page.evaluate(() => (window as any).mockCalls.filter((c: any) => c.method === 'SaveSettings'))).toEqual([]);
  await page.locator('#save-settings').click();
  await expect(page.locator('.modal')).toHaveCount(0);
  const submitted = await page.evaluate(() => (window as any).mockCalls.find((c: any) => c.method === 'SaveSettings').args[0]);
  expect(submitted.chatChannels[0]).toMatchObject({ id: 'chat-main', apiKey: '', hasKey: false, clearKey: true });
  expect(JSON.stringify(submitted)).not.toContain('fake-stored-chat-key');
  const saved = await page.evaluate(() => (window as any).mockSettings);
  expect(saved.chat).toMatchObject({ provider: 'custom', baseUrl: 'https://relay.example/api/v1', model: 'my-fast-model', smartModel: 'my-smart-model', stream: false, apiKey: '', hasKey: false });
  expect(saved.speech.baseUrl).toBe(initialSettings.speech.baseUrl);
  expect(saved.speech.hasKey).toBe(true);
  await page.locator('#settings-toggle').click();
  await page.locator('[data-edit-id="chat-main"]').click();
  await expect(page.locator('#new-channel-url')).toHaveValue('https://relay.example/api/v1');
  await expect(page.locator('#new-channel-model')).toHaveValue('my-fast-model');
  await expect(page.locator('#new-channel-stream')).not.toBeChecked();
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
  await page.locator('[data-edit-id="chat-main"]').click();
  await expect(page.locator('#add-channel-submit')).toBeInViewport();
  await page.locator('#new-channel-model').fill('small-screen-model');
  await page.locator('#add-channel-submit').click();
  await page.locator('#save-settings').click();
  await expect(page.locator('.modal')).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).mockSettings.chat.model)).toBe('small-screen-model');
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
  const local = page.locator('[data-speech-id="local_sherpa"]');
  await expect(local).toContainText('未检测');
  await local.locator('input[data-activate-speech]').check();
  await expect(page.locator('#new-speech-key, #new-speech-url')).toHaveCount(0);
  await expect(local).toContainText('无需 API Key');
  expect(await page.evaluate(() => (window as any).mockSettings.speech.provider)).toBe('openai');
  await page.locator('#save-settings').click();
  await expect(page.locator('.modal')).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).mockSettings.speech)).toMatchObject({ provider: 'sherpa', stream: true, apiKey: '', baseUrl: '' });
  expect(await page.evaluate(() => (window as any).mockSettings.speechChannels.map((channel: any) => channel.id))).toEqual(['speech-main']);
  await page.locator('#settings-toggle').click();
  await page.locator('[data-tab="speech"]').click();
  await expect(page.locator('input[data-activate-speech="local_sherpa"]')).toBeChecked();
});
