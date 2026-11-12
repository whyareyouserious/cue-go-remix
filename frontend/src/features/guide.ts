import { call } from '../api/wails';
import { state } from '../app/state';
import { closeModal, modal } from '../components/modal';
import { run } from '../components/toast';
import { type Settings } from '../types';
import { $ } from '../utils/dom';
import { openSettings } from './settings/index';

export function showGuide() {
  if (!state.licenseStatus.allowed)
    return;
  if (state.pendingConsent)
    return;
  const body = modal('欢迎使用 cue', '实时转录对话，快速得到回答', false, 'guide');
  body.innerHTML = `<div class="onboarding"><div class="guide-step"><strong>1. 配置模型与语音</strong><p>在设置中填写 API Key、第三方中转地址和模型名。聊天与语音可分别配置，也可选择 Whisper 本地转写。</p></div><div class="guide-step"><strong>2. 开始会话</strong><p>点击工具条的“开始会话”。开启系统音频后可转录对方的声音。点击“显示”展开回答面板。</p></div><div class="guide-step"><strong>3. 按 Enter 回答</strong><p>对方说的话会填入输入框，按 Enter 或发送按钮获得回答；手工输入时不会被转录覆盖。转录历史按需展开。</p></div><div class="guide-shortcuts"><span>我该怎么说？ <kbd>${state.status.platform === 'darwin' ? '⌘' : 'Ctrl'} ↵</kbd></span><span>智能辅助 <kbd>${state.status.platform === 'darwin' ? '⌘' : 'Ctrl'} ⇧ ↵</kbd></span><span>屏幕解题 <kbd>${state.status.platform === 'darwin' ? '⌘' : 'Ctrl'} H</kbd></span></div><p class="field-hint">截图和云端语音会发送到所选服务。录屏排除效果取决于系统与捕获工具。</p></div>`;
  $('#modal-footer').hidden = false;
  $('#modal-footer').innerHTML = '<button id="guide-later" class="text-button">稍后配置</button><button id="guide-start" class="primary-button">配置模型</button>';
  const acknowledge = async (configure: boolean) => {
    if (state.connected)
      state.settings = await call<Settings>('SaveSettings', { ...state.settings, onboarded: true });
    if (configure)
      openSettings();
    else
      closeModal();
  };
  $('#guide-later').addEventListener('click', () => void run(() => acknowledge(false)));
  $('#guide-start').addEventListener('click', () => void run(() => acknowledge(true)));
}
