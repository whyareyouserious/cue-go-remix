import { call } from '../api/wails';
import { getOverlayViewport, getScreenBounds, scheduleLayout } from '../app/layout';
import { type ModalKind, state } from '../app/state';
import { $ } from '../utils/dom';
import { esc } from '../utils/format';
import { icon } from './icons';
import { run } from './toast';

export let modalOffset = { x: 0, y: 0 };

export function isModalDragged(): boolean {
  return modalOffset.x !== 0 || modalOffset.y !== 0;
}

export function updateModalBasePosition() {
  const modalEl = document.querySelector<HTMLElement>('.modal');
  if (!modalEl)
    return;
  // 已拖动状态下保持当前偏移，动态内容变化不强行重设基准点
  if (isModalDragged())
    return;

  const bounds = getScreenBounds();
  const screenW = bounds.width || window.innerWidth || 1000;
  const screenH = bounds.height || window.innerHeight || 800;

  const rect = modalEl.getBoundingClientRect();
  const isWide = modalEl.classList.contains('modal-wide') || modalEl.classList.contains('modal-settings');
  const modalW = rect.width || (isWide ? Math.min(900, screenW - 32) : 480);
  const modalH = rect.height || (isWide ? Math.min(680, screenH - 76) : 360);

  // 弹窗每次在悬浮条所在屏幕可用区域正中打开，不再跟随菜单条位置，按实际内容尺寸居中
  const rawBaseX = Math.round((screenW - modalW) / 2);
  const rawBaseY = Math.round((screenH - modalH) / 2);

  // 严格限制在工作区之内（安全边距 12px）
  const baseX = Math.max(12, Math.min(screenW - modalW - 12, rawBaseX));
  const baseY = Math.max(12, Math.min(screenH - modalH - 12, rawBaseY));

  modalEl.style.setProperty('--modal-base-x', `${baseX}px`);
  modalEl.style.setProperty('--modal-base-y', `${baseY}px`);
}

export function clampModalToBounds(): boolean {
  const modalEl = document.querySelector<HTMLElement>('.modal');
  if (!modalEl || !isModalDragged())
    return false;
  const bounds = getScreenBounds();
  const screenW = bounds.width || window.innerWidth || 1000;
  const screenH = bounds.height || window.innerHeight || 800;
  const r = modalEl.getBoundingClientRect();
  if (!r.width || !r.height)
    return false;

  let adjusted = false;
  let nextTx = modalOffset.x;
  let nextTy = modalOffset.y;

  // 右边界越界：拉回屏内（留 12px 边距）
  if (r.right > screenW - 12) {
    nextTx -= Math.ceil(r.right - (screenW - 12));
    adjusted = true;
  }
  // 左边界越界
  if (r.left < 12) {
    nextTx += Math.ceil(12 - r.left);
    adjusted = true;
  }
  // 下边界越界：拉回屏内（留 12px 边距）
  if (r.bottom > screenH - 12) {
    nextTy -= Math.ceil(r.bottom - (screenH - 12));
    adjusted = true;
  }
  // 上边界越界
  if (r.top < 12) {
    nextTy += Math.ceil(12 - r.top);
    adjusted = true;
  }

  if (adjusted) {
    modalOffset.x = nextTx;
    modalOffset.y = nextTy;
    modalEl.style.setProperty('--modal-tx', `${nextTx}px`);
    modalEl.style.setProperty('--modal-ty', `${nextTy}px`);
  }
  return adjusted;
}

export function setActiveModal(kind?: ModalKind) {
  state.activeModal = kind;
  $('#settings-toggle').setAttribute('aria-expanded', String(kind === 'settings'));
  $('#help').setAttribute('aria-expanded', String(kind === 'guide'));
}

export function modal(title: string, subtitle = '', wide = false, kind: ModalKind = 'other') {
  $('#opacity-popover').hidden = true;
  $('#opacity-toggle').setAttribute('aria-expanded', 'false');
  if (state.pendingConsent) {
    const id = state.pendingConsent;
    state.pendingConsent = '';
    void run(() => call('Consent', id, false));
  }
  setActiveModal(kind);
  state.restoreFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const root = $('#modal-root');
  const isSettings = kind === 'settings';
  const effectiveSubtitle = subtitle || (isSettings ? '可按住顶部自由移动 · 双击复位' : '');

  // 每次打开弹窗重置偏移量，确保初始状态绝对居中，无任何意外漂移
  modalOffset = { x: 0, y: 0 };

  root.innerHTML = `<div class="modal-backdrop">
    <section class="modal glass ${wide ? 'modal-wide' : ''} ${isSettings ? 'modal-settings' : ''}" data-hit-region role="dialog" aria-modal="true" aria-labelledby="modal-title">
      <header class="modal-header" title="按住可自由拖拽移动，双击复位">
        <div class="modal-header-info">
          <span class="modal-drag-grip" aria-hidden="true">${icon('drag')}</span>
          <div>
            <h2 id="modal-title">${esc(title)}</h2>
            ${effectiveSubtitle ? `<p class="modal-subtitle">${esc(effectiveSubtitle)}</p>` : ''}
          </div>
        </div>
        <button class="icon-button modal-close-btn" id="modal-close" title="关闭" aria-label="关闭">${icon('close')}</button>
      </header>
      <div id="modal-body" class="modal-body"></div>
      <footer id="modal-footer" class="modal-footer" hidden></footer>
    </section>
  </div>`;

  const modalEl = root.querySelector<HTMLElement>('.modal')!;
  const headerEl = root.querySelector<HTMLElement>('.modal-header')!;

  modalEl.style.setProperty('--modal-tx', '0px');
  modalEl.style.setProperty('--modal-ty', '0px');
  updateModalBasePosition();

  bindModalDrag(modalEl, headerEl);

  $('#modal-close').addEventListener('click', closeModal);
  $('.modal-backdrop').addEventListener('click', event => {
    if (event.target === event.currentTarget)
      closeModal();
  });
  // 宿主此时可能只有菜单条高度，聚焦不能滚动页面并扰乱尺寸上报。
  $<HTMLButtonElement>('#modal-close').focus({ preventScroll: true });
  scheduleLayout();
  return $('#modal-body');
}

function bindModalDrag(modalEl: HTMLElement, headerEl: HTMLElement) {
  let isDragging = false;
  let startX = 0;
  let startY = 0;
  let initialTx = 0;
  let initialTy = 0;

  // 双击标题栏快速重置居中
  headerEl.addEventListener('dblclick', event => {
    if ((event.target as HTMLElement).closest('button, input, select, textarea, a'))
      return;
    modalOffset = { x: 0, y: 0 };
    modalEl.style.setProperty('--modal-tx', '0px');
    modalEl.style.setProperty('--modal-ty', '0px');
    updateModalBasePosition();
    scheduleLayout();
  });

  headerEl.addEventListener('pointerdown', event => {
    if (event.button !== 0)
      return;
    if ((event.target as HTMLElement).closest('button, input, select, textarea, a, .icon-button'))
      return;

    isDragging = true;
    startX = event.clientX;
    startY = event.clientY;
    initialTx = modalOffset.x;
    initialTy = modalOffset.y;

    headerEl.classList.add('is-dragging');
    modalEl.classList.add('is-dragging');
    try {
      headerEl.setPointerCapture(event.pointerId);
    } catch {}
    event.preventDefault();
  });

  headerEl.addEventListener('pointermove', event => {
    if (!isDragging)
      return;

    const dx = event.clientX - startX;
    const dy = event.clientY - startY;
    let nextX = initialTx + dx;
    let nextY = initialTy + dy;

    // 四向边界安全限制：获取当前视口与屏幕尺寸
    const bounds = getScreenBounds();
    const screenW = bounds.width || window.innerWidth || 1000;
    const screenH = bounds.height || window.innerHeight || 800;

    const currentRect = modalEl.getBoundingClientRect();
    const modalW = currentRect.width;
    const modalH = currentRect.height;

    const isFullscreen = document.getElementById('app')?.classList.contains('overlay-fullscreen');
    let baseX = 0;
    let baseY = 0;
    if (isFullscreen) {
      baseX = parseFloat(modalEl.style.getPropertyValue('--modal-base-x')) || 0;
      baseY = parseFloat(modalEl.style.getPropertyValue('--modal-base-y')) || 0;
    } else {
      baseX = currentRect.left - modalOffset.x;
      baseY = currentRect.top - modalOffset.y;
    }

    // 左边界：工作区内至少 12px
    const minTx = 12 - baseX;
    // 右边界：工作区内至少 12px
    const maxTx = screenW - modalW - 12 - baseX;
    nextX = Math.max(minTx, Math.min(maxTx, nextX));

    // 上边界：工作区顶部至少 12px
    const minTy = 12 - baseY;
    // 下边界：工作区底部至少 12px
    const maxTy = screenH - modalH - 12 - baseY;
    nextY = Math.max(minTy, Math.min(maxTy, nextY));

    modalOffset.x = nextX;
    modalOffset.y = nextY;
    modalEl.style.setProperty('--modal-tx', `${nextX}px`);
    modalEl.style.setProperty('--modal-ty', `${nextY}px`);

    // 拖动过程中实时通知宿主更新命中区，保持零延迟
    scheduleLayout();
  });

  const stopDrag = (event: PointerEvent) => {
    if (!isDragging)
      return;
    isDragging = false;
    headerEl.classList.remove('is-dragging');
    modalEl.classList.remove('is-dragging');
    try {
      headerEl.releasePointerCapture(event.pointerId);
    } catch {}
    clampModalToBounds();
    scheduleLayout();
  };

  headerEl.addEventListener('pointerup', stopDrag);
  headerEl.addEventListener('pointercancel', stopDrag);
}

export function closeModal() {
  if (state.pendingConsent) {
    const id = state.pendingConsent;
    state.pendingConsent = '';
    void run(() => call('Consent', id, false));
  }
  // 关闭模态框时：严格先还原坐标与文档流，再进行尺寸测量与上报
  modalOffset.x = 0;
  modalOffset.y = 0;
  $('#modal-root').replaceChildren();
  $('#app').classList.remove('overlay-fullscreen');
  document.documentElement.style.setProperty('--toolbar-x', '0px');
  document.documentElement.style.setProperty('--toolbar-y', '0px');
  setActiveModal();

  // 坐标完全还原回常规紧凑原点后，再调度布局测量尺寸
  scheduleLayout();

  if (state.restoreFocus?.isConnected)
    state.restoreFocus.focus({ preventScroll: true });
}
