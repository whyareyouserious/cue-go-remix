import { $ } from '../utils/dom';

export function toast(message: string, error = false) {
  const node = document.createElement('div');
  node.className = `toast ${error ? 'toast-error' : ''}`;
  node.dataset.hitRegion = '';
  node.textContent = message;
  $('#toast-region').append(node);
  window.setTimeout(() => node.remove(), error ? 7500 : 3500);
}

export async function run<T>(action: () => Promise<T>): Promise<T | undefined> {
  try {
    return await action();
  }
  catch (error) {
    toast(error instanceof Error ? error.message : String(error), true);
  }
}
