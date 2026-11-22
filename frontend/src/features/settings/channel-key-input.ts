import { call } from '../../api/wails';
import { toast } from '../../components/toast';
import type { APIChannel } from '../../types';
import { maskApiKey } from '../../utils/api-key';
import type { ChannelKind } from './channel-import';

/** 管理密钥显示与编辑：脱敏值仅用于显示，只有真实输入才写入草稿；完整已存密钥按点击读取。 */
export function createChannelKeyInput(kind: ChannelKind, getDraft: () => APIChannel, initial?: APIChannel) {
  let mode: 'masked' | 'password' | 'revealed' = initial?.hasKey || initial?.apiKey ? 'masked' : 'password';
  let revealedKey = '';
  let generation = 0;
  let loading = false;
  let input: HTMLInputElement | null = null;
  let button: HTMLButtonElement | null = null;
  let hint: HTMLElement | null = null;
  let clear: HTMLInputElement | null = null;

  function canUseSavedKey() {
    const draft = getDraft();
    return !!initial && !draft.clearKey && draft.provider === initial.provider && draft.baseUrl.trim() === initial.baseUrl;
  }

  function refresh() {
    if (!input || !button) return;
    const draft = getDraft();
    const raw = draft.clearKey ? '' : draft.apiKey || (canUseSavedKey() ? initial?.apiKey || revealedKey : '');
    draft.hasKey = !!raw || (canUseSavedKey() && !!initial?.hasKey);
    const preview = draft.hasKey ? (raw ? maskApiKey(raw) : initial?.maskedApiKey || '*****') : '';
    if (mode === 'masked' && !draft.hasKey) mode = 'password';
    input.type = mode === 'password' ? 'password' : 'text';
    input.readOnly = mode === 'masked';
    input.value = mode === 'masked' ? preview : raw;
    input.placeholder = draft.clearKey ? '密钥已清除' : '输入 API Key';
    button.disabled = loading;
    button.setAttribute('aria-pressed', String(mode === 'revealed'));
    button.setAttribute('aria-label', mode === 'revealed' ? '隐藏密钥' : '显示密钥');
    button.title = loading ? '正在读取密钥…' : mode === 'revealed' ? '隐藏密钥' : '显示密钥';
    if (hint) hint.textContent = mode === 'masked' ? '点击眼睛查看或修改' : mode === 'revealed' ? '再次点击眼睛隐藏' : ['custom', 'ollama'].includes(draft.provider) ? '可选，按服务要求填写' : '请填写此服务的密钥';
    if (clear) clear.checked = !!draft.clearKey;
  }

  /** 地址、提供商或清除标记变化时撤销读取，防止迟到响应填回旧账号密钥。 */
  function reset() {
    generation++;
    revealedKey = '';
    loading = false;
    mode = 'masked';
    refresh();
  }

  function bind(root: HTMLElement, inputSelector: string, buttonSelector: string) {
    generation++;
    loading = false;
    input = root.querySelector<HTMLInputElement>(inputSelector);
    button = root.querySelector<HTMLButtonElement>(buttonSelector);
    hint = root.querySelector('#channel-key-hint');
    clear = root.querySelector<HTMLInputElement>('#channel-clear-key');
    if (!input || !button) return;
    const boundInput = input;
    const boundButton = button;
    input.addEventListener('input', () => {
      generation++;
      loading = false;
      const draft = getDraft();
      draft.apiKey = boundInput.value;
      draft.clearKey = !draft.apiKey && !!initial?.hasKey;
      revealedKey = '';
      if (clear) clear.checked = !!draft.clearKey;
      // 正在输入时保持密码/明文模式，不把输入值替换成脱敏文本。
      draft.hasKey = !!draft.apiKey;
      boundButton.disabled = false;
    });
    button.addEventListener('click', async () => {
      if (mode === 'revealed') {
        mode = 'masked';
        revealedKey = '';
        refresh();
        return;
      }
      const draft = getDraft();
      if (draft.apiKey || (canUseSavedKey() && initial?.apiKey) || !draft.hasKey) {
        mode = 'revealed';
        refresh();
        return;
      }
      const requestGeneration = ++generation;
      loading = true;
      refresh();
      try {
        const key = await call<string>('GetChannelKey', kind, initial!.id);
        if (requestGeneration !== generation || !boundInput.isConnected || !canUseSavedKey()) return;
        if (!key) throw new Error('该账号尚未保存 API Key');
        revealedKey = key;
        mode = 'revealed';
      } catch {
        if (requestGeneration === generation && boundInput.isConnected) toast('读取密钥失败，请重试', true);
      } finally {
        if (requestGeneration === generation && boundInput.isConnected) {
          loading = false;
          refresh();
        }
      }
    });
    refresh();
  }

  return { bind, reset, dispose: reset };
}
