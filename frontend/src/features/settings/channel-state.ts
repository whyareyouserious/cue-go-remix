import { call, copy } from '../../api/wails';
import { state } from '../../app/state';
import { toast } from '../../components/toast';
import type { APIChannel, Endpoint, Settings } from '../../types';

export type ChannelKind = 'chat' | 'speech';

const legacyKeys = ['cue_api_channels_pool', 'cue_active_channel_id', 'cue_speech_channels_pool', 'cue_active_speech_channel_id'];
let pendingLegacy = new Map<string, string>();

export function emptyEndpoint(): Endpoint {
  return { provider: '', baseUrl: '', apiKey: '', hasKey: false, model: '', smartModel: '', stream: false };
}

/** 内置卡片只描述引擎，不将尚未检测的运行库或模型标为可用。 */
export function localSpeechChannels(): APIChannel[] {
  return [
    { id: 'local_sherpa', name: 'Sherpa 本地实时（中文专精）', provider: 'sherpa', baseUrl: '', apiKey: '', hasKey: false, model: 'zipformer', stream: true, createdAt: '内建引擎', status: 'untested' },
    { id: 'local_whisper', name: 'Whisper.cpp 本地分段引擎', provider: 'local', baseUrl: '', apiKey: '', hasKey: false, model: state.draft.whisperModel || 'base', stream: false, createdAt: '内建引擎', status: 'untested' },
  ];
}

/** 同步兼容端点供预览和表单使用；已生效配置只由保存成功的响应更新。 */
export function syncChannelDraft(kind: ChannelKind) {
  const channels = kind === 'chat' ? state.draft.chatChannels : [...localSpeechChannels(), ...(state.draft.speechChannels || [])];
  const activeId = kind === 'chat' ? state.draft.activeChatChannelId : state.draft.activeSpeechChannelId;
  const channel = channels?.find(item => item.id === activeId);
  state.draft[kind] = channel ? {
    provider: channel.provider, baseUrl: channel.baseUrl, apiKey: channel.apiKey || '', hasKey: channel.hasKey,
    chatProtocol: channel.chatProtocol,
    clearKey: channel.clearKey, model: channel.model, smartModel: channel.smartModel || channel.model, stream: channel.stream,
  } : emptyEndpoint();
}

/** 比较实际生效端点，卡片草稿修改后不再误标“正在使用”。 */
export function channelIsSaved(kind: ChannelKind, channel: APIChannel): boolean {
  const id = kind === 'chat' ? state.settings.activeChatChannelId : state.settings.activeSpeechChannelId;
  if (id !== channel.id) return false;
  const saved = state.settings[kind];
  return ['provider', 'baseUrl', 'model', 'stream'].every(key => saved[key as keyof Endpoint] === channel[key as keyof APIChannel])
    && (saved.chatProtocol || 'chat-completions') === (channel.chatProtocol || 'chat-completions')
    && (kind === 'speech' || saved.smartModel === (channel.smartModel || channel.model))
    && !channel.apiKey && !channel.clearKey;
}

/** 仅在点击复制时获取该账号凭据，完整密钥不放进卡片 DOM。 */
export async function copyChannelKey(kind: ChannelKind, channel: APIChannel) {
  try {
    if (channel.clearKey) throw new Error('该账号密钥已在草稿中清除');
    const key = channel.apiKey || (channel.hasKey ? await call<string>('GetChannelKey', kind, channel.id) : '');
    if (!key) throw new Error('该账号未提供密钥');
    await copy(key);
    toast('API Key 已复制');
  } catch (error) {
    toast(error instanceof Error ? error.message : String(error), true);
  }
}

/** 旧前端账号池只转入草稿；实际选择仍以当前后端配置为准，成功保存后才清理旧存储。 */
export function prepareChannelDraft() {
  state.draft.chatChannels ||= [];
  state.draft.speechChannels ||= [];
  pendingLegacy = new Map();
  try {
    const staged = { chat: structuredClone(state.draft.chatChannels), speech: structuredClone(state.draft.speechChannels) };
    for (const key of legacyKeys) {
      const value = localStorage.getItem(key);
      if (value !== null) pendingLegacy.set(key, value);
    }
    for (const [kind, key] of [['chat', legacyKeys[0]], ['speech', legacyKeys[2]]] as const) {
      const raw = pendingLegacy.get(key);
      if (!raw) continue;
      const parsed: unknown = JSON.parse(raw);
      if (!Array.isArray(parsed)) throw new Error('旧账号列表格式不正确');
      const target = staged[kind];
      for (const item of parsed) {
        if (!item || !['id', 'name', 'provider', 'baseUrl', 'apiKey', 'model'].every(field => typeof item[field] === 'string') || typeof item.stream !== 'boolean' || (item.smartModel !== undefined && typeof item.smartModel !== 'string')) {
          throw new Error('旧账号列表存在无效字段');
        }
        if (target.some(saved => saved.id === item.id)) continue;
        // 旧“转入当前配置”卡片只有 HasKey；映射到后端迁移出的同一账号才能继承其密钥。
        const existing = !item.apiKey && item.hasKey && target.find(saved => saved.provider === item.provider && saved.baseUrl === item.baseUrl && saved.model === item.model);
        if (existing) continue;
        target.push({ ...item, hasKey: !!item.apiKey, status: 'untested', latency: undefined });
      }
    }
    state.draft.chatChannels = staged.chat;
    state.draft.speechChannels = staged.speech;
    if (!state.draft.activeChatChannelId && staged.chat.length) {
      state.draft.activeChatChannelId = staged.chat[0].id;
      syncChannelDraft('chat');
    }
    if (!state.draft.activeSpeechChannelId && staged.speech.length) {
      state.draft.activeSpeechChannelId = staged.speech[0].id;
      syncChannelDraft('speech');
    }
    if (pendingLegacy.size) toast('旧账号列表已转入草稿，保存设置后完成迁移；当前服务保持不变');
  } catch {
    pendingLegacy.clear();
    toast('旧账号列表无法完整迁移，已保留原数据，请检查后重新导入', true);
  }
}

export function finishChannelMigration(saved: Settings) {
  if (!saved.chatChannels || !saved.speechChannels) return;
  try {
    for (const [key, value] of pendingLegacy) {
      if (localStorage.getItem(key) === value) localStorage.removeItem(key);
    }
    pendingLegacy.clear();
  } catch {
    toast('设置已保存；旧浏览器缓存未能清理，下次打开时会再次核对', true);
  }
}
