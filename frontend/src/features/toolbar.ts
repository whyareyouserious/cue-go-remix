import { preview } from '../api/wails';
import { recordUIEvent } from '../app/diagnostics';
import { resizeQuestion, scheduleLayout } from '../app/layout';
import { state } from '../app/state';
import { icon } from '../components/icons';
import { closeModal } from '../components/modal';
import { $ } from '../utils/dom';

export function renderStatus() {
  const label = $('#connection-status');
  label.className = `status-label ${state.status.listening ? 'is-live' : ''}`;
  label.textContent = !state.connected ? '桌面服务未连接' : state.status.listening ? '正在转录' : preview ? '预览模式' : '准备就绪 · 转录关闭';
  $('#session-toggle').innerHTML = `${icon(state.status.listening ? 'stop' : 'play')}<span>${state.status.listening ? '结束会话' : '开始会话'}</span>`;
  $('#session-toggle').classList.toggle('is-live', state.status.listening);
  $('#listen-dot').classList.toggle('is-live', state.status.listening);
  $<HTMLButtonElement>('#session-toggle').disabled = !state.connected || !state.licenseStatus.allowed;
  $<HTMLButtonElement>('#send').disabled = !state.connected || !state.licenseStatus.allowed || state.status.busy;
  $('#cancel-answer').hidden = !state.status.busy;
  $('#smart-toggle').setAttribute('aria-pressed', String(state.settings.smart));
  $('#composer').classList.toggle('listening', state.status.listening);
  $('#model-label').textContent = (state.settings.smart ? state.settings.chat.smartModel : state.settings.chat.model) || '请设置模型';
  $('#audio-description').textContent = `麦克风 · ${state.settings.meetingAudio ? '系统音频开启' : '系统音频关闭'}`;
  $('#say-key').textContent = state.status.platform === 'darwin' ? '⌘ ↵' : 'Ctrl ↵';
  $('#assist-key').textContent = state.status.platform === 'darwin' ? '⌘ ⇧ ↵' : 'Ctrl ⇧ ↵';
  $<HTMLTextAreaElement>('#question').placeholder = `询问屏幕或对话内容，或按 ${state.status.platform === 'darwin' ? '⌘' : 'Ctrl'} ⇧ ↵ 智能辅助`;
  const slideCount = document.querySelector('#slide-count');
  if (slideCount)
    slideCount.textContent = String((state.status.slides || []).length);
  const warnings = state.status.warnings || [];
  $('#warning-banner').textContent = warnings.join('；');
  $('#warning-banner').hidden = warnings.length === 0;
  document.documentElement.style.setProperty('--cue-opacity', String(Math.min(1, Math.max(.2, state.settings.opacity || 1))));
  $<HTMLInputElement>('#quick-opacity').value = String(Math.round(state.settings.opacity * 100));
  $('#opacity-value').textContent = `${Math.round(state.settings.opacity * 100)}%`;
  scheduleLayout();
}

export function setCollapsed(value: boolean) {
  if (state.collapsed !== value)
    recordUIEvent('panel-toggle', value ? 'collapsed' : 'expanded');
  state.collapsed = value;
  $('#workspace').hidden = state.collapsed;
  $('#collapse-toggle').setAttribute('aria-expanded', String(!state.collapsed));
  $('#collapse-toggle').setAttribute('aria-label', state.collapsed ? '显示面板' : '隐藏面板');
  $('#collapse-toggle').title = state.collapsed ? '显示面板' : '隐藏面板';
  $('#collapse-toggle span').textContent = state.collapsed ? '显示' : '隐藏';
  $('#collapse-toggle').classList.toggle('collapsed', state.collapsed);
  $('.desktop').classList.toggle('collapsed', state.collapsed);
  if (!state.collapsed)
    resizeQuestion();
  scheduleLayout();
}

export async function toggleCollapsed() { setCollapsed(!state.collapsed); }

export function toggleSidebar() {
  state.sidebar = !state.sidebar;
  recordUIEvent('sidebar-toggle', state.sidebar ? 'open' : 'closed');
  $('#transcript-panel').hidden = !state.sidebar;
  $('#sidebar-toggle').setAttribute('aria-pressed', String(state.sidebar));
  $('.desktop').classList.toggle('sidebar-open', state.sidebar);
  scheduleLayout();
}

export function setOpacityOpen(open: boolean) {
  if (open && !state.licenseStatus.allowed)
    return;
  if (open && state.pendingConsent)
    return;
  if (open && state.activeModal)
    closeModal();
  $('#opacity-popover').hidden = !open;
  $('#opacity-toggle').setAttribute('aria-expanded', String(open));
}
