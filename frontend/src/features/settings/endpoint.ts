import { $ } from '../../utils/dom';
import { esc } from '../../utils/format';
import { icon } from '../../components/icons';
import { readSettingsPage } from './form';
import { recordUIEvent } from '../../app/diagnostics';
import { state } from '../../app/state';
import { type Endpoint } from '../../types';

export const providers: Record<string, {
  name: string;
  url: string;
  model: string;
  smart: string;
}> = {
  custom: { name: '第三方中转 / 自定义', url: '', model: '', smart: '' },
  openai: { name: 'OpenAI', url: 'https://api.openai.com/v1', model: 'gpt-4o-mini', smart: 'gpt-4o' },
  anthropic: { name: 'Anthropic', url: 'https://api.anthropic.com', model: 'claude-haiku-4-5-20251001', smart: 'claude-sonnet-4-5-20250929' },
  gemini: { name: 'Google Gemini', url: 'https://generativelanguage.googleapis.com/v1beta', model: 'gemini-2.5-flash', smart: 'gemini-2.5-pro' },
  azure: { name: 'Azure AI', url: '', model: '', smart: '' },
  deepseek: { name: 'DeepSeek', url: 'https://api.deepseek.com', model: 'deepseek-chat', smart: 'deepseek-reasoner' },
  groq: { name: 'Groq', url: 'https://api.groq.com/openai/v1', model: 'llama-3.1-8b-instant', smart: 'llama-3.3-70b-versatile' },
  cerebras: { name: 'Cerebras', url: 'https://api.cerebras.ai/v1', model: 'gpt-oss-120b', smart: 'gpt-oss-120b' },
  minimax: { name: 'MiniMax', url: 'https://api.minimax.io/v1', model: 'MiniMax-M2.5', smart: 'MiniMax-M2.5' },
  ollama: { name: 'Ollama（本机）', url: 'http://127.0.0.1:11434/v1', model: 'llama3.2', smart: 'llama3.2' },
};

export function endpointForm(endpoint: Endpoint, speech: boolean) {
  const available = speech ? ['sherpa', 'openai', 'custom', 'groq', 'gemini', 'deepgram', 'local'] : ['cerebras', 'openai', 'anthropic', 'gemini', 'groq', 'custom', 'ollama', 'minimax', 'deepseek', 'azure'];
  const options = available.map(p => `<button type="button" data-provider-value="${p}" aria-pressed="${endpoint.provider === p}">${p === 'custom' ? '第三方中转' : p === 'sherpa' ? '本地实时（中文）' : p === 'local' ? 'Whisper（分段）' : p === 'ollama' ? 'Ollama' : p === 'gemini' ? 'Gemini' : providers[p]?.name || 'Deepgram'}</button>`).join('');
  const local = ['local', 'sherpa'].includes(endpoint.provider);
  return `<div class="field"><span class="field-label" id="provider-label">${speech ? '语音转写服务' : '对话模型服务'}</span><input id="endpoint-provider" data-endpoint="provider" type="hidden" value="${esc(endpoint.provider)}"/><div class="provider-segments" role="group" aria-labelledby="provider-label">${options}</div></div>
    ${!local ? `<div class="field"><label for="endpoint-url">Base URL <span>服务地址</span></label><input id="endpoint-url" data-endpoint="baseUrl" type="url" value="${esc(endpoint.baseUrl)}" placeholder="https://your-gateway.example/v1" autocomplete="off" spellcheck="false"/><p class="field-hint">填入服务商提供的接口地址。自定义中转使用 OpenAI 兼容协议。</p></div>
    <div class="field"><label for="endpoint-key">API Key <span>${endpoint.hasKey ? '已保存，留空继续使用' : '保存在本机'}</span></label><div class="password-row"><input id="endpoint-key" data-endpoint="apiKey" type="password" value="${esc(endpoint.apiKey)}" placeholder="${endpoint.hasKey ? '••••••••（已保存）' : '输入服务商提供的 API Key'}" autocomplete="new-password" spellcheck="false"/><button id="key-visibility" type="button" class="icon-button" aria-label="显示密钥">${icon('eye')}</button></div><label class="check-label"><input id="clear-endpoint-key" type="checkbox" ${endpoint.clearKey ? 'checked' : ''}/>清除已保存密钥</label></div>
    <div class="field-grid"><div class="field"><label for="endpoint-model">${speech ? '转写模型' : '快速模型'}</label><input id="endpoint-model" data-endpoint="model" value="${esc(endpoint.model)}" placeholder="${speech ? 'whisper-1' : '服务商提供的模型名称'}" spellcheck="false"/></div>${!speech ? `<div class="field"><label for="endpoint-smart">深度思考模型</label><input id="endpoint-smart" data-endpoint="smartModel" value="${esc(endpoint.smartModel)}" placeholder="服务商提供的模型名称" spellcheck="false"/></div>` : ''}</div>
    ${!speech ? `<label class="switch-row"><span><strong>流式回答</strong><small>逐步显示回答；中转不支持时可关闭</small></span><input id="endpoint-stream" type="checkbox" role="switch" ${endpoint.stream ? 'checked' : ''}/></label>` : `<p class="field-hint">语音使用自己的接口与密钥，不会自动复用聊天密钥。</p>${['openai', 'deepgram', 'gemini'].includes(endpoint.provider) ? `<label class="switch-row"><span><strong>实时转写</strong><small>逐步更新文字；需要提供商支持实时语音连接</small></span><input id="endpoint-stream" type="checkbox" role="switch" ${endpoint.stream ? 'checked' : ''}/></label><p class="field-hint">${endpoint.provider === 'gemini' ? '实时模式需要 Gemini Live 模型（例如 gemini-3.5-transcribe-live），普通 Flash 模型用于分段转写。' : endpoint.provider === 'openai' ? '实时模式建议使用 gpt-4o-mini-transcribe；whisper-1 通常只提供最终文字。' : 'nova-3 支持实时和分段转写。'}</p>` : ''}`}` : `<div class="callout">${endpoint.provider === 'sherpa' ? '在本机实时识别中文，说话时持续更新字幕，停顿后确认整句。输出统一为简体中文，无需 API Key；运行时与模型随应用提供。' : '音频留在本机，由 whisper.cpp 分段转写。请在“本地模型”页准备模型和运行时；本地模式不会自动切换到云端。'}</div>`}
    ${speech ? `<label class="switch-row"><span><strong>采集系统音频</strong><small>捕获会议中对方的声音，与麦克风分开记录</small></span><input data-setting="meetingAudio" type="checkbox" role="switch" ${state.draft.meetingAudio ? 'checked' : ''}/></label><p class="field-hint">macOS 开启后需要屏幕录制权限，并显示系统录屏指示。设备或权限失败会显示错误。</p>` : '<p class="field-hint">模型名称由你的服务商决定。截图问答需要支持图片的模型。</p>'}`;
}

export function renderEndpointPage(page: HTMLElement, renderPage: () => void) {
  if (state.settingsTab !== 'chat' && state.settingsTab !== 'speech') return;
  page.innerHTML = `${endpointForm(state.draft[state.settingsTab], state.settingsTab === 'speech')}<button class="secondary-button" data-settings-page="${state.settingsTab === 'chat' ? 'access' : 'models'}">${state.settingsTab === 'chat' ? '本地助手授权' : 'Whisper 本地模型管理'}</button>`;
  page.querySelectorAll<HTMLButtonElement>('[data-provider-value]').forEach(button => button.addEventListener('click', () => {
    readSettingsPage();
    const endpoint = state.draft[state.settingsTab as 'chat' | 'speech'];
    const provider = button.dataset.providerValue!;
    recordUIEvent('provider-select', provider);
    if (endpoint.provider === provider)
      return;
    const preset = providers[provider];
    endpoint.provider = provider;
    endpoint.baseUrl = preset?.url || (provider === 'deepgram' ? 'https://api.deepgram.com/v1' : '');
    endpoint.model = state.settingsTab === 'speech' ? ({ openai: 'whisper-1', custom: 'whisper-1', groq: 'whisper-large-v3', gemini: 'gemini-2.5-flash', deepgram: 'nova-3', local: '' }[provider] || '') : preset?.model || '';
    endpoint.smartModel = preset?.smart || '';
    endpoint.apiKey = '';
    endpoint.hasKey = false;
    endpoint.clearKey = true;
    if (state.settingsTab === 'speech')
      endpoint.stream = provider === 'sherpa';
    renderPage();
    $('#settings-notice').textContent = ['local', 'sherpa'].includes(provider) ? '已切换本地服务，保存后生效' : '已切换服务，请填写对应密钥后保存';
  }));
  const endpointKind = state.settingsTab;
  document.querySelector('#endpoint-key')?.addEventListener('focus', () => recordUIEvent('key-focus', endpointKind));
  document.querySelector('#key-visibility')?.addEventListener('click', () => { const key = $<HTMLInputElement>('#endpoint-key'); key.type = key.type === 'password' ? 'text' : 'password'; });
  if (state.settingsTab === 'speech')
    document.querySelector('#endpoint-stream')?.addEventListener('change', () => {
      const enabled = $<HTMLInputElement>('#endpoint-stream').checked;
      const input = $<HTMLInputElement>('#endpoint-model');
      if (state.draft.speech.provider === 'gemini' && (input.value === 'gemini-2.5-flash' || input.value === 'gemini-3.5-transcribe-live'))
        input.value = enabled ? 'gemini-3.5-transcribe-live' : 'gemini-2.5-flash';
      if (state.draft.speech.provider === 'openai' && input.value === 'whisper-1' && enabled)
        input.value = 'gpt-4o-mini-transcribe';
    });
}
