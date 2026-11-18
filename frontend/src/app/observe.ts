import { $ } from '../utils/dom';
import { scheduleLayout } from './layout';

export function observeLayout() {
  new MutationObserver(scheduleLayout).observe($('#app'), { childList: true, subtree: true, attributes: true, attributeFilter: ['hidden', 'class'] });
  window.addEventListener('resize', scheduleLayout);
}
