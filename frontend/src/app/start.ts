import { call, connect } from '../api/wails';
import { applyLicense } from '../features/license';
import { renderStatus } from '../features/toolbar';
import { renderTranscript } from '../features/transcript';
import { type LicenseStatus, type Settings, type Status } from '../types';
import { $ } from '../utils/dom';
import { handleEvent } from './events';
import { scheduleLayout } from './layout';
import { state } from './state';

export async function start() {
  try {
    await connect(handleEvent);
    const revision = state.licenseRevision;
    const [loadedSettings, loadedStatus, loadedLicense] = await Promise.all([call<Settings>('GetSettings'), call<Status>('GetStatus'), call<LicenseStatus>('GetLicenseStatus')]);
    state.settings = loadedSettings;
    state.status = loadedStatus;
    if (revision === state.licenseRevision && loadedLicense)
      state.licenseStatus = loadedLicense;
    state.status.transcript ||= [];
    state.status.slides ||= [];
    state.status.warnings ||= [];
    state.connected = true;
    applyLicense(state.licenseStatus);
    renderTranscript();
    scheduleLayout();
  }
  catch (error) {
    renderStatus();
    $('#warning-banner').hidden = false;
    $('#warning-banner').textContent = `无法连接 Cue 桌面服务：${error instanceof Error ? error.message : String(error)}。请从桌面应用启动；浏览器预览请使用 ?preview=1。`;
    $('#license-message').textContent = '无法连接桌面服务，请从 Cue 应用启动；浏览器预览请加 ?preview=1。';
    scheduleLayout();
  }
}
