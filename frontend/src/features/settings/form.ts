import { state } from '../../app/state';
import type { ProjectExperience } from '../../types';
import { parseShortcut } from '../../utils/shortcut';

export function readSettingsPage() {
  const page = document.querySelector('#settings-page');
  if (!page)
    return;
  if (state.settingsTab === 'chat' || state.settingsTab === 'speech') {
    const endpoint = state.draft[state.settingsTab];
    const old = state.settings[state.settingsTab];
    const inputs = page.querySelectorAll<HTMLInputElement>('[data-endpoint]');
    if (inputs.length > 0) {
      inputs.forEach(input => { (endpoint as unknown as Record<string, unknown>)[input.dataset.endpoint!] = input.value.trim(); });
      const clear = page.querySelector<HTMLInputElement>('#clear-endpoint-key');
      if (clear)
        endpoint.clearKey = clear.checked;
      if (endpoint.baseUrl !== old.baseUrl || endpoint.provider !== old.provider)
        endpoint.clearKey = true;
      const stream = page.querySelector<HTMLInputElement>('#endpoint-stream');
      if (stream)
        endpoint.stream = stream.checked;
    }
  }
  if (state.settingsTab === 'style') {
    const currentScenario = state.draft.scenarios?.find(s => s.id === state.draft.activeScenarioId) || state.draft.scenarios?.[0];
    if (currentScenario) {
      page.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>('[data-profile]').forEach(input => {
        (currentScenario.profile as unknown as Record<string, unknown>)[input.dataset.profile!] = input.value;
      });
      page.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>('[data-preference]').forEach(input => {
        (currentScenario.preferences as unknown as Record<string, unknown>)[input.dataset.preference!] = input.value;
      });
      const projectEls = page.querySelectorAll<HTMLElement>('.project-item');
      const projects: ProjectExperience[] = [];
      projectEls.forEach(item => {
        const name = item.querySelector<HTMLInputElement>('[data-project="name"]')?.value || '';
        const role = item.querySelector<HTMLInputElement>('[data-project="role"]')?.value || '';
        const achievements = item.querySelector<HTMLTextAreaElement>('[data-project="achievements"]')?.value || '';
        projects.push({ name, role, achievements });
      });
      currentScenario.profile.projects = projects;
    }
  }
  if (state.settingsTab === 'shortcuts') {
    if (!state.draft.shortcuts) {
      state.draft.shortcuts = structuredClone(state.settings.shortcuts || {});
    }
    page.querySelectorAll<HTMLInputElement>('input[data-shortcut]').forEach(input => {
      const k = input.dataset.shortcut!;
      state.draft.shortcuts![k] = parseShortcut(input.value, state.settings.shortcuts?.[k]);
    });
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
