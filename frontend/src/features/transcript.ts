import { state } from '../app/state';
import { $ } from '../utils/dom';
import { clock, esc } from '../utils/format';

export function renderTranscript() {
  const turns = state.status.transcript || [];
  const list = $('#transcript-list');
  const follow = list.scrollHeight - list.scrollTop - list.clientHeight < 70;
  $('#transcript-count').textContent = `${turns.length} 条`;
  if (!turns.length) {
    list.innerHTML = '<div class="transcript-empty">开始会话后，对话记录会显示在这里。</div>';
    return;
  }
  list.innerHTML = turns.map(t => `<button class="turn turn-${t.channel === 'them' ? 'them' : 'you'}" data-turn="${esc(t.id)}" title="点击，针对这句话提问"><span class="turn-meta"><strong>${t.channel === 'them' ? '对方' : '我'}</strong><time>${clock(t.time)}</time></span><span class="turn-text">${esc(t.text)}</span></button>`).join('');
  if (follow)
    list.scrollTop = list.scrollHeight;
}

export function renderInterim() {
  $('#interim').innerHTML = Object.entries(state.interim).filter(([, text]) => text).map(([channel, text]) => `<p><span>${channel === 'them' ? '对方' : '我'} · 正在说</span>${esc(text)}</p>`).join('');
}
