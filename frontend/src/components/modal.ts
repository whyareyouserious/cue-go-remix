import { call } from '../api/wails';
import { type ModalKind, state } from '../app/state';
import { $ } from '../utils/dom';
import { esc } from '../utils/format';
import { icon } from './icons';
import { run } from './toast';

export function setActiveModal(kind?: ModalKind) {
  state.activeModal = kind;
  $('#settings-toggle').setAttribute('aria-expanded', String(kind === 'settings'));
  $('#help').setAttribute('aria-expanded', String(kind === 'guide'));
}

export function modal(title: string, subtitle = '', wide = false, kind: ModalKind = 'other') {
  $('#opacity-popover').hidden = true;
  $('#opacity-toggle').setAttribute('aria-expanded', 'false');
  if (state.pendingConsent) {
    const id = state.pendingConsent;
    state.pendingConsent = '';
    void run(() => call('Consent', id, false));
  }
  setActiveModal(kind);
  state.restoreFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const root = $('#modal-root');
  root.innerHTML = `<div class="modal-backdrop" data-hit-region><section class="modal glass ${wide ? 'modal-wide' : ''}" data-hit-region role="dialog" aria-modal="true" aria-labelledby="modal-title"><header class="modal-header"><div><h2 id="modal-title">${esc(title)}</h2>${subtitle ? `<p>${esc(subtitle)}</p>` : ''}</div><button class="icon-button" id="modal-close" title="关闭" aria-label="关闭">${icon('close')}</button></header><div id="modal-body" class="modal-body"></div><footer id="modal-footer" class="modal-footer" hidden></footer></section></div>`;
  $('#modal-close').addEventListener('click', closeModal);
  $('.modal-backdrop').addEventListener('click', event => {
    if (event.target === event.currentTarget)
      closeModal();
  });
  // 宿主此时可能只有菜单条高度，聚焦不能滚动页面并扰乱尺寸上报。
  $<HTMLButtonElement>('#modal-close').focus({ preventScroll: true });
  return $('#modal-body');
}

export function closeModal() {
  if (state.pendingConsent) {
    const id = state.pendingConsent;
    state.pendingConsent = '';
    void run(() => call('Consent', id, false));
  }
  $('#modal-root').replaceChildren();
  setActiveModal();
  if (state.restoreFocus?.isConnected)
    state.restoreFocus.focus({ preventScroll: true });
}
