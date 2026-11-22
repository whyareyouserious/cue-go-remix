import { call } from '../api/wails';
import { clampModalToBounds, updateModalBasePosition } from '../components/modal';
import { $ } from '../utils/dom';
import { state } from './state';

let layoutScheduled = false;

let lastLayout = '';

let layoutQueue = Promise.resolve();

let layoutEpoch = 0;

export interface OverlayViewport {
  maxWidth: number;
  maxHeight: number;
  workAreaX?: number;
  workAreaY?: number;
  windowX?: number;
  windowY?: number;
}

let currentViewport: OverlayViewport = {
  maxWidth: 1000,
  maxHeight: 800,
  workAreaX: 0,
  workAreaY: 0,
  windowX: 0,
  windowY: 0,
};

export function getOverlayViewport(): OverlayViewport {
  return currentViewport;
}

export function getScreenBounds() {
  return { width: currentViewport.maxWidth, height: currentViewport.maxHeight };
}

const layoutObserver = new ResizeObserver(scheduleLayout);

export function scheduleLayout() {
  if (layoutScheduled)
    return;
  layoutScheduled = true;
  requestAnimationFrame(() => {
    layoutScheduled = false;
    if (!state.connected)
      return;

    // 动态内容变化时按实际内容尺寸居中并校准屏内边界
    if (Boolean(state.activeModal)) {
      updateModalBasePosition();
      clampModalToBounds();
    }

    // 严禁包含全宽占位容器（如 .modal-backdrop, .confirm-backdrop, .models-modal-overlay），只统计具名交互内容的真实边界
    const regions = Array.from(document.querySelectorAll<HTMLElement>('[data-hit-region]:not(.modal-backdrop):not(.confirm-backdrop):not(.models-modal-overlay):not(.add-channel-overlay)')).flatMap(element => {
      layoutObserver.observe(element);
      if (!element.getClientRects().length)
        return [];
      const r = element.getBoundingClientRect();
      return r.width && r.height ? [{ x: Math.floor(r.x), y: Math.floor(r.y), width: Math.ceil(r.width), height: Math.ceil(r.height) }] : [];
    });

    const hasConfirm = Boolean(document.querySelector('.confirm-backdrop'));
    const hasSubModal = Boolean(document.querySelector('.models-modal-overlay, .add-channel-overlay'));
    const requestedFullscreen = Boolean(state.activeModal) || hasConfirm || hasSubModal;
    let width = -1;
    let height = -1;

    if (!requestedFullscreen) {
      width = state.licenseStatus.allowed && !state.collapsed && state.sidebar ? 1000 : 700;
      height = Math.min(currentViewport.maxHeight, Math.max(64, ...regions.map(r => r.y + r.height + 14)));
    }

    const signature = JSON.stringify([width, height, regions]);
    if (signature === lastLayout)
      return;
    lastLayout = signature;

    // 仅在确认 signature 改变并确实发起新请求后才推进 epoch，避免同几何 rAF 导致正在途中的有效响应被误弃
    const currentEpoch = ++layoutEpoch;
    layoutQueue = layoutQueue.then(async () => {
      const bounds = await call<OverlayViewport | undefined>('SetOverlayLayout', width, height, regions);
      if (!bounds || !(bounds.maxWidth > 0) || !(bounds.maxHeight > 0))
        return;

      // 严格忽略过期模式回授：若请求期间状态已改变（如打开后立即按 Escape 退出），坚决丢弃旧响应
      const nowActive = Boolean(state.activeModal) || Boolean(document.querySelector('.confirm-backdrop'));
      if (currentEpoch !== layoutEpoch || requestedFullscreen !== nowActive) {
        return;
      }

      const viewportChanged =
        currentViewport.maxWidth !== bounds.maxWidth ||
        currentViewport.maxHeight !== bounds.maxHeight ||
        currentViewport.workAreaX !== bounds.workAreaX ||
        currentViewport.workAreaY !== bounds.workAreaY ||
        (bounds.windowX !== undefined && currentViewport.windowX !== bounds.windowX) ||
        (bounds.windowY !== undefined && currentViewport.windowY !== bounds.windowY);

      currentViewport = {
        maxWidth: bounds.maxWidth,
        maxHeight: bounds.maxHeight,
        workAreaX: bounds.workAreaX ?? 0,
        workAreaY: bounds.workAreaY ?? 0,
        windowX: bounds.windowX ?? currentViewport.windowX ?? 0,
        windowY: bounds.windowY ?? currentViewport.windowY ?? 0,
      };

      document.documentElement.style.setProperty('--screen-width', `${currentViewport.maxWidth}px`);
      document.documentElement.style.setProperty('--screen-height', `${currentViewport.maxHeight}px`);

      const toolbarX = (currentViewport.windowX ?? 0) - (currentViewport.workAreaX ?? 0);
      const toolbarY = (currentViewport.windowY ?? 0) - (currentViewport.workAreaY ?? 0);
      document.documentElement.style.setProperty('--toolbar-x', `${toolbarX}px`);
      document.documentElement.style.setProperty('--toolbar-y', `${toolbarY}px`);

      const appEl = $('#app');
      const wasFullscreen = appEl.classList.contains('overlay-fullscreen');
      const currentModalActive = nowActive;
      const modeChanged = wasFullscreen !== currentModalActive;
      appEl.classList.toggle('overlay-fullscreen', currentModalActive);

      const column = Math.min(700, currentViewport.maxWidth);
      const panel = Math.min(624, column - 24);
      const sideRoom = currentViewport.maxWidth - (column + panel) / 2 - 12;
      $('.desktop').classList.toggle('sidebar-below', sideRoom < 180);
      document.documentElement.style.setProperty('--side-w', `${Math.max(180, Math.min(300, sideRoom))}px`);

      if (currentModalActive) {
        updateModalBasePosition();
      }

      if (clampModalToBounds()) {
        scheduleLayout();
        return;
      }
      if (viewportChanged || modeChanged) {
        scheduleLayout();
      }
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
