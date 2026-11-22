import { icon } from '../../components/icons';
import { toast } from '../../components/toast';
import type { APIChannel } from '../../types';
import { esc } from '../../utils/format';
import { createChannelKeyInput } from './channel-key-input';
import { scheduleLayout } from '../../app/layout';
import { parseChannelImport, prepareChannel } from './channel-import';

export interface PresetSpeechProvider {
  id: string;
  name: string;
  provider: string;
  baseUrl: string;
  model: string;
  stream: boolean;
  desc: string;
}

export const PRESET_SPEECH_PROVIDERS: PresetSpeechProvider[] = [
  { id: 'openai-whisper', name: 'OpenAI Whisper', provider: 'openai', baseUrl: 'https://api.openai.com/v1', model: 'whisper-1', stream: false, desc: '经典分段转写，适合高质量长语音' },
  { id: 'openai-realtime', name: 'OpenAI 实时流式', provider: 'openai', baseUrl: 'https://api.openai.com/v1', model: 'gpt-4o-mini-transcribe', stream: true, desc: '支持逐步吐字实时更新' },
  { id: 'deepgram', name: 'Deepgram Nova-3', provider: 'deepgram', baseUrl: 'https://api.deepgram.com/v1', model: 'nova-3', stream: true, desc: '极速语音云服务，支持实时与分段' },
  { id: 'groq-whisper', name: 'Groq Whisper', provider: 'groq', baseUrl: 'https://api.groq.com/openai/v1', model: 'whisper-large-v3', stream: false, desc: 'LPU 硬件极速分段转写' },
  { id: 'gemini-speech', name: 'Google Gemini', provider: 'gemini', baseUrl: 'https://generativelanguage.googleapis.com/v1beta', model: 'gemini-2.5-flash', stream: false, desc: 'Gemini 多模态语音理解' },
  { id: 'gemini-live', name: 'Gemini Live (实时)', provider: 'gemini', baseUrl: 'https://generativelanguage.googleapis.com/v1beta', model: 'gemini-3.5-transcribe-live', stream: true, desc: 'Gemini 实时流式连接' },
  { id: 'custom-speech', name: '第三方自定义中转', provider: 'custom', baseUrl: '', model: 'whisper-1', stream: false, desc: '兼容 OpenAI Audio 转写中转' },
];

/** 新增或编辑云端语音转写服务草稿；一次回调整批数据，真正持久化由设置保存负责。 */
export function openAddSpeechModal(onAdd: (channels: APIChannel[]) => void, initial?: APIChannel) {
  document.querySelector('.add-channel-overlay')?.remove();
  const overlay = document.createElement('div');
  overlay.className = 'add-channel-overlay glass-backdrop';
  overlay.tabIndex = -1;
  const first = PRESET_SPEECH_PROVIDERS[0];
  let selectedPreset: PresetSpeechProvider | undefined = initial
    ? PRESET_SPEECH_PROVIDERS.find(p => p.provider === initial.provider && p.baseUrl === initial.baseUrl && p.model === initial.model)
    : first;
  let currentTab: 'apikey' | 'import' = 'apikey';
  let importText = '';
  let draft: APIChannel = initial ? { ...initial } : {
    id: 'sp_' + crypto.randomUUID(), name: first.name, provider: first.provider,
    baseUrl: first.baseUrl, apiKey: '', hasKey: false, model: first.model,
    stream: first.stream, createdAt: new Date().toLocaleString(), status: 'untested',
  };

  const keyInput = createChannelKeyInput('speech', () => draft, initial);

  function readForm() {
    if (currentTab === 'import') {
      importText = overlay.querySelector<HTMLTextAreaElement>('#import-speech-text')?.value || '';
      return;
    }
    draft.name = overlay.querySelector<HTMLInputElement>('#new-speech-name')?.value || '';
    draft.baseUrl = overlay.querySelector<HTMLInputElement>('#new-speech-url')?.value || '';
    draft.model = overlay.querySelector<HTMLInputElement>('#new-speech-model')?.value || '';

    draft.stream = !!overlay.querySelector<HTMLInputElement>('#new-speech-stream')?.checked;
    draft.clearKey = !!overlay.querySelector<HTMLInputElement>('#channel-clear-key')?.checked;
  }

  function render() {
    overlay.innerHTML = `
      <div class="add-channel-modal card glass animate-in" style="pointer-events:auto" data-hit-region role="dialog" aria-modal="true" aria-labelledby="channel-dialog-title">
        <div class="add-channel-header">
          <h3 id="channel-dialog-title">${initial ? '编辑云端语音转写服务' : '添加云端语音转写服务'}</h3>
          <button type="button" class="icon-button close-btn" id="add-speech-close" aria-label="关闭">${icon('x')}</button>
        </div>

        <nav class="add-channel-tabs" role="tablist" ${initial ? 'hidden' : ''}>
          <button type="button" class="tab-item disabled" disabled title="功能准备中">
            <span class="tab-icon">💻</span> 官方登录
          </button>
          <button type="button" class="tab-item disabled" disabled title="功能准备中">
            <span class="tab-icon">🌐</span> OAuth 授权
          </button>
          <button type="button" class="tab-item disabled" disabled title="功能准备中">
            <span class="tab-icon">📄</span> Token / JSON
          </button>
          <button type="button" class="tab-item ${currentTab === 'apikey' ? 'active' : ''}" data-modal-tab="apikey">
            <span class="tab-icon">🔑</span> 调用 API Key
          </button>
          <button type="button" class="tab-item ${currentTab === 'import' ? 'active' : ''}" data-modal-tab="import">
            <span class="tab-icon">📥</span> 导入
          </button>
        </nav>

        <div class="add-channel-body">
          ${currentTab === 'apikey' ? `
            <div class="preset-section">
              <label class="section-label">快捷选择语音服务商预设（点击自动填充）</label>
              <div class="preset-chips">
                ${PRESET_SPEECH_PROVIDERS.map(p => `
                  <button type="button" class="preset-chip ${p.id === selectedPreset?.id ? 'active' : ''}" data-preset-id="${p.id}">
                    ${esc(p.name)}
                  </button>
                `).join('')}
              </div>
            </div>

            <div class="form-section">
              <div class="field">
                <label for="new-speech-key">调用 API Key <span id="channel-key-hint"></span></label>
                <div class="password-row">
                  <input id="new-speech-key" type="password" value="" placeholder="输入 API Key" autocomplete="new-password" spellcheck="false" />
                  <button id="new-speech-key-vis" type="button" class="icon-button" aria-label="显示/隐藏密钥">${icon('eye')}</button>
                </div>
                ${initial ? `<label class="check-label"><input id="channel-clear-key" type="checkbox" ${draft.clearKey ? 'checked' : ''}/>清除已保存密钥</label>` : ''}
              </div>

              <div class="field">
                <label for="new-speech-url">基础地址 (Base URL)</label>
                <input id="new-speech-url" type="url" value="${esc(draft.baseUrl)}" placeholder="不填写则是官方默认" autocomplete="off" spellcheck="false" />
              </div>

              <div class="field">
                <label for="new-speech-name">服务名称</label>
                <input id="new-speech-name" type="text" value="${esc(draft.name)}" placeholder="例如：OpenAI 语音主渠道" autocomplete="off" spellcheck="false" />
              </div>

              <div class="field">
                <label for="new-speech-model">转写模型名称</label>
                <input id="new-speech-model" type="text" value="${esc(draft.model)}" placeholder="如 whisper-1 或 nova-3" spellcheck="false" />
              </div>

              <label class="switch-row" style="margin-top: 6px; padding: 6px 0;">
                <span><strong>实时转写模式 (Stream)</strong><small>说话时逐步更新文字，需要提供商支持实时连接</small></span>
                <input id="new-speech-stream" type="checkbox" role="switch" ${draft.stream ? 'checked' : ''} />
              </label>
            </div>
          ` : `
            <div class="import-section">
              <div class="callout" style="margin-bottom: 12px;">
                <strong>语音配置导入格式说明：</strong>
                <p style="margin: 4px 0 0; font-size: 11.5px; color: var(--tx-mut);">
                  1. 单行格式：<code>服务名,sk-xxx,https://api.openai.com/v1,whisper-1</code><br/>
                  2. JSON 数组：<code>[{"name":"...", "apiKey":"...", "baseUrl":"...", "model":"..."}]</code>
                </p>
              </div>

              <div class="field">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px;">
                  <label for="import-speech-text" style="margin: 0;">粘贴配置内容</label>
                  <button type="button" id="fill-speech-example-btn" class="secondary-button btn-sm" style="font-size: 11px; padding: 2px 8px;">填入格式示例</button>
                </div>
                <textarea id="import-speech-text" rows="7" placeholder="在此粘贴语音服务配置..." style="font-family: var(--mono); font-size: 12px;">${esc(importText)}</textarea>
              </div>
            </div>
          `}
        </div>

        <div class="add-channel-footer">
          <button type="button" id="add-speech-cancel" class="secondary-button">取消</button>
          <button type="button" id="add-speech-submit" class="primary-button">
            ${currentTab === 'apikey' ? (initial ? '保存到草稿' : '加入草稿') : '导入到草稿'}
          </button>
        </div>
      </div>
    `;

    bindEvents();
    scheduleLayout();
  }

  function bindEvents() {
    overlay.querySelector('#add-speech-close')?.addEventListener('click', close);
    overlay.querySelector('#add-speech-cancel')?.addEventListener('click', close);
    overlay.querySelectorAll<HTMLButtonElement>('[data-modal-tab]').forEach(button => {
      button.addEventListener('click', () => {
        if (initial) return;
        readForm();
        currentTab = button.dataset.modalTab as 'apikey' | 'import';
        render();
      });
    });
    if (currentTab === 'apikey') {
      overlay.querySelectorAll<HTMLButtonElement>('[data-preset-id]').forEach(button => {
        button.addEventListener('click', () => {
          const found = PRESET_SPEECH_PROVIDERS.find(p => p.id === button.dataset.presetId);
          if (!found) return;
          readForm();
          const changed = found.provider !== draft.provider || found.baseUrl !== draft.baseUrl.trim();
          draft = { ...draft, name: found.name, provider: found.provider, baseUrl: found.baseUrl,
            model: found.model,  stream: found.stream };
          if (changed) { draft.apiKey = ''; draft.clearKey = false; }
          selectedPreset = found;
          keyInput.reset();
          render();
        });
      });
      keyInput.bind(overlay, '#new-speech-key', '#new-speech-key-vis');
      overlay.querySelector('#new-speech-url')?.addEventListener('input', () => {
        const before = draft.baseUrl.trim();
        readForm();
        if (before !== draft.baseUrl.trim()) {
          draft.apiKey = '';
          const keyInput = overlay.querySelector<HTMLInputElement>('#new-speech-key');
          if (keyInput) keyInput.value = '';
        }
        keyInput.reset();
      });
      overlay.querySelector('#channel-clear-key')?.addEventListener('change', () => {
        readForm();
        if (draft.clearKey) {
          draft.apiKey = '';
          const keyInput = overlay.querySelector<HTMLInputElement>('#new-speech-key');
          if (keyInput) keyInput.value = '';
        }
        keyInput.reset();
      });
    } else {
      overlay.querySelector('#fill-speech-example-btn')?.addEventListener('click', () => {
        const input = overlay.querySelector<HTMLTextAreaElement>('#import-speech-text');
        if (input) input.value = JSON.stringify([{name: '示例服务（请替换密钥）', provider: 'openai',
          baseUrl: 'https://api.openai.com/v1', apiKey: '请替换为自己的密钥',
          model: 'whisper-1', stream: false }], null, 2);
      });
    }
    overlay.querySelector('#add-speech-submit')?.addEventListener('click', () => {
      try {
        readForm();
        const channels = currentTab === 'import'
          ? parseChannelImport(importText, 'speech')
          : [prepareChannel(draft, 'speech', initial)];
        onAdd(channels);
        toast(initial ? '编辑已加入草稿，保存设置后生效' : `已将 ${channels.length} 个服务加入草稿，保存设置后生效`);
        close();
      } catch (error) {
        toast(error instanceof Error ? error.message : String(error), true);
      }
    });
  }

  function close() {
    keyInput.dispose();
    overlay.remove();
    scheduleLayout();
  }

  overlay.addEventListener('keydown', event => {
    if (event.key === 'Escape' && !event.isComposing) {
      event.preventDefault();
      event.stopPropagation();
      close();
    }
  });
  render();
  (document.querySelector('#modal-root') || document.body).appendChild(overlay);
  overlay.focus({ preventScroll: true });
  scheduleLayout();
}
