import type { CueEvent } from './types';
import { initialSettings } from './types';

export const preview = new URLSearchParams(location.search).get('preview') === '1';
let runtime: typeof import('/wails/runtime.js') | undefined;
let previewSettings = structuredClone(initialSettings);

export async function connect(onEvent: (event: CueEvent) => void) {
  if (preview) return;
  const runtimeURL = '/wails/runtime.js';
  const bridge = await import(/* @vite-ignore */ runtimeURL) as typeof import('/wails/runtime.js');
  runtime = bridge;
  bridge.Events.On('cue:event', event => onEvent(Array.isArray(event.data) ? event.data[0] : event.data));
}

export async function call<T = void>(method: string, ...args: unknown[]): Promise<T> {
  if (preview) {
    if (method === 'GetSettings') return structuredClone(previewSettings) as T;
    if (method === 'SaveSettings') { previewSettings = structuredClone(args[0] as typeof initialSettings); return previewSettings as T; }
    if (method === 'GetStatus') return { platform: 'darwin', listening: false, busy: false, dataDir: '浏览器预览不保存数据', transcript: [], slides: [], warnings: [] } as T;
    if (method.startsWith('List')) return [] as T;
    if (['SetCollapsed', 'SetOverlayLayout', 'SetPassthrough', 'Cancel', 'StopListening', 'ClearTranscript'].includes(method)) return undefined as T;
    throw new Error('当前是界面预览，请在 Cue 桌面应用中使用此功能。');
  }
  if (!runtime) throw new Error('尚未连接桌面服务，请通过 Cue 桌面应用启动。');
  return runtime.Call.ByName(`main.App.${method}`, ...args);
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
