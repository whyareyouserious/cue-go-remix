import { call, connect } from '../api/wails';
import { renderStatus, setCollapsed } from '../features/toolbar';
import { renderTranscript } from '../features/transcript';
import { type Settings, type Status } from '../types';
import { $ } from '../utils/dom';
import { handleEvent } from './events';
import { scheduleLayout } from './layout';
import { state } from './state';

export async function start() {
  try {
    await connect(handleEvent);
    const [loadedSettings, loadedStatus] = await Promise.all([call<Settings>('GetSettings'), call<Status>('GetStatus')]);
    state.settings = loadedSettings;
    state.status = loadedStatus;
    state.status.transcript ||= [];
    state.status.slides ||= [];
    state.status.warnings ||= [];
    state.connected = true;
    renderStatus();
    renderTranscript();
    scheduleLayout();
  }
  catch (error) {
    renderStatus();
    $('#warning-banner').hidden = false;
    $('#warning-banner').textContent = `无法连接 Cue 桌面服务：${error instanceof Error ? error.message : String(error)}。请从桌面应用启动；浏览器预览请使用 ?preview=1。`;
    setCollapsed(false);
  }
}
