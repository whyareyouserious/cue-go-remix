export type ChatProtocol = 'chat-completions' | 'responses';

export interface Endpoint {
  provider: string; baseUrl: string; apiKey: string; hasKey: boolean;
  /** 聊天接口协议，省略时沿用服务商原协议；语音不使用。 */
  chatProtocol?: ChatProtocol;
  clearKey?: boolean; model: string; smartModel: string; stream: boolean;
  whisperModel?: string; whisperPath?: string;
}
export interface ProjectExperience {
  name: string; role: string; achievements: string;
}
export interface Profile {
  role: string; industry: string; years: string; skills: string;
  strengths: string; goals: string; projects: ProjectExperience[];
}
export interface Preferences {
  length: string; tone: string; structure: string; examples: string; custom: string;
}
export interface Scenario {
  id: string; name: string; profile: Profile; preferences: Preferences;
}
export interface Settings {
  chat: Endpoint; speech: Endpoint; smart: boolean; language: string; aiRules: string;
  chatChannels?: APIChannel[]; speechChannels?: APIChannel[];
  activeChatChannelId?: string; activeSpeechChannelId?: string;
  background: string; scenarios: Scenario[]; activeScenarioId: string;
  shortcuts?: Record<string, string>; meetingAudio: boolean; opacity: number; contentProtection: boolean;
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

export interface EndpointCheckResult {
  success: boolean;
  latency: number;
  message: string;
}

export interface APIChannel {
  id: string;
  name: string;
  provider: string;
  /** 独立渠道的聊天协议，保存设置后才生效。 */
  chatProtocol?: ChatProtocol;
  baseUrl: string;
  apiKey: string;
  hasKey: boolean;
  /** 后端提供的密钥脱敏预览，仅用于展示。 */
  maskedApiKey?: string;
  clearKey?: boolean;
  model: string;
  smartModel?: string;
  stream: boolean;
  createdAt?: string;
  latency?: number;
  status?: 'success' | 'failed' | 'untested';
}

export interface RecentError {
  id: string;
  time: number;
  level: 'error' | 'warn';
  category: string;
}

export interface DiagnosticsExport {
  appVersion: string;
  platform: string;
  arch: string;
  goVersion: string;
  uptimeSeconds: number;
  contentProtection: boolean;
  meetingAudio: boolean;
  chatProvider: string;
  chatModel: string;
  speechProvider: string;
  speechModel: string;
  recentErrors: RecentError[];
  recentLogEvents: string[];
  exportedAt: number;
}

export const initialSettings: Settings = {
  chat: { provider: 'openai', baseUrl: 'https://api.openai.com/v1', apiKey: '', hasKey: false, model: 'gpt-4o-mini', smartModel: 'gpt-4o', stream: true },
  speech: { provider: 'openai', baseUrl: 'https://api.openai.com/v1', apiKey: '', hasKey: false, model: 'whisper-1', smartModel: '', stream: false },
  smart: false, language: 'zh', aiRules: '', background: '',
  activeScenarioId: 'default',
  scenarios: [
    {
      id: 'default',
      name: '默认场景',
      profile: {
        role: '',
        industry: '',
        years: '',
        skills: '',
        strengths: '',
        goals: '',
        projects: [],
      },
      preferences: {
        length: '',
        tone: '',
        structure: '',
        examples: '',
        custom: '',
      },
    },
  ],
  shortcuts: {
    say: 'CmdOrCtrl+Enter',
    assist: 'CmdOrCtrl+Shift+Enter',
    leetcode: 'CmdOrCtrl+H',
    toggle: 'CmdOrCtrl+Shift+/',
    quit: 'CmdOrCtrl+Shift+X',
    passthrough: 'CmdOrCtrl+Shift+I',
  },
  meetingAudio: false, opacity: 1, contentProtection: true, slidesEnabled: false, slidesInterval: 3000,
  whisperModel: 'base', whisperPath: '', whisperThreads: 0, onboarded: false,
  windowX: 0, windowY: 0, windowPositioned: false,
};

export interface LicenseStatus {
  state: 'checking' | 'inactive' | 'license' | 'trial' | 'grace' | 'locked';
  allowed: boolean; mode: string; expiresAt: number; graceSeconds: number;
  message: string; saved: boolean; maskedKey?: string;
}
