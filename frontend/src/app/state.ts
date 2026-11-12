import { initialSettings, type LicenseStatus, type Meeting, type ModelInfo, type Status } from '../types';

export type ModalKind = 'settings' | 'guide' | 'other';

export const state = {
  settings: structuredClone(initialSettings),
  status: ({ platform: '', listening: false, busy: false, dataDir: '', transcript: [], slides: [], warnings: [] }) as Status,
  connected: false,
  licenseStatus: ({ state: 'checking', allowed: false, mode: '', expiresAt: 0, graceSeconds: 0, message: '正在检查本机授权', saved: false }) as LicenseStatus,
  licenseBusy: false,
  licenseRevision: 0,
  collapsed: true,
  sidebar: false,
  manualInput: false,
  fromTranscript: false,
  lastQuestion: '',
  activeAnswer: '',
  renderPending: false,
  settingsTab: 'chat',
  draft: structuredClone(initialSettings),
  models: ([]) as ModelInfo[],
  meetings: ([]) as Meeting[],
  interim: ({}) as Record<string, string>,
  liveQuestion: (undefined) as {
    prefix: string;
    suppressed: boolean;
  } | undefined,
  questionRevision: 0,
  pendingConsent: '',
  activeModal: (undefined) as ModalKind | undefined,
  restoreFocus: (null) as HTMLElement | null,
  userSpeechStart: 0,
  softClearTimer: (undefined) as number | undefined,
  answers: new Map<string, {
    text: string;
    element: HTMLElement;
    done: boolean;
    stopped?: boolean;
    dirty: boolean;
  }>(),
};
