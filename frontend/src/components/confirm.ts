import { scheduleLayout } from '../app/layout';
import { esc } from '../utils/format';

export function confirmAction(message: string): Promise<boolean> {
  return new Promise(resolve => {
    const previous = document.activeElement as HTMLElement | null;
    const overlay = document.createElement('div');
    overlay.className = 'confirm-backdrop';
    overlay.innerHTML = `<section class="confirm-dialog" data-hit-region role="alertdialog" aria-modal="true" aria-labelledby="confirm-title"><h3 id="confirm-title">确认操作</h3><p>${esc(message)}</p><div><button class="secondary-button" data-confirm-cancel>取消</button><button class="primary-button" data-confirm-accept>确认</button></div></section>`;
    const done = (accepted: boolean) => {
      overlay.remove(); if (previous?.isConnected)
        previous.focus(); scheduleLayout(); resolve(accepted);
    };
    overlay.querySelector('[data-confirm-cancel]')!.addEventListener('click', () => done(false));
    overlay.querySelector('[data-confirm-accept]')!.addEventListener('click', () => done(true));
    document.body.append(overlay);
    scheduleLayout();
    (overlay.querySelector('[data-confirm-cancel]') as HTMLElement).focus();
  });
}
