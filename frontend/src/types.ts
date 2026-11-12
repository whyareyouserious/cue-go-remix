export interface Endpoint {
  provider: string; baseUrl: string; apiKey: string; hasKey: boolean;
  clearKey?: boolean; model: string; smartModel: string; stream: boolean;
}
export interface Settings {
  chat: Endpoint; speech: Endpoint; smart: boolean; language: string; aiRules: string;
  background: string; meetingAudio: boolean; opacity: number; contentProtection: boolean;
  slidesEnabled: boolean; slidesInterval: number; whisperModel: string; whisperPath: string;
  whisperThreads: number; onboarded: boolean; windowX: number; windowY: number; windowPositioned: boolean;
}
export interface Turn { id: string; channel: string; text: string; time: number }
export interface Meeting { id: string; title: string; startedAt: number; endedAt: number; turns: Turn[]; notes: string }
export interface Slide { id: string; time: number; text: string }
export interface Status {
  platform: string; listening: boolean; busy: boolean; dataDir: string;
  transcript: Turn[]; slides: Slide[]; warnings: string[];
}
export interface ModelInfo { id: string; bytes: number; sha256: string; installed: boolean; downloading: boolean; downloaded: number }
export interface CueEvent { type: string; id?: string; text?: string; channel?: string; data?: unknown }
export interface Caller { id: string; name?: string; scope?: string; scopes?: string[] }

export const initialSettings: Settings = {
  chat: { provider: 'openai', baseUrl: 'https://api.openai.com/v1', apiKey: '', hasKey: false, model: 'gpt-4o-mini', smartModel: 'gpt-4o', stream: true },
  speech: { provider: 'openai', baseUrl: 'https://api.openai.com/v1', apiKey: '', hasKey: false, model: 'whisper-1', smartModel: '', stream: false },
  smart: false, language: 'zh', aiRules: '', background: '', meetingAudio: false,
  opacity: 1, contentProtection: true, slidesEnabled: false, slidesInterval: 3000,
  whisperModel: 'base', whisperPath: '', whisperThreads: 0, onboarded: false,
  windowX: 0, windowY: 0, windowPositioned: false,
};

export interface LicenseStatus {
  state: 'checking' | 'inactive' | 'license' | 'trial' | 'grace' | 'locked';
  allowed: boolean; mode: string; expiresAt: number; graceSeconds: number;
  message: string; saved: boolean; maskedKey?: string;
}
