import { call } from '../api/wails';
import { state } from '../app/state';
import { icon } from '../components/icons';
import { type ModelInfo } from '../types';
import { $ } from '../utils/dom';
import { esc, size } from '../utils/format';

export async function loadModels() {
  state.models = (await call<ModelInfo[]>('ListModels')) || [];
  if (document.querySelector('#model-list'))
    renderModels();
}

export function renderModels() {
  $('#model-list').innerHTML = state.models.length ? state.models.map(m => {
    const percent = Math.min(100, Math.max(0, Math.round(m.downloaded / Math.max(1, m.bytes) * 100)));
    return `<div class="model-row"><div class="model-title"><div><strong>${esc(m.id)}</strong><small>${size(m.bytes)}${m.id.includes('.en') ? ' · 英语' : ' · 多语言'}${m.installed ? ' · 已安装' : ''}</small></div><div class="model-actions">${m.installed ? `<button data-model-select="${esc(m.id)}" class="text-button">选用</button><button data-model-delete="${esc(m.id)}" class="icon-button danger" title="删除模型">${icon('trash')}</button>` : m.downloading ? `<button data-model-cancel="${esc(m.id)}" class="text-button">取消</button>` : `<button data-model-import="${esc(m.id)}" class="icon-button" title="导入模型">${icon('upload')}</button><button data-model-download="${esc(m.id)}" class="icon-button" title="下载模型">${icon('download')}</button>`}</div></div>${m.downloading ? `<div class="model-progress"><progress value="${percent}" max="100"></progress><span>${percent}% · ${size(m.downloaded)}</span></div>` : ''}</div>`;
  }).join('') : '<div class="empty-state">暂无模型目录，请点击刷新。</div>';
}
