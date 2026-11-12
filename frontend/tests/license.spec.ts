import { test, expect, type Page } from '@playwright/test';
import { initialSettings, type LicenseStatus } from '../src/types';

const inactive: LicenseStatus = { state: 'inactive', allowed: false, mode: '', expiresAt: 0, graceSeconds: 0, message: '输入 Cue 卡密或领取试用', saved: false };
const licensed: LicenseStatus = { state: 'license', allowed: true, mode: 'license', expiresAt: 1893456000000, graceSeconds: 0, message: '授权有效', saved: true, maskedKey: 'CUE-****-TEST' };
const trial: LicenseStatus = { ...licensed, state: 'trial', mode: 'trial', maskedKey: undefined, message: '试用有效' };

async function mockLicenseDesktop(page: Page, status = inactive, hold: string[] = [], bounds = { maxWidth: 1440, maxHeight: 900 }) {
  await page.addInitScript(({ settings, status, hold, bounds, licensed, trial, inactive }) => {
    const w = window as any;
    w.mockSettings = { ...settings, onboarded: true, chat: { ...settings.chat, hasKey: true } };
    w.mockLicense = status;
    w.mockCalls = [];
    w.mockHolds = hold;
    w.mockPending = {};
    w.mockErrors = {};
    w.mockBounds = bounds;
    w.mockResponses = { ActivateLicense: licensed, StartTrial: trial, RetryLicense: licensed, ForgetLicense: inactive };
  }, { settings: initialSettings, status, hold, bounds, licensed, trial, inactive });
  await page.route('**/wails/runtime.js*', route => route.fulfill({ contentType: 'application/javascript', body: `
    export const Events = { On: (_, callback) => {
      window.cueEmit = data => callback({data});
      return () => {};
    }};
    export const Clipboard = { SetText: async () => {} };
    export const Browser = { OpenURL: async () => {} };
    export const Call = { ByName: async (name, ...args) => {
      if (!name.startsWith('cue-go-remix/internal/controller.App.')) throw new Error('Wails 绑定名称不匹配：' + name);
      const method = name.split('.').pop();
      window.mockCalls.push({method, args});
      if (window.mockHolds.includes(method)) return new Promise((resolve, reject) => { window.mockPending[method] = {resolve, reject}; });
      if (window.mockErrors[method]) throw new Error(window.mockErrors[method]);
      if (method === 'GetSettings') return window.mockSettings;
      if (method === 'GetStatus') return {platform:'darwin',listening:false,busy:false,dataDir:'/tmp/cue-license-ui-test',transcript:[],slides:[],warnings:[]};
      if (method === 'GetLicenseStatus') return {...window.mockLicense};
      if (method === 'SetOverlayLayout') return window.mockBounds;
      if (method === 'SaveSettings') return window.mockSettings = {...args[0]};
      if (method in window.mockResponses) return {...window.mockResponses[method]};
      if (method.startsWith('List')) return [];
    }};
  ` }));
  await page.goto('/');
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.some((call: any) => call.method === 'GetLicenseStatus'))).toBeTruthy();
}

async function calls(page: Page, methods: string[]) {
  return page.evaluate(methods => (window as any).mockCalls.filter((call: any) => methods.includes(call.method)), methods);
}

async function expectNoAutomaticWork(page: Page) {
  expect(await calls(page, ['StartListening', 'Ask', 'CaptureSlide', 'StartSlideCapture', 'ClearTranscript'])).toEqual([]);
}

async function emitLicense(page: Page, status: LicenseStatus) {
  await page.evaluate(status => (window as any).cueEmit({ type: 'license-status', data: status }), status);
}

test('启动检查完成前保持激活面板，禁用授权操作且不采集或发送问题', async ({ page }) => {
  await mockLicenseDesktop(page, inactive, ['GetLicenseStatus']);
  await expect(page.locator('#license-panel')).toBeVisible();
  await expect(page.locator('#toolbar')).toBeHidden();
  await expect(page.locator('#license-message')).toContainText('正在检查');
  for (const id of ['#license-activate', '#license-trial', '#license-retry']) await expect(page.locator(id)).toBeDisabled();
  await page.locator('#license-form').dispatchEvent('submit');
  await page.keyboard.press('Meta+Shift+A');
  await expectNoAutomaticWork(page);
  expect(await calls(page, ['ActivateLicense', 'StartTrial', 'RetryLicense'])).toEqual([]);
  await page.evaluate(status => (window as any).mockPending.GetLicenseStatus.resolve(status), inactive);
  await expect(page.locator('#license-trial')).toBeEnabled();
  await expect(page.locator('#toolbar')).toBeHidden();
});

test('试用只有主动点击才领取，验证中忽略重复提交，成功后也不自动开始会话', async ({ page }) => {
  await mockLicenseDesktop(page, inactive, ['StartTrial']);
  await expect(page.locator('#license-trial')).toBeEnabled();
  expect(await calls(page, ['StartTrial'])).toEqual([]);
  await page.locator('#license-trial').click();
  for (const id of ['#license-activate', '#license-trial', '#license-retry']) await expect(page.locator(id)).toBeDisabled();
  await expect(page.locator('#license-panel')).toHaveAttribute('aria-busy', 'true');
  await page.locator('#license-form').dispatchEvent('submit');
  expect(await calls(page, ['StartTrial', 'ActivateLicense'])).toHaveLength(1);
  await expectNoAutomaticWork(page);
  await page.evaluate(status => (window as any).mockPending.StartTrial.resolve(status), trial);
  await expect(page.locator('#toolbar')).toBeVisible();
  await expect(page.locator('#license-panel')).toBeHidden();
  await expect(page.locator('#session-toggle')).toContainText('开始会话');
  await expectNoAutomaticWork(page);
});

test('错误卡展示原因并保留输入，成功验证才显示工具条并清空卡密', async ({ page }) => {
  await mockLicenseDesktop(page);
  await page.evaluate(() => { (window as any).mockErrors.ActivateLicense = '卡密不属于 Cue'; });
  await page.locator('#license-key').fill('TEST-ONLY-WRONG-PRODUCT');
  await page.locator('#license-key').press('Enter');
  await expect(page.locator('#license-message')).toHaveText('卡密不属于 Cue');
  await expect(page.locator('#license-key')).toHaveValue('TEST-ONLY-WRONG-PRODUCT');
  await expect(page.locator('#toolbar')).toBeHidden();
  await expect(page.locator('#license-activate')).toBeEnabled();
  await expect(page.locator('#license-panel')).toHaveAttribute('aria-busy', 'false');
  await page.evaluate(() => { delete (window as any).mockErrors.ActivateLicense; });
  await page.locator('#license-key').fill('TEST-ONLY-VALID-CUE');
  await page.locator('#license-activate').click();
  await expect(page.locator('#toolbar')).toBeVisible();
  await expect(page.locator('#license-key')).toHaveValue('');
  expect((await calls(page, ['ActivateLicense'])).map((call: any) => call.args)).toEqual([['TEST-ONLY-WRONG-PRODUCT'], ['TEST-ONLY-VALID-CUE']]);
  expect(await page.locator('body').innerText()).not.toContain('TEST-ONLY-VALID-CUE');
  await expectNoAutomaticWork(page);
});

test('授权失效返回激活面板，重新验证保留问题和回答且不自动恢复采集', async ({ page }) => {
  await mockLicenseDesktop(page, licensed);
  await page.locator('#collapse-toggle').click();
  await page.locator('#question').fill('尚未发送的会议问题');
  await page.evaluate(() => {
    const emit = (window as any).cueEmit;
    emit({ type: 'answer-start', id: 'saved-answer', text: '之前的问题' });
    emit({ type: 'answer-token', id: 'saved-answer', text: '已经生成的回答内容。' });
    emit({ type: 'answer-done', id: 'saved-answer' });
  });
  await page.locator('#settings-toggle').click();
  await emitLicense(page, { ...licensed, state: 'locked', allowed: false, message: '卡密已冻结，请联系管理员' });
  await expect(page.locator('#license-message')).toHaveText('卡密已冻结，请联系管理员');
  await expect(page.locator('#toolbar')).toBeHidden();
  await expect(page.locator('.modal')).toHaveCount(0);
  await page.keyboard.press('Meta+Shift+A');
  await page.evaluate(() => (window as any).cueEmit({ type: 'shortcut', text: 'assist' }));
  await expectNoAutomaticWork(page);
  await page.locator('#license-retry').click();
  await expect(page.locator('#toolbar')).toBeVisible();
  await expect(page.locator('#question')).toHaveValue('尚未发送的会议问题');
  await expect(page.locator('.answer-content')).toContainText('已经生成的回答内容');
  expect(await calls(page, ['RetryLicense'])).toHaveLength(1);
  await expectNoAutomaticWork(page);
});

test('设置授权页支持试用转付费与确认清除，取消清除保留当前会话', async ({ page }) => {
  await mockLicenseDesktop(page, trial);
  await page.locator('#settings-toggle').click();
  await page.locator('[data-tab="license"]').click();
  await expect(page.locator('#license-summary')).toContainText('24 小时试用');
  await expect(page.locator('#settings-page')).toContainText('清除本机授权不会恢复试用资格');
  await page.locator('#replacement-key').fill('TEST-ONLY-NEW-LICENSE');
  await page.locator('#replace-license').click();
  await expect(page.locator('#license-summary')).toContainText('卡密授权');
  await expect(page.locator('#license-summary')).toContainText('CUE-****-TEST');
  await expect(page.locator('#replacement-key')).toHaveValue('');
  expect((await calls(page, ['ActivateLicense']))[0].args).toEqual(['TEST-ONLY-NEW-LICENSE']);
  await page.locator('#forget-license').click();
  await expect(page.getByRole('alertdialog')).toContainText('设备绑定仍需管理员解除');
  await page.locator('[data-confirm-cancel]').click();
  expect(await calls(page, ['ForgetLicense'])).toEqual([]);
  await expect(page.locator('#toolbar')).toBeVisible();
  await page.locator('#forget-license').click();
  await page.locator('[data-confirm-accept]').click();
  await expect(page.locator('#license-panel')).toBeVisible();
  await expect(page.locator('#toolbar')).toBeHidden();
  await expect(page.locator('.modal')).toHaveCount(0);
  expect(await calls(page, ['ForgetLicense'])).toHaveLength(1);
  await expectNoAutomaticWork(page);
});

test('换卡失败先推送旧授权状态时仍显示错误提示并保留当前授权', async ({ page }) => {
  await mockLicenseDesktop(page, licensed, ['ActivateLicense']);
  await page.locator('#settings-toggle').click();
  await page.locator('[data-tab="license"]').click();
  const previousSummary = await page.locator('#license-summary').innerText();
  await page.locator('#replacement-key').fill('TEST-ONLY-WRONG-REPLACEMENT');
  await page.locator('#replace-license').click();
  await emitLicense(page, licensed);
  await page.evaluate(() => (window as any).mockPending.ActivateLicense.reject(new Error('此卡密已绑定其他电脑，请联系管理员解绑')));
  await expect(page.locator('#toast-region .toast-error')).toHaveText('此卡密已绑定其他电脑，请联系管理员解绑');
  await expect(page.locator('#toast-region .toast-error')).toBeVisible();
  await expect(page.locator('#toolbar')).toBeVisible();
  await expect(page.locator('#license-panel')).toBeHidden();
  await expect(page.locator('#license-summary')).toHaveText(previousSummary);
  await expect(page.locator('#session-toggle')).toBeEnabled();
  expect((await calls(page, ['ActivateLicense']))[0].args).toEqual(['TEST-ONLY-WRONG-REPLACEMENT']);
  expect(await page.locator('#toast-region').innerText()).not.toContain('TEST-ONLY-WRONG-REPLACEMENT');
  await expectNoAutomaticWork(page);
});

test('系统凭据保存失败时授权仍可使用，成功事件与响应不重复弹出警告', async ({ page }) => {
  await mockLicenseDesktop(page, inactive, ['ActivateLicense']);
  const warning = '本次已激活，但系统凭据保存失败；下次启动需重新输入卡密';
  const unsaved = { ...licensed, saved: false, message: warning };
  await page.locator('#license-key').fill('TEST-ONLY-UNSAVED-LICENSE');
  await page.locator('#license-activate').click();
  await emitLicense(page, unsaved);
  await page.evaluate(status => (window as any).mockPending.ActivateLicense.resolve(status), unsaved);
  await expect(page.locator('#license-panel')).toHaveAttribute('aria-busy', 'false');
  await expect(page.locator('#toolbar')).toBeVisible();
  await expect(page.locator('#license-panel')).toBeHidden();
  await expect(page.locator('#toast-region .toast-error')).toHaveText(warning);
  await expect(page.locator('#toast-region .toast-error')).toBeVisible();
  await expect(page.locator('#toast-region .toast-error')).toHaveCount(1);
  await expect(page.locator('#license-key')).toHaveValue('');
  await page.locator('#settings-toggle').click();
  await page.locator('[data-tab="license"]').click();
  await expect(page.locator('#license-summary')).toContainText('卡密授权');
  await expect(page.locator('#license-summary')).toContainText('未保存到系统凭据库');
  await expectNoAutomaticWork(page);
});

test('断网宽限倒计时可见，服务端锁定立即退出业务界面', async ({ page }) => {
  await mockLicenseDesktop(page, licensed);
  await page.locator('#collapse-toggle').click();
  await emitLicense(page, { ...licensed, state: 'grace', graceSeconds: 45, message: '网络暂时不可用' });
  await expect(page.locator('#license-grace')).toBeVisible();
  await expect(page.locator('#license-grace')).toContainText('45 秒后暂停功能');
  await expect(page.locator('#session-toggle')).toBeEnabled();
  await emitLicense(page, { ...licensed, state: 'locked', allowed: false, message: '无法验证授权，请联网后重试' });
  await expect(page.locator('#toolbar')).toBeHidden();
  await expect(page.locator('#license-panel')).toBeVisible();
  await expect(page.locator('#session-toggle')).toBeDisabled();
  await expectNoAutomaticWork(page);
});

test('启动时较新的授权事件不会被旧状态查询覆盖', async ({ page }) => {
  await mockLicenseDesktop(page, licensed, ['GetLicenseStatus']);
  await emitLicense(page, { ...licensed, state: 'locked', allowed: false, message: '当前会话已被管理员下线' });
  await page.evaluate(status => (window as any).mockPending.GetLicenseStatus.resolve(status), licensed);
  await expect(page.locator('#license-message')).toHaveText('当前会话已被管理员下线');
  await expect(page.locator('#license-retry')).toBeEnabled();
  await expect(page.locator('#toolbar')).toBeHidden();
  await expectNoAutomaticWork(page);
});

test('重新验证期间较新的锁定事件不会被迟到的成功响应覆盖', async ({ page }) => {
  await mockLicenseDesktop(page, inactive, ['RetryLicense']);
  await page.locator('#license-retry').click();
  await emitLicense(page, { ...licensed, state: 'locked', allowed: false, message: '当前会话已被管理员撤销' });
  await page.evaluate(status => (window as any).mockPending.RetryLicense.resolve(status), licensed);
  await expect(page.locator('#license-retry')).toBeEnabled();
  await expect(page.locator('#license-message')).toHaveText('当前会话已被管理员撤销');
  await expect(page.locator('#license-panel')).toHaveAttribute('aria-busy', 'false');
  await expect(page.locator('#toolbar')).toBeHidden();
  await expectNoAutomaticWork(page);
});

test('78px 菜单窗口切回激活时不滚动页面，扩容后操作按钮可到达', async ({ page }) => {
  await page.setViewportSize({ width: 700, height: 78 });
  await mockLicenseDesktop(page, licensed);
  await expect(page.locator('#toolbar')).toBeVisible();
  await emitLicense(page, { ...licensed, state: 'locked', allowed: false, message: '授权已到期，请输入新卡' });
  await expect(page.locator('#license-panel')).toBeVisible();
  const panel = (await page.locator('#license-panel').boundingBox())!;
  expect(await page.evaluate(() => [window.scrollY, document.documentElement.scrollTop, document.body.scrollTop])).toEqual([0, 0, 0]);
  await expect.poll(() => page.evaluate(() => (window as any).mockCalls.filter((call: any) => call.method === 'SetOverlayLayout').at(-1)?.args[1])).toBeGreaterThan(78);
  const height = await page.evaluate(() => (window as any).mockCalls.filter((call: any) => call.method === 'SetOverlayLayout').at(-1).args[1]);
  await page.setViewportSize({ width: 700, height });
  await page.locator('#license-key').click();
  await page.locator('#license-key').press('Tab');
  await expect(page.locator('#license-activate')).toBeFocused();
  for (const id of ['#license-activate', '#license-trial', '#license-retry', '#license-quit']) await expect(page.locator(id)).toBeInViewport();
  expect(await page.locator('#license-panel').boundingBox()).toEqual(panel);
  expect(await page.evaluate(() => [window.scrollY, document.documentElement.scrollTop, document.body.scrollTop])).toEqual([0, 0, 0]);
});

test('小工作区内激活面板不溢出，按钮与退出仍可操作', async ({ page }) => {
  await page.setViewportSize({ width: 620, height: 480 });
  await mockLicenseDesktop(page, inactive, [], { maxWidth: 620, maxHeight: 480 });
  await expect.poll(() => page.evaluate(() => document.documentElement.style.getPropertyValue('--screen-height'))).toBe('480px');
  for (const id of ['#license-key', '#license-activate', '#license-trial', '#license-retry', '#license-quit']) await expect(page.locator(id)).toBeInViewport();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  await page.screenshot({ path: 'test-results/license-small-screen.png' });
  await page.locator('#license-quit').click();
  expect(await calls(page, ['Quit'])).toHaveLength(1);
});
