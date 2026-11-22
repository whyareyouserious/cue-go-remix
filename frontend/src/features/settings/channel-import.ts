import type { APIChannel } from '../../types';
import { supportsChatProtocol } from '../../utils/chat-protocol';

export type ChannelKind = 'chat' | 'speech';

const providers: Record<ChannelKind, string[]> = {
  chat: ['openai', 'custom', 'anthropic', 'gemini', 'azure', 'deepseek', 'groq', 'cerebras', 'minimax', 'ollama'],
  speech: ['openai', 'custom', 'groq', 'gemini', 'deepgram'],
};

/** 校验渠道草稿；只有原渠道地址和服务商未变化时，空密钥才可保留原凭据。 */
export function prepareChannel(candidate: APIChannel, kind: ChannelKind, initial?: APIChannel): APIChannel {
  const value = { ...candidate };
  value.name = value.name.trim();
  value.provider = value.provider.trim().toLowerCase();
  value.baseUrl = value.baseUrl.trim();
  value.apiKey = value.apiKey.trim();
  value.model = value.model.trim();
  if (kind === 'chat') value.smartModel = value.smartModel?.trim() || value.model;
  if (!value.name) throw new Error('请填写服务名称');
  if (Array.from(value.name).length > 100) throw new Error('服务名称最多 100 个字符');
  if (!providers[kind].includes(value.provider)) throw new Error('不支持此服务商，请选择已有预设或自定义中转');
  if (value.chatProtocol) {
    if (kind !== 'chat') throw new Error('语音渠道不能设置聊天协议');
    if (!['chat-completions', 'responses'].includes(value.chatProtocol)) throw new Error('聊天协议仅支持 Chat Completions 或 Responses');
    if (!supportsChatProtocol(value.provider)) throw new Error('当前服务商使用原生协议，请清除兼容聊天协议设置');
  }
  if (!value.model) throw new Error('请填写模型名称');
  if (Array.from(value.model).length > 200 || Array.from(value.smartModel || '').length > 200)
    throw new Error('模型名称最多 200 个字符');
  if (kind === 'speech' && value.stream && !['openai', 'deepgram', 'gemini'].includes(value.provider))
    throw new Error('此语音服务暂不支持实时流式，请关闭实时开关');
  if (value.baseUrl) {
    let url: URL;
    try { url = new URL(value.baseUrl); } catch { throw new Error('服务地址格式不正确，请填写完整的 HTTP / HTTPS 地址'); }
    if (!['http:', 'https:'].includes(url.protocol) || !url.hostname || url.username || url.password || url.hash)
      throw new Error('服务地址仅支持 HTTP / HTTPS，不能包含账号密码或片段');
  } else if (['custom', 'azure'].includes(value.provider)) {
    throw new Error('请填写此服务的基础地址');
  }
  if (/[\r\n]/.test(value.apiKey)) throw new Error('API Key 不能包含换行');

  const sameEndpoint = !!initial && value.provider === initial.provider && value.baseUrl === initial.baseUrl;
  const explicitClear = value.clearKey === true;
  if (explicitClear) value.apiKey = '';
  if (!value.apiKey && sameEndpoint && !explicitClear) value.apiKey = initial.apiKey || '';
  value.hasKey = !!value.apiKey || (sameEndpoint && !explicitClear && !!initial.hasKey);
  value.clearKey = !value.apiKey && (explicitClear || (!!initial && !sameEndpoint));
  if (!value.hasKey && !['custom', 'ollama'].includes(value.provider) && !(initial && explicitClear))
    throw new Error(sameEndpoint ? '请填写 API Key' : '此服务需要 API Key；更换服务商或地址后请重新填写');

  if (initial) {
    value.id = initial.id;
    value.createdAt = initial.createdAt;
  }
  const changed = !initial || ['provider', 'baseUrl', 'chatProtocol', 'model', 'smartModel', 'stream', 'apiKey'].some(key =>
    value[key as keyof APIChannel] !== initial[key as keyof APIChannel]) || !!value.clearKey !== !!initial.clearKey;
  if (changed) {
    value.status = 'untested';
    delete value.latency;
  }
  return value;
}

function importString(item: Record<string, unknown>, key: string, fallback = ''): string {
  const value = item[key];
  if (value === undefined) return fallback;
  if (typeof value !== 'string') throw new Error(`${key} 必须是文本`);
  return value.trim();
}

/** 整批解析并校验 JSON 或逗号/制表符逐行格式；任一条失败则不返回部分结果。 */
export function parseChannelImport(content: string, kind: ChannelKind): APIChannel[] {
  const text = content.trim();
  if (!text) throw new Error('请先粘贴配置内容');
  let items: unknown[];
  if (/^[\[{]/.test(text)) {
    let parsed: unknown;
    try { parsed = JSON.parse(text); } catch { throw new Error('JSON 格式不正确，本次未导入任何服务'); }
    items = Array.isArray(parsed) ? parsed : [parsed];
  } else {
    items = text.split(/\r?\n/).filter(line => line.trim()).map((line, index) => {
      const parts = line.split(/[,，\t]/).map(part => part.trim());
      if (parts.length < 2 || parts.length > (kind === 'speech' ? 4 : 3))
        throw new Error(`第 ${index + 1} 行格式不正确，请按说明填写`);
      const [name, apiKey, baseUrl = '', model] = parts;
      return { name, apiKey, baseUrl, ...(model ? { model } : {}) };
    });
  }
  if (!items.length) throw new Error('配置列表不能为空');
  return items.map((item, index) => {
    try {
      if (!item || typeof item !== 'object' || Array.isArray(item)) throw new Error('每条配置必须是对象');
      const source = item as Record<string, unknown>;
      if (source.stream !== undefined && typeof source.stream !== 'boolean') throw new Error('stream 必须为 true 或 false');
      // 即使存在主字段，也检查别名类型，避免错误值被静默忽略。
      const aliasKey = importString(source, 'key');
      const aliasURL = importString(source, 'url');
      const smartModel = importString(source, 'smartModel');
      const provider = importString(source, 'provider', 'custom');
      return prepareChannel({
        id: `${kind === 'chat' ? 'ch' : 'sp'}_${crypto.randomUUID()}`,
        name: importString(source, 'name', provider),
        provider,
        chatProtocol: importString(source, 'chatProtocol') as APIChannel['chatProtocol'],
        baseUrl: importString(source, 'baseUrl', aliasURL),
        apiKey: importString(source, 'apiKey', aliasKey),
        hasKey: false,
        model: importString(source, 'model', kind === 'chat' ? 'gpt-4o-mini' : 'whisper-1'),
        ...(kind === 'chat' ? { smartModel } : {}),
        stream: source.stream === undefined ? kind === 'chat' : source.stream as boolean,
        createdAt: new Date().toLocaleString(),
        status: 'untested',
      }, kind);
    } catch (error) {
      throw new Error(`第 ${index + 1} 条配置：${error instanceof Error ? error.message : String(error)}；本次未导入任何服务`);
    }
  });
}
