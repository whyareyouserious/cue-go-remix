import { call } from '../../api/wails';
import { state } from '../../app/state';
import { closeModal } from '../../components/modal';
import { run, toast } from '../../components/toast';
import { $ } from '../../utils/dom';
import { esc } from '../../utils/format';

export function renderAppearancePage(page: HTMLElement) {
  page.innerHTML = `<h3>保持专注，按需出现</h3><div class="field"><label for="setting-opacity">窗口透明度</label><input id="setting-opacity" data-setting="opacity" type="range" min="0.2" max="1" step="0.01" value="${state.draft.opacity}"/></div><label class="switch-row"><span><strong>屏幕共享排除</strong><small>请求系统在录屏或共享时排除 Cue 窗口</small></span><input data-setting="contentProtection" type="checkbox" role="switch" ${state.draft.contentProtection ? 'checked' : ''}/></label><p class="field-hint">效果取决于系统与录屏工具，macOS 无法保证所有捕获方式都排除窗口。更改后可能需要重启。</p><div class="switch-row"><span><strong>鼠标穿透</strong><small>让鼠标点击直接到达窗口下方的应用</small></span><button id="enable-passthrough" class="secondary-button">开启穿透</button></div><p class="field-hint">开启后，用 ${state.status.platform === 'darwin' ? 'Cmd' : 'Ctrl'} + Shift + I 恢复交互。快捷键注册失败时无法开启。</p><label class="switch-row"><span><strong>跟踪会议页面</strong><small>听写期间检查屏幕变化，生成页面文字记录</small></span><input data-setting="slidesEnabled" type="checkbox" role="switch" ${state.draft.slidesEnabled ? 'checked' : ''}/></label><div class="field"><label for="slides-interval">页面检查间隔 <span>毫秒</span></label><input id="slides-interval" data-setting="slidesInterval" type="number" min="1500" max="15000" step="500" value="${state.draft.slidesInterval}"/></div><p class="field-hint">页面描述请求会发送截图至所选模型，并可能产生费用。仅在开启后采集，记录保留在本次运行的内存中。</p><div class="data-location"><span>本地数据目录</span><code>${esc(state.status.dataDir)}</code></div>`;
  $('#enable-passthrough').addEventListener('click', () => void run(async () => { await call('SetPassthrough', true); closeModal(); toast(`已开启鼠标穿透，按 ${state.status.platform === 'darwin' ? 'Cmd' : 'Ctrl'} + Shift + I 恢复交互。`); }));
  page.insertAdjacentHTML('beforeend', `<div class="settings-tools"><button class="secondary-button" data-open-meetings>会议记录</button><button class="secondary-button" data-open-slides>页面记录 <span id="slide-count">${(state.status.slides || []).length}</span></button><button class="secondary-button" data-mode="leetcode">屏幕解题</button></div>`);
}
