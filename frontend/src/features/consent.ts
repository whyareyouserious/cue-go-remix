import { call } from '../api/wails';
import { state } from '../app/state';
import { icon } from '../components/icons';
import { closeModal, modal } from '../components/modal';
import { run } from '../components/toast';
import { type CueEvent } from '../types';
import { $ } from '../utils/dom';
import { esc } from '../utils/format';

export function showConsent(event: CueEvent) {
  const info = event.data as {
    caller?: string;
    scope?: string;
  } | undefined;
  const body = modal('本机程序请求访问', '只有你允许后，对方才能使用对应能力');
  state.pendingConsent = event.id || '';
  body.innerHTML = `<div class="consent-content">${icon('shield')}<h3>${esc(info?.caller || '一个本机程序')}</h3><p>${esc(event.text || `请求权限：${info?.scope || '读取状态'}`)}</p><p class="field-hint">此名称由请求方提供，不代表已验证程序身份。授权可在设置中撤销。</p></div>`;
  const respond = async (allowed: boolean) => {
    const current = $('.modal');
    state.pendingConsent = '';
    try {
      await call('Consent', event.id || '', allowed);
    }
    catch (error) {
      if (current.isConnected)
        state.pendingConsent = event.id || '';
      throw error;
    }
    if (current.isConnected)
      closeModal();
  };
  $('#modal-footer').hidden = false;
  $('#modal-footer').innerHTML = '<button id="consent-deny" class="secondary-button">拒绝</button><button id="consent-allow" class="primary-button">允许</button>';
  $('#consent-deny').addEventListener('click', () => void run(() => respond(false)));
  $('#consent-allow').addEventListener('click', () => void run(() => respond(true)));
}
