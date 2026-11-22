import type { APIChannel } from '../types';

/** 这些提供商使用兼容接口，可按渠道选择 Chat Completions 或 Responses。 */
export function supportsChatProtocol(provider: string): boolean {
  return ['openai', 'custom', 'azure', 'cerebras', 'deepseek', 'groq', 'minimax', 'ollama'].includes(provider);
}

/** 展示渠道实际使用的协议；原生接口不显示为兼容接口。 */
export function chatProtocolLabel(channel: Pick<APIChannel, 'provider' | 'chatProtocol'>): string {
  if (!supportsChatProtocol(channel.provider)) return '原生协议';
  return channel.chatProtocol === 'responses' ? 'Responses' : 'Chat Completions';
}
