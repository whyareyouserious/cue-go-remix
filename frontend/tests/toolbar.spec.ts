import { test, expect } from '@playwright/test';
import { initialSettings } from '../src/types';

for (const [button, title] of [['#settings-toggle', '设置'], ['#help', '欢迎使用 cue']]) {
  test(`菜单条${title}按钮第一次打开、第二次关闭，再次点击仍可打开`, async ({ page }) => {
    await page.goto('/?preview=1');
    await page.locator(button).click();
    await expect(page.locator('#modal-title')).toHaveText(title);
    await page.locator(button).click();
    await expect(page.locator('.modal')).toHaveCount(0);
    await expect(page.locator(button)).toHaveAttribute('aria-expanded', 'false');
    await page.locator(button).click();
    await expect(page.locator('#modal-title')).toHaveText(title);
    await expect(page.locator(button)).toHaveAttribute('aria-expanded', 'true');
    await page.locator('#modal-close').click();
    await expect(page.locator(button)).toHaveAttribute('aria-expanded', 'false');
  });
}

test('设置、指南和透明度互相切换，关闭设置不保存草稿', async ({ page }) => {
  await page.goto('/?preview=1');
  await page.locator('#settings-toggle').click();
  await page.locator('[data-tab="speech"]').click();
  await page.locator('#settings-toggle').click();
  await expect(page.locator('.modal')).toHaveCount(0);
  await page.locator('#settings-toggle').click();
  await page.locator('#endpoint-model').fill('未保存的临时模型');
  await page.locator('#help').click();
  await expect(page.locator('#modal-title')).toHaveText('欢迎使用 cue');
  await expect(page.locator('#settings-toggle')).toHaveAttribute('aria-expanded', 'false');
  await page.locator('#opacity-toggle').click();
  await expect(page.locator('.modal')).toHaveCount(0);
  await expect(page.locator('#opacity-popover')).toBeVisible();
  await expect(page.locator('#help')).toHaveAttribute('aria-expanded', 'false');
  await page.locator('#settings-toggle').click();
  await expect(page.locator('#opacity-popover')).toBeHidden();
  await expect(page.locator('#opacity-toggle')).toHaveAttribute('aria-expanded', 'false');
  await expect(page.locator('#endpoint-model')).toHaveValue(initialSettings.chat.model);
  await page.keyboard.press('Escape');
  await expect(page.locator('#settings-toggle')).toHaveAttribute('aria-expanded', 'false');
  await page.locator('#opacity-toggle').click();
  await page.locator('#help').click();
  await expect(page.locator('#opacity-popover')).toBeHidden();
  await page.locator('#guide-start').click();
  await expect(page.locator('#modal-title')).toHaveText('设置');
  await expect(page.locator('#help')).toHaveAttribute('aria-expanded', 'false');
  await expect(page.locator('#settings-toggle')).toHaveAttribute('aria-expanded', 'true');
  await page.locator('#save-settings').click();
  await expect(page.locator('.modal')).toHaveCount(0);
  await expect(page.locator('#settings-toggle')).toHaveAttribute('aria-expanded', 'false');
});

test('菜单条弹层切换不会替用户拒绝待处理授权', async ({ page }) => {
  await page.addInitScript(settings => {
    (window as any).mockSettings = settings;
    (window as any).consentCalls = [];
  }, initialSettings);
  await page.route('**/wails/runtime.js*', route => route.fulfill({ contentType: 'application/javascript', body: `
    export const Events = { On: (_, callback) => { window.cueEmit = data => callback({data}); } };
    export const Call = { ByName: async (name, ...args) => {
      if (!name.startsWith('cue-go-remix/internal/controller.App.')) throw new Error('Wails 绑定名称不匹配：' + name);
      const method = name.split('.').pop();
      if (method === 'GetSettings') return window.mockSettings;
      if (method === 'GetLicenseStatus') return {state:'license',allowed:true,mode:'license',expiresAt:Date.now()+86400000,graceSeconds:0,message:'授权有效',saved:true};
      if (method === 'GetStatus') return {platform:'darwin',listening:false,busy:false,dataDir:'/tmp/cue-test',transcript:[],slides:[],warnings:[]};
      if (method === 'Consent') window.consentCalls.push(args);
    }};
  ` }));
  await page.goto('/');
  await expect(page.locator('#connection-status')).toContainText('准备就绪');
  await page.locator('#opacity-toggle').click();
  await page.evaluate(() => (window as any).cueEmit({ type: 'consent', id: 'pending-toolbar', text: '允许读取状态？' }));
  await expect(page.locator('.consent-content')).toBeVisible();
  await expect(page.locator('#opacity-popover')).toBeHidden();
  for (const button of ['#settings-toggle', '#help', '#opacity-toggle']) {
    await page.locator(button).click();
    await expect(page.locator('.consent-content')).toBeVisible();
    await expect(page.locator(button)).toHaveAttribute('aria-expanded', 'false');
  }
  expect(await page.evaluate(() => (window as any).consentCalls)).toEqual([]);
  await page.keyboard.press('Escape');
  await expect(page.locator('.modal')).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).consentCalls)).toEqual([['pending-toolbar', false]]);
});
