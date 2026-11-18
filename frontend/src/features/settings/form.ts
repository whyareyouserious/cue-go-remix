import { state } from '../../app/state';

export function readSettingsPage() {
  const page = document.querySelector('#settings-page');
  if (!page)
    return;
  if (state.settingsTab === 'chat' || state.settingsTab === 'speech') {
    const endpoint = state.draft[state.settingsTab];
    const old = state.settings[state.settingsTab];
    page.querySelectorAll<HTMLInputElement>('[data-endpoint]').forEach(input => { (endpoint as unknown as Record<string, unknown>)[input.dataset.endpoint!] = input.value.trim(); });
    const clear = page.querySelector<HTMLInputElement>('#clear-endpoint-key');
    if (clear)
      endpoint.clearKey = clear.checked;
    if (endpoint.baseUrl !== old.baseUrl || endpoint.provider !== old.provider)
      endpoint.clearKey = true;
    const stream = page.querySelector<HTMLInputElement>('#endpoint-stream');
    if (stream)
      endpoint.stream = stream.checked;
  }
  page.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>('[data-setting]').forEach(input => {
    const key = input.dataset.setting!;
    let value: string | boolean | number = input.value;
    if (input instanceof HTMLInputElement) {
      if (input.type === 'checkbox')
        value = input.checked;
      if (input.type === 'number' || input.type === 'range')
        value = Number(input.value);
    }
    (state.draft as unknown as Record<string, unknown>)[key] = value;
  });
}
