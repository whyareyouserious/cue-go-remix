import { call } from '../../api/wails';
import { recordUIEvent } from '../../app/diagnostics';
import { state } from '../../app/state';
import { icon } from '../../components/icons';
import { closeModal, modal } from '../../components/modal';
import { run, toast } from '../../components/toast';
import type { Settings } from '../../types';
import { $ } from '../../utils/dom';
import { renderLicenseSettings } from '../license';
import { renderStatus } from '../toolbar';
import { renderAccessPage } from './access';
import { renderAppearancePage } from './appearance';
import { renderEndpointPage } from './endpoint';
import { readSettingsPage } from './form';
import { renderModelsPage } from './models';
import { renderStylePage } from './style';
export { readSettingsPage } from './form';

export function openSettings(tab = 'chat') {
  if (!state.licenseStatus.allowed)
    return;
  if (state.pendingConsent)
    return;
  if (['chat', 'speech', 'style', 'appearance'].includes(tab))
    recordUIEvent('settings-open', tab);
  state.draft = structuredClone(state.settings);
  state.settingsTab = tab;
  modal('设置', '', true, 'settings');
  $('#modal-body').innerHTML = `<div class="settings-layout"><nav class="settings-nav" aria-label="设置分类">${[
    ['chat', 'spark', '密钥'], ['speech', 'mic', '音频'], ['style', 'sliders', '风格'], ['appearance', 'eclipse', '外观'], ['license', 'settings', '授权'],
  ].map(([id, glyph, title]) => `<button data-tab="${id}" class="${tab === id ? 'active' : ''}">${icon(glyph)}${title}</button>`).join('')}</nav><div id="settings-page" class="settings-page"></div></div>`;
  $('#modal-footer').hidden = false;
  $('#modal-footer').innerHTML = '<span id="settings-notice">更改将在保存后生效</span><button class="primary-button" id="save-settings">保存设置</button>';
  document.querySelectorAll<HTMLButtonElement>('[data-tab]').forEach(button => button.addEventListener('click', () => { readSettingsPage(); state.settingsTab = button.dataset.tab!; recordUIEvent('settings-open', state.settingsTab); renderSettingsPage(); }));
  $('#save-settings').addEventListener('click', () => void run(saveSettings));
  renderSettingsPage();
}

export function renderSettingsPage() {
  const activeTab = state.settingsTab === 'models' ? 'speech' : state.settingsTab === 'access' ? 'chat' : state.settingsTab;
  document.querySelectorAll('[data-tab]').forEach(button => button.classList.toggle('active', (button as HTMLElement).dataset.tab === activeTab));
  const page = $('#settings-page');
  switch (state.settingsTab) {
    case 'chat': case 'speech': renderEndpointPage(page, renderSettingsPage); break;
    case 'models': renderModelsPage(page); break;
    case 'style': renderStylePage(page); break;
    case 'license': renderLicenseSettings(page); break;
    case 'appearance': renderAppearancePage(page); break;
    default: renderAccessPage(page);
  }
}

export async function saveSettings() {
  recordUIEvent('settings-save', state.settingsTab);
  readSettingsPage();
  for (const endpoint of [state.draft.chat, state.draft.speech]) {
    if (!['local', 'sherpa'].includes(endpoint.provider) && endpoint.baseUrl) {
      let url: URL;
      try {
        url = new URL(endpoint.baseUrl);
      }
      catch {
        throw new Error('Base URL 格式不正确，请填写完整地址。');
      }
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password)
        throw new Error('服务地址只支持 HTTP / HTTPS，且不能包含账号密码。');
    }
    if (endpoint.apiKey.trim())
      endpoint.clearKey = false;
  }
  if (!state.draft.chat.model.trim())
    throw new Error('请填写快速模型名称。');
  if (!state.draft.chat.smartModel.trim())
    state.draft.chat.smartModel = state.draft.chat.model;
  const save = $<HTMLButtonElement>('#save-settings');
  save.disabled = true;
  save.textContent = '正在保存…';
  try {
    state.settings = await call<Settings>('SaveSettings', state.draft);
    renderStatus();
    closeModal();
    toast('设置已保存');
  }
  finally {
    if (save.isConnected) {
      save.disabled = false;
      save.textContent = '保存设置';
    }
  }
}
