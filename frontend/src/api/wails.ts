import type { CueEvent } from '../types';
import { previewCall } from './preview';

export const preview = new URLSearchParams(location.search).get('preview') === '1';
let runtime: typeof import('/wails/runtime.js') | undefined;

export async function connect(onEvent: (event: CueEvent) => void) {
  if (preview) return;
  const runtimeURL = '/wails/runtime.js';
  const bridge = await import(/* @vite-ignore */ runtimeURL) as typeof import('/wails/runtime.js');
  runtime = bridge;
  bridge.Events.On('cue:event', event => onEvent(Array.isArray(event.data) ? event.data[0] : event.data));
}

export async function call<T = void>(method: string, ...args: unknown[]): Promise<T> {
  if (preview) return previewCall<T>(method, ...args);
  if (!runtime) throw new Error('尚未连接桌面服务，请通过 Cue 桌面应用启动。');
  return runtime.Call.ByName(`cue-go-remix/internal/controller.App.${method}`, ...args);
}

export async function copy(text: string) {
  if (runtime) await runtime.Clipboard.SetText(text);
  else await navigator.clipboard.writeText(text);
}

export async function openLink(url: string) {
  const parsed = new URL(url);
  if (!['https:', 'http:'].includes(parsed.protocol)) throw new Error('不支持的链接类型');
  if (runtime) await runtime.Browser.OpenURL(parsed.href);
  else window.open(parsed.href, '_blank', 'noopener,noreferrer');
}
