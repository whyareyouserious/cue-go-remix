import { call } from '../api/wails';
import { scheduleLayout } from '../app/layout';
import { state } from '../app/state';
import { confirmAction } from '../components/confirm';
import { closeModal } from '../components/modal';
import { run, toast } from '../components/toast';
import { type LicenseStatus } from '../types';
import { $ } from '../utils/dom';
import { esc } from '../utils/format';
import { renderStatus, setOpacityOpen } from './toolbar';

export function renderLicenseControls() {
  const checking = state.licenseStatus.state === 'checking';
  $('#license-panel').setAttribute('aria-busy', String(state.licenseBusy || checking));
  for (const id of ['#license-activate', '#license-trial', '#license-retry'])
    $<HTMLButtonElement>(id).disabled = state.licenseBusy || !state.connected || checking;
}

export function applyLicense(next: LicenseStatus) {
  if (!next || typeof next.allowed !== 'boolean')
    return;
  const lost = state.licenseStatus.allowed && !next.allowed;
  const storageWarning = next.allowed && !next.saved && next.message && next.message !== state.licenseStatus.message;
  state.licenseStatus = next;
  $('.desktop').hidden = !next.allowed;
  $('#license-panel').hidden = next.allowed;
  $('#license-message').textContent = next.message || (next.state === 'checking' ? '正在联网验证授权' : '输入 Cue 卡密或领取试用');
  renderLicenseControls();
  if (lost) {
    setOpacityOpen(false);
    if ($('#modal-root').childElementCount)
      closeModal();
  }
  if (next.allowed)
    $<HTMLInputElement>('#license-key').value = '';
  const summary = document.querySelector('#license-summary');
  if (summary)
    summary.textContent = licenseSummary();
  const banner = $('#license-grace');
  banner.hidden = next.state !== 'grace';
  banner.textContent = `授权服务暂时无法连接，正在重试。${next.graceSeconds} 秒后暂停功能。`;
  renderStatus();
  scheduleLayout();
  if (storageWarning)
    toast(next.message, true);
}

export function licenseSummary() {
  return `${state.licenseStatus.mode === 'trial' ? '24 小时试用' : '卡密授权'}${state.licenseStatus.maskedKey ? ' · ' + state.licenseStatus.maskedKey : ''} · 到期：${state.licenseStatus.expiresAt ? new Date(state.licenseStatus.expiresAt).toLocaleString('zh-CN') : '—'}${state.licenseStatus.saved ? '' : ' · 未保存到系统凭据库'}`;
}

export async function licenseAction(method: string, ...args: unknown[]) {
  if (state.licenseBusy || !state.connected || state.licenseStatus.state === 'checking')
    return;
  const revision = state.licenseRevision;
  state.licenseBusy = true;
  applyLicense(state.licenseStatus);
  try {
    const next = await call<LicenseStatus>(method, ...args);
    if (revision === state.licenseRevision)
      applyLicense(next);
  }
  catch (error) {
    const text = error instanceof Error ? error.message : String(error);
    if (state.licenseStatus.allowed)
      toast(text, true);
    else if (revision === state.licenseRevision)
      $('#license-message').textContent = text;
  }
  finally {
    state.licenseBusy = false;
    renderLicenseControls();
  }
}

export function renderLicenseSettings(page: HTMLElement) {
  page.innerHTML = `<h3>Cue 授权</h3><p id="license-summary" class="section-description">${esc(licenseSummary())}</p><p class="field-hint">退出软件不会解除绑机。更换电脑请联系管理员；清除本机授权不会恢复试用资格。</p><div class="field"><label for="replacement-key">更换卡密 / 转为付费</label><input id="replacement-key" type="password" autocomplete="off" maxlength="128" placeholder="输入新的 Cue 卡密" /></div><div class="license-actions"><button id="replace-license" class="primary-button">验证新卡</button><button id="forget-license" class="text-button danger">清除本机授权</button></div>`;
  $('#replace-license').addEventListener('click', () => void licenseAction('ActivateLicense', $<HTMLInputElement>('#replacement-key').value).then(() => {
    const input = document.querySelector<HTMLInputElement>('#replacement-key'); if (input)
      input.value = '';
  }));
  $('#forget-license').addEventListener('click', () => void run(async () => {
    if (await confirmAction('清除本机保存的卡密并停止当前会话？设备绑定仍需管理员解除。'))
      await licenseAction('ForgetLicense');
  }));
}
