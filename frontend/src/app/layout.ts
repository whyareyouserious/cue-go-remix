import { call } from '../api/wails';
import { $ } from '../utils/dom';
import { state } from './state';

let layoutScheduled = false;

let lastLayout = '';

let layoutQueue = Promise.resolve();

let screenWidth = 1000;

let screenHeight = 800;

const layoutObserver = new ResizeObserver(scheduleLayout);

export function scheduleLayout() {
  if (layoutScheduled)
    return;
  layoutScheduled = true;
  requestAnimationFrame(() => {
    layoutScheduled = false;
    if (!state.connected)
      return;
    const regions = Array.from(document.querySelectorAll<HTMLElement>('[data-hit-region]')).flatMap(element => {
      layoutObserver.observe(element);
      if (!element.getClientRects().length)
        return [];
      const r = element.getBoundingClientRect();
      return r.width && r.height ? [{ x: Math.floor(r.x), y: Math.floor(r.y), width: Math.ceil(r.width), height: Math.ceil(r.height) }] : [];
    });
    const width = Math.min(screenWidth, state.licenseStatus.allowed && !state.collapsed && state.sidebar ? 1000 : 700);
    const height = Math.min(screenHeight, Math.max(64, ...regions.map(r => r.y + r.height + 14)));
    const signature = JSON.stringify([width, height, regions]);
    if (signature === lastLayout)
      return;
    lastLayout = signature;
    layoutQueue = layoutQueue.then(async () => {
      const bounds = await call<{
        maxWidth: number;
        maxHeight: number;
      } | undefined>('SetOverlayLayout', width, height, regions);
      if (!bounds || !(bounds.maxWidth > 0) || !(bounds.maxHeight > 0))
        return;
      if (screenWidth === bounds.maxWidth && screenHeight === bounds.maxHeight)
        return;
      screenWidth = bounds.maxWidth;
      screenHeight = bounds.maxHeight;
      document.documentElement.style.setProperty('--screen-width', `${screenWidth}px`);
      document.documentElement.style.setProperty('--screen-height', `${screenHeight}px`);
      const column = Math.min(700, screenWidth);
      const panel = Math.min(624, column - 24);
      const sideRoom = screenWidth - (column + panel) / 2 - 12;
      $('.desktop').classList.toggle('sidebar-below', sideRoom < 180);
      document.documentElement.style.setProperty('--side-w', `${Math.max(180, Math.min(300, sideRoom))}px`);
      scheduleLayout();
    }).catch(error => {
      lastLayout = '';
      console.error('调整悬浮窗口失败', error instanceof Error ? error.message : String(error));
    });
  });
}

export function resizeQuestion() {
  const input = $<HTMLTextAreaElement>('#question');
  input.style.height = '22px';
  input.style.height = `${Math.min(140, Math.max(22, input.scrollHeight))}px`;
  scheduleLayout();
}
