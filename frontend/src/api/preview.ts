import { initialSettings } from '../types';

let previewSettings = structuredClone(initialSettings);

export function previewCall<T>(method: string, ...args: unknown[]): T {
  if (method === 'GetSettings') return structuredClone(previewSettings) as T;
  if (method === 'SaveSettings') { previewSettings = structuredClone(args[0] as typeof initialSettings); return previewSettings as T; }
  if (method === 'GetStatus') return { platform: 'darwin', listening: false, busy: false, dataDir: '浏览器预览不保存数据', transcript: [], slides: [], warnings: [] } as T;
  if (method.startsWith('List')) return [] as T;
  if (['SetCollapsed', 'SetOverlayLayout', 'SetPassthrough', 'Cancel', 'StopListening', 'ClearTranscript'].includes(method)) return undefined as T;
  throw new Error('当前是界面预览，请在 Cue 桌面应用中使用此功能。');
}
