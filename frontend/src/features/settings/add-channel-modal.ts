import { icon } from '../../components/icons';
import { toast } from '../../components/toast';
import type { APIChannel } from '../../types';
import { esc } from '../../utils/format';
import { createChannelKeyInput } from './channel-key-input';
import { scheduleLayout } from '../../app/layout';
import { parseChannelImport, prepareChannel } from './channel-import';
import { supportsChatProtocol } from '../../utils/chat-protocol';

export interface PresetProvider {
  id: string;
  name: string;
  provider: string;
  baseUrl: string;
  model: string;
  smartModel: string;
  stream: boolean;
}

export const PRESET_PROVIDERS: PresetProvider[] = [
  { id: 'deepseek', name: 'DeepSeek', provider: 'deepseek', baseUrl: 'https://api.deepseek.com/v1', model: 'deepseek-chat', smartModel: 'deepseek-reasoner', stream: true },
  { id: 'openai', name: 'OpenAI', provider: 'openai', baseUrl: 'https://api.openai.com/v1', model: 'gpt-4o-mini', smartModel: 'gpt-4o', stream: true },
  { id: 'siliconflow', name: 'SiliconFlow (硅基流动)', provider: 'custom', baseUrl: 'https://api.siliconflow.cn/v1', model: 'deepseek-ai/DeepSeek-V3', smartModel: 'deepseek-ai/DeepSeek-R1', stream: true },
  { id: 'openrouter', name: 'OpenRouter', provider: 'custom', baseUrl: 'https://openrouter.ai/api/v1', model: 'google/gemini-2.5-flash', smartModel: 'deepseek/deepseek-r1', stream: true },
  { id: 'zhipu', name: 'Zhipu GLM (智谱)', provider: 'custom', baseUrl: 'https://open.bigmodel.cn/api/paas/v4', model: 'glm-4-flash', smartModel: 'glm-4-plus', stream: true },
  { id: 'moonshot', name: 'Moonshot (Kimi)', provider: 'custom', baseUrl: 'https://api.moonshot.cn/v1', model: 'moonshot-v1-8k', smartModel: 'moonshot-v1-32k', stream: true },
  { id: 'minimax', name: 'MiniMax', provider: 'minimax', baseUrl: 'https://api.minimax.io/v1', model: 'MiniMax-M2.5', smartModel: 'MiniMax-M2.5', stream: true },
  { id: 'gemini', name: 'Google Gemini', provider: 'gemini', baseUrl: 'https://generativelanguage.googleapis.com/v1beta', model: 'gemini-2.5-flash', smartModel: 'gemini-2.5-pro', stream: true },
  { id: 'anthropic', name: 'Anthropic Claude', provider: 'anthropic', baseUrl: 'https://api.anthropic.com', model: 'claude-haiku-4-5-20251001', smartModel: 'claude-sonnet-4-5-20250929', stream: true },
  { id: 'groq', name: 'Groq', provider: 'groq', baseUrl: 'https://api.groq.com/openai/v1', model: 'llama-3.1-8b-instant', smartModel: 'llama-3.3-70b-versatile', stream: true },
  { id: 'bailian', name: 'Bailian (阿里百炼)', provider: 'custom', baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1', model: 'qwen-plus', smartModel: 'qwen-max', stream: true },
  { id: 'stepfun', name: 'StepFun (阶跃星辰)', provider: 'custom', baseUrl: 'https://api.stepfun.com/v1', model: 'step-1-8k', smartModel: 'step-2-16k', stream: true },
  { id: 'packycode', name: 'PackyCode', provider: 'custom', baseUrl: 'https://api.packycode.com/v1', model: 'gpt-4o', smartModel: 'gpt-4o', stream: true },
  { id: 'aihubmix', name: 'AiHubMix', provider: 'custom', baseUrl: 'https://aihubmix.com/v1', model: 'gpt-4o-mini', smartModel: 'gpt-4o', stream: true },
  { id: 'ollama', name: 'Ollama (本机)', provider: 'ollama', baseUrl: 'http://127.0.0.1:11434/v1', model: 'llama3.2', smartModel: 'llama3.2', stream: true },
  { id: 'cerebras', name: 'Cerebras', provider: 'cerebras', baseUrl: 'https://api.cerebras.ai/v1', model: 'gpt-oss-120b', smartModel: 'gpt-oss-120b', stream: true },
  { id: 'custom', name: '自定义 / 兼容中转', provider: 'custom', baseUrl: '', model: '', smartModel: '', stream: true },
];

/** 新增或编辑 API 账号与渠道草稿；一次回调整批数据，真正持久化由设置保存负责。 */
export function openAddChannelModal(onAdd: (channels: APIChannel[]) => void, initial?: APIChannel) {
  document.querySelector('.add-channel-overlay')?.remove();
  const overlay = document.createElement('div');
  overlay.className = 'add-channel-overlay glass-backdrop';
  overlay.tabIndex = -1;
  const first = PRESET_PROVIDERS[0];
  let selectedPreset: PresetProvider | undefined = initial
    ? PRESET_PROVIDERS.find(p => p.provider === initial.provider && p.baseUrl === initial.baseUrl && p.model === initial.model)
    : first;
  let currentTab: 'apikey' | 'import' = 'apikey';
  let importText = '';
  let draft: APIChannel = initial ? { ...initial } : {
    id: 'ch_' + crypto.randomUUID(), name: first.name, provider: first.provider,
    baseUrl: first.baseUrl, apiKey: '', hasKey: false, model: first.model,
    smartModel: first.smartModel, stream: first.stream, createdAt: new Date().toLocaleString(), status: 'untested',
  };

  const keyInput = createChannelKeyInput('chat', () => draft, initial);

  function readForm() {
    if (currentTab === 'import') {
      importText = overlay.querySelector<HTMLTextAreaElement>('#import-text')?.value || '';
      return;
    }
    draft.name = overlay.querySelector<HTMLInputElement>('#new-channel-name')?.value || '';
    draft.baseUrl = overlay.querySelector<HTMLInputElement>('#new-channel-url')?.value || '';
    draft.chatProtocol = supportsChatProtocol(draft.provider)
      ? overlay.querySelector<HTMLSelectElement>('#new-channel-protocol')?.value as APIChannel['chatProtocol']
      : undefined;
    draft.model = overlay.querySelector<HTMLInputElement>('#new-channel-model')?.value || '';
    draft.smartModel = overlay.querySelector<HTMLInputElement>('#new-channel-smart')?.value || '';
    draft.stream = !!overlay.querySelector<HTMLInputElement>('#new-channel-stream')?.checked;
    draft.clearKey = !!overlay.querySelector<HTMLInputElement>('#channel-clear-key')?.checked;
  }

  function render() {
    overlay.innerHTML = `
      <div class="add-channel-modal card glass animate-in" style="pointer-events:auto" data-hit-region role="dialog" aria-modal="true" aria-labelledby="channel-dialog-title">
        <div class="add-channel-header">
          <h3 id="channel-dialog-title">${initial ? '编辑 API 账号与渠道' : '添加 API 账号与渠道'}</h3>
          <button type="button" class="icon-button close-btn" id="add-channel-close" aria-label="关闭">${icon('x')}</button>
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
              <label class="section-label">快捷选择服务商预设（点击自动填充配置）</label>
              <div class="preset-chips">
                ${PRESET_PROVIDERS.map(p => `
                  <button type="button" class="preset-chip ${p.id === selectedPreset?.id ? 'active' : ''}" data-preset-id="${p.id}">
                    ${esc(p.name)}
                  </button>
                `).join('')}
              </div>
            </div>

            <div class="form-section">
              <div class="field">
                <label for="new-channel-key">调用 API Key <span id="channel-key-hint"></span></label>
                <div class="password-row">
                  <input id="new-channel-key" type="password" value="" placeholder="输入 API Key" autocomplete="new-password" spellcheck="false" />
                  <button id="new-key-vis" type="button" class="icon-button" aria-label="显示/隐藏密钥">${icon('eye')}</button>
                </div>
                ${initial ? `<label class="check-label"><input id="channel-clear-key" type="checkbox" ${draft.clearKey ? 'checked' : ''}/>清除已保存密钥</label>` : ''}
              </div>

              <div class="field">
                <label for="new-channel-url">基础地址 (Base URL)</label>
                <input id="new-channel-url" type="url" value="${esc(draft.baseUrl)}" placeholder="不填写则是官方默认" autocomplete="off" spellcheck="false" />
              </div>

              ${supportsChatProtocol(draft.provider) ? `<div class="field">
                <label for="new-channel-protocol">聊天接口协议</label>
                <select id="new-channel-protocol">
                  <option value="chat-completions" ${draft.chatProtocol !== 'responses' ? 'selected' : ''}>Chat Completions</option>
                  <option value="responses" ${draft.chatProtocol === 'responses' ? 'selected' : ''}>Responses</option>
                </select>
                <p class="field-hint">按服务商支持的接口选择；仅支持 /v1/responses 的网关请选择 Responses。</p>
              </div>` : ''}

              <div class="field">
                <label for="new-channel-name">供应商名称</label>
                <input id="new-channel-name" type="text" value="${esc(draft.name)}" placeholder="例如：DeepSeek 生产渠道 / 自定义中转" autocomplete="off" spellcheck="false" />
              </div>

              <div class="field-grid">
                <div class="field">
                  <label for="new-channel-model">快速模型名称</label>
                  <input id="new-channel-model" type="text" value="${esc(draft.model)}" placeholder="如 gpt-4o-mini 或 deepseek-chat" spellcheck="false" />
                </div>
                <div class="field">
                  <label for="new-channel-smart">深度思考模型</label>
                  <input id="new-channel-smart" type="text" value="${esc(draft.smartModel || '')}" placeholder="如 deepseek-reasoner 或 gpt-4o" spellcheck="false" />
                </div>
              </div>

              <label class="switch-row" style="margin-top: 6px; padding: 6px 0;">
                <span><strong>流式传输响应 (Stream)</strong><small>实时逐字显示回答内容</small></span>
                <input id="new-channel-stream" type="checkbox" role="switch" ${draft.stream ? 'checked' : ''} />
              </label>
            </div>
          ` : `
            <div class="import-section">
              <div class="callout" style="margin-bottom: 12px;">
                <strong>支持的导入格式说明：</strong>
                <p style="margin: 4px 0 0; font-size: 11.5px; color: var(--tx-mut);">
                  1. 单条或多条格式：<code>渠道名称,sk-xxx,https://api.example.com/v1</code>（每行一条）<br/>
                  2. 标准 JSON 数组：<code>[{"name":"...", "apiKey":"...", "baseUrl":"...", "model":"...", "chatProtocol":"responses"}]</code>
                </p>
              </div>

              <div class="field">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px;">
                  <label for="import-text" style="margin: 0;">粘贴配置内容</label>
                  <button type="button" id="fill-example-btn" class="secondary-button btn-sm" style="font-size: 11px; padding: 2px 8px;">填入格式示例</button>
                </div>
                <textarea id="import-text" rows="7" placeholder="在此粘贴 API 账号配置..." style="font-family: var(--mono); font-size: 12px;">${esc(importText)}</textarea>
              </div>
            </div>
          `}
        </div>

        <div class="add-channel-footer">
          <button type="button" id="add-channel-cancel" class="secondary-button">取消</button>
          <button type="button" id="add-channel-submit" class="primary-button">
            ${currentTab === 'apikey' ? (initial ? '保存到草稿' : '加入草稿') : '导入到草稿'}
          </button>
        </div>
      </div>
    `;

    bindEvents();
    scheduleLayout();
  }

  function bindEvents() {
    overlay.querySelector('#add-channel-close')?.addEventListener('click', close);
    overlay.querySelector('#add-channel-cancel')?.addEventListener('click', close);
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
          const found = PRESET_PROVIDERS.find(p => p.id === button.dataset.presetId);
          if (!found) return;
          readForm();
          const changed = found.provider !== draft.provider || found.baseUrl !== draft.baseUrl.trim();
          draft = { ...draft, name: found.name, provider: found.provider, baseUrl: found.baseUrl,
            model: found.model, smartModel: found.smartModel, stream: found.stream, chatProtocol: undefined };
          if (changed) { draft.apiKey = ''; draft.clearKey = false; }
          selectedPreset = found;
          keyInput.reset();
          render();
        });
      });
      keyInput.bind(overlay, '#new-channel-key', '#new-key-vis');
      overlay.querySelector('#new-channel-url')?.addEventListener('input', () => {
        const before = draft.baseUrl.trim();
        readForm();
        if (before !== draft.baseUrl.trim()) {
          draft.apiKey = '';
          const keyInput = overlay.querySelector<HTMLInputElement>('#new-channel-key');
          if (keyInput) keyInput.value = '';
        }
        keyInput.reset();
      });
      overlay.querySelector('#channel-clear-key')?.addEventListener('change', () => {
        readForm();
        if (draft.clearKey) {
          draft.apiKey = '';
          const keyInput = overlay.querySelector<HTMLInputElement>('#new-channel-key');
          if (keyInput) keyInput.value = '';
        }
        keyInput.reset();
      });
    } else {
      overlay.querySelector('#fill-example-btn')?.addEventListener('click', () => {
        const input = overlay.querySelector<HTMLTextAreaElement>('#import-text');
        if (input) input.value = JSON.stringify([{name: '示例服务（请替换密钥）', provider: 'openai',
          baseUrl: 'https://api.openai.com/v1', apiKey: '请替换为自己的密钥',
          model: 'gpt-4o-mini', stream: true }], null, 2);
      });
    }
    overlay.querySelector('#add-channel-submit')?.addEventListener('click', () => {
      try {
        readForm();
        const channels = currentTab === 'import'
          ? parseChannelImport(importText, 'chat')
          : [prepareChannel(draft, 'chat', initial)];
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
