import { state } from '../app/state';
import { icon } from '../components/icons';
import { modal } from '../components/modal';
import { $ } from '../utils/dom';
import { clock } from '../utils/format';
import { markdown } from '../utils/markdown';
import { openSettings } from './settings/index';

export function showSlides() {
  const body = modal('页面记录', '会议中的页面变化与内容摘要');
  body.innerHTML = '<div id="slides-content"></div>';
  renderSlides();
}

export function renderSlides() {
  const slides = state.status.slides || [];
  $('#slides-content').innerHTML = slides.length ? slides.map((slide, i) => `<article class="slide-card"><span class="eyebrow">页面 ${i + 1} · ${clock(slide.time)}</span><div class="markdown">${markdown(slide.text)}</div></article>`).join('') : `<div class="empty-state large-empty">${icon('screen')}<h3>让页面，也成为上下文</h3><p>在设置中开启“跟踪会议页面”，<br/>开始会话后自动记录变化的页面内容。</p><button class="secondary-button" id="configure-slides">前往设置</button></div>`;
  document.querySelector('#configure-slides')?.addEventListener('click', () => openSettings('appearance'));
}
