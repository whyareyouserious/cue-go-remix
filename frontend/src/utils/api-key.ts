import type { APIChannel } from '../types';

/** 密钥预览保留前后各四个字符；短密钥完全遮盖，防止拼接出完整凭据。 */
export function maskApiKey(key: string): string {
  const characters = Array.from(key);
  if (!characters.length) return '';
  if (characters.length <= 8) return '*****';
  return `${characters.slice(0, 4).join('')}*****${characters.slice(-4).join('')}`;
}

/** 优先展示当前草稿密钥的预览；清除或失效的凭据不能显示旧账号预览。 */
export function channelKeyPreview(channel: APIChannel): string {
  if (channel.clearKey) return '';
  if (channel.apiKey) return maskApiKey(channel.apiKey);
  return channel.hasKey ? channel.maskedApiKey || '*****' : '';
}
