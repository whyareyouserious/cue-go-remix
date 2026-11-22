import { scheduleLayout } from '../../app/layout';
import { state } from '../../app/state';
import { icon } from '../../components/icons';
import { run } from '../../components/toast';
import { esc } from '../../utils/format';
import { loadModels } from '../models';

export function openModelsModal(onClose?: () => void) {
  const existing = document.querySelector<HTMLElement>('#models-modal-close');
  if (existing) {
    existing.focus({ preventScroll: true });
    return;
  }

  const parent = document.querySelector<HTMLElement>('.modal-settings');
  const previousFocus = document.activeElement;

  const overlay = document.createElement('div');
  overlay.className = 'models-modal-overlay glass-backdrop';

  const currentThreads = state.draft.whisperThreads !== undefined ? state.draft.whisperThreads : 4;

  overlay.innerHTML = `
    <div class="models-modal-dialog card glass animate-in" data-hit-region role="dialog" aria-modal="true" aria-labelledby="models-modal-title">
      <div class="models-modal-header">
        <div class="header-title-wrap">
          <h4 id="models-modal-title">Whisper 本地模型与运行时管理</h4>
          <small class="header-desc">下载多语言离线语音模型，配置本地计算线程与运行时</small>
        </div>
        <button type="button" class="icon-button close-btn" id="models-modal-close" aria-label="关闭模型管理">${icon('close')}</button>
      </div>

      <div class="models-modal-body">
        <div class="field">
          <label for="whisper-path">whisper-server 路径 <span>留空使用应用随附运行时</span></label>
          <input id="whisper-path" data-setting="whisperPath" value="${esc(state.draft.whisperPath)}" placeholder="留空使用随应用提供的运行时" spellcheck="false" autocomplete="off" />
        </div>

        <div class="field-grid">
          <div class="field">
            <label for="whisper-model">当前选用模型</label>
            <input id="whisper-model" data-setting="whisperModel" value="${esc(state.draft.whisperModel || 'base')}" placeholder="base" spellcheck="false" autocomplete="off" />
          </div>
          <div class="field">
            <label for="whisper-threads">计算线程数 <span>0 为系统自动</span></label>
            <input id="whisper-threads" data-setting="whisperThreads" type="number" min="0" max="64" value="${currentThreads}" />
          </div>
        </div>

        <div class="models-heading" style="display: flex; align-items: center; justify-content: space-between; margin-top: 14px; margin-bottom: 8px;">
          <div>
            <strong style="color: var(--tx-1); font-size: 13.5px;">离线模型库</strong>
            <small style="color: var(--tx-mut); margin-left: 8px; font-size: 11.5px;">中文推荐使用 base 等多语言模型；以 .en 结尾为纯英语模型</small>
          </div>
          <button type="button" id="refresh-models-modal" class="secondary-button btn-sm">刷新模型库</button>
        </div>

        <div id="model-list" class="model-list">
          正在读取已安装模型…
        </div>
      </div>

      <div class="models-modal-footer">
        <button type="button" id="models-modal-done" class="primary-button">完成</button>
      </div>
    </div>
  `;

  function saveCurrentInputs() {
    const pathInput = overlay.querySelector<HTMLInputElement>('#whisper-path');
    const modelInput = overlay.querySelector<HTMLInputElement>('#whisper-model');
    const threadsInput = overlay.querySelector<HTMLInputElement>('#whisper-threads');
    if (pathInput) state.draft.whisperPath = pathInput.value.trim();
    if (modelInput) state.draft.whisperModel = modelInput.value.trim();
    if (threadsInput) {
      const val = threadsInput.value.trim();
      state.draft.whisperThreads = val === '' ? 0 : Math.max(0, parseInt(val, 10) || 0);
    }
  }

  function close() {
    saveCurrentInputs();
    overlay.remove();
    if (parent) parent.inert = false;
    if (onClose) onClose();
    const focusTarget = previousFocus instanceof HTMLElement && previousFocus.isConnected
      ? previousFocus : document.querySelector<HTMLElement>('#manage-local-models-btn');
    focusTarget?.focus({ preventScroll: true });
    scheduleLayout();
  }

  overlay.querySelector('#models-modal-close')?.addEventListener('click', close);
  overlay.querySelector('#models-modal-done')?.addEventListener('click', close);
  overlay.querySelector('#refresh-models-modal')?.addEventListener('click', () => void run(loadModels));

  // 与设置窗口共同销毁；Escape 统一交给全局最上层弹窗处理，避免同一按键连关两层。
  document.querySelector('#modal-root')!.appendChild(overlay);
  if (parent) parent.inert = true;
  overlay.querySelector<HTMLButtonElement>('#models-modal-close')?.focus({ preventScroll: true });
  scheduleLayout();
  void run(loadModels);
}
