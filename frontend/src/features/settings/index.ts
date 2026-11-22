import { call } from '../../api/wails';
import { recordUIEvent } from '../../app/diagnostics';
import { state } from '../../app/state';
import { icon } from '../../components/icons';
import { closeModal, modal } from '../../components/modal';
import { run, toast } from '../../components/toast';
import type { Settings } from '../../types';
import { $ } from '../../utils/dom';
import { renderStatus } from '../toolbar';
import { renderAccessPage } from './access';
import { renderAppearancePage } from './appearance';
import { renderChannelsPage } from './channels';
import { renderSpeechChannelsPage } from './speech-channels';
import { prepareChannelDraft, finishChannelMigration } from './channel-state';
import { readSettingsPage } from './form';
import { openModelsModal } from './models-modal';
import { renderStylePage } from './style';
import { renderShortcutsPage } from './shortcuts';
import { renderDiagnosticsPage } from './diagnostics';
export { readSettingsPage } from './form';

export function openSettings(tab = 'chat') {
  if (state.pendingConsent)
    return;
  if (['chat', 'speech', 'style', 'appearance', 'shortcuts', 'diagnostics'].includes(tab))
    recordUIEvent('settings-open', tab);
  state.draft = structuredClone(state.settings);
  prepareChannelDraft();
  state.settingsTab = tab;
  modal('设置', '', true, 'settings');
  $('#modal-body').innerHTML = `<div class="settings-layout"><nav class="settings-nav" aria-label="设置分类">${[
    ['chat', 'spark', '密钥'],
    ['speech', 'mic', '音频'],
    ['style', 'sliders', '风格'],
    ['appearance', 'eclipse', '外观'],
    ['shortcuts', 'keyboard', '按键'],
    ['diagnostics', 'shield', '诊断'],
  ].map(([id, glyph, title]) => `<button data-tab="${id}" class="${tab === id ? 'active' : ''}">${icon(glyph)}${title}</button>`).join('')}</nav><div id="settings-page" class="settings-page"></div></div>`;
  $('#modal-footer').hidden = false;
  $('#modal-footer').innerHTML = '<span id="settings-notice">更改将在保存后生效</span><button class="primary-button" id="save-settings">保存设置</button>';
  document.querySelectorAll<HTMLButtonElement>('[data-tab]').forEach(button => button.addEventListener('click', () => {
    readSettingsPage();
    state.settingsTab = button.dataset.tab!;
    recordUIEvent('settings-open', state.settingsTab);
    renderSettingsPage();
  }));
  $('#save-settings').addEventListener('click', () => void run(saveSettings));
  renderSettingsPage();
}

export function renderSettingsPage() {
  // 旧模型入口也统一打开子弹窗，底层始终保留音频设置。
  const showModels = state.settingsTab === 'models';
  if (showModels) state.settingsTab = 'speech';
  const activeTab = state.settingsTab === 'access' ? 'chat' : state.settingsTab;
  document.querySelectorAll('[data-tab]').forEach(button => button.classList.toggle('active', (button as HTMLElement).dataset.tab === activeTab));
  const page = $('#settings-page');
  switch (state.settingsTab) {
    case 'chat': renderChannelsPage(page, renderSettingsPage); break;
    case 'speech': renderSpeechChannelsPage(page, renderSettingsPage); break;
    case 'style': renderStylePage(page); break;
    case 'appearance': renderAppearancePage(page); break;
    case 'shortcuts': renderShortcutsPage(page); break;
    case 'diagnostics': void renderDiagnosticsPage(page); break;
    default: renderAccessPage(page);
  }
  // 二级页面复用统一导航，返回前读取当前表单，保留尚未保存的配置草稿。
  const parent = state.settingsTab === 'access' ? { tab: 'chat', label: '返回密钥设置' } : undefined;
  if (parent) {
    page.insertAdjacentHTML('afterbegin', `<div class="settings-subpage-nav"><button type="button" class="secondary-button" data-settings-page="${parent.tab}"><span aria-hidden="true">←</span>${parent.label}</button></div>`);
  }
  if (showModels) openModelsModal(renderSettingsPage);
}

export async function saveSettings() {
  recordUIEvent('settings-save', state.settingsTab);
  readSettingsPage();

  // 检测是否有尚未点击“应用按键”的快捷键草稿修改，阻止静默覆盖并保留草稿
  if (state.draft.shortcuts && state.settings.shortcuts) {
    let hasUnappliedShortcuts = false;
    for (const [k, v] of Object.entries(state.draft.shortcuts)) {
      if (state.settings.shortcuts[k] !== v) {
        hasUnappliedShortcuts = true;
        break;
      }
    }
    if (hasUnappliedShortcuts) {
      state.settingsTab = 'shortcuts';
      renderSettingsPage();
      throw new Error('检测到快捷键有未应用的修改，请在按键页点击“应用按键”完成系统注册，避免修改丢失。');
    }
  }

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
  if (state.draft.activeChatChannelId && !state.draft.chat.model.trim())
    throw new Error('请填写快速模型名称。');
  if (!state.draft.chat.smartModel.trim())
    state.draft.chat.smartModel = state.draft.chat.model;
  const save = $<HTMLButtonElement>('#save-settings');
  save.disabled = true;
  save.textContent = '正在保存…';
  try {
    const updated = await call<Settings>('SaveSettings', state.draft);
    state.settings = updated;
    finishChannelMigration(updated);
    // 同步 draft 与 settings，避免旧快捷键或位置草稿互相覆盖
    state.draft = structuredClone(updated);
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
