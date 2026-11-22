import { state } from '../../app/state';
import { icon } from '../../components/icons';
import { run } from '../../components/toast';
import { $ } from '../../utils/dom';
import { esc } from '../../utils/format';
import { loadModels } from '../models';
import { readSettingsPage } from './form';
import { renderSettingsPage } from './index';

export function renderModelsPage(page: HTMLElement) {
  const currentThreads = state.draft.whisperThreads !== undefined ? state.draft.whisperThreads : 4;
  page.innerHTML = `
    <div class="models-header-row">
      <button type="button" id="models-back-btn" class="nav-back-icon-btn" title="返回音频服务" aria-label="返回音频服务">
        ${icon('arrow-left')}
      </button>
      <h3 class="models-title">Whisper 本地模型与运行时管理</h3>
    </div>
    <p class="section-description">准备 whisper-server 运行时，下载或导入多语言模型。中文建议使用 base 等多语言模型；以 .en 结尾的模型只适合英语。</p>
    <div class="field">
      <label for="whisper-path">whisper-server 路径 <span>留空使用随应用提供的运行时</span></label>
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
    <div class="models-heading">
      <h4>离线模型库</h4>
      <button id="refresh-models" class="text-button">刷新</button>
    </div>
    <div id="model-list" class="model-list">正在读取已安装模型…</div>
  `;

  $('#models-back-btn')?.addEventListener('click', () => {
    readSettingsPage();
    state.settingsTab = 'speech';
    renderSettingsPage();
  });

  $('#refresh-models')?.addEventListener('click', () => void run(loadModels));
  void run(loadModels);
}
