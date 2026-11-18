import { state } from '../../app/state';
import { run } from '../../components/toast';
import { $ } from '../../utils/dom';
import { esc } from '../../utils/format';
import { loadModels } from '../models';

export function renderModelsPage(page: HTMLElement) {
  page.innerHTML = `<h3>Whisper 本地转写</h3><p class="section-description">先准备 whisper-server 运行时，再下载或导入模型。中文建议使用多语言模型，例如 base；带 .en 的模型只适合英语。</p><div class="field"><label for="whisper-path">whisper-server 路径</label><input id="whisper-path" data-setting="whisperPath" value="${esc(state.draft.whisperPath)}" placeholder="留空使用随应用提供的运行时" spellcheck="false"/></div><div class="field-grid"><div class="field"><label for="whisper-model">当前模型</label><input id="whisper-model" data-setting="whisperModel" value="${esc(state.draft.whisperModel)}" placeholder="base" spellcheck="false"/></div><div class="field"><label for="whisper-threads">计算线程 <span>0 为自动</span></label><input id="whisper-threads" data-setting="whisperThreads" type="number" min="0" max="64" value="${state.draft.whisperThreads}"/></div></div><div class="models-heading"><h4>模型库</h4><button id="refresh-models" class="text-button">刷新</button></div><div id="model-list" class="model-list">正在读取模型…</div>`;
  $('#refresh-models').addEventListener('click', () => void run(loadModels));
  void run(loadModels);
}
