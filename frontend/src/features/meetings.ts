import { call } from '../api/wails';
import { state } from '../app/state';
import { icon } from '../components/icons';
import { modal } from '../components/modal';
import { type Meeting } from '../types';
import { $ } from '../utils/dom';
import { clock, date, esc } from '../utils/format';
import { markdown } from '../utils/markdown';

export async function showMeetings() {
  const body = modal('会议记忆', '本机保存的对话与会后纪要', true);
  body.innerHTML = '<div class="empty-state">正在读取会议…</div>';
  state.meetings = (await call<Meeting[]>('ListMeetings')) || [];
  if (!document.querySelector('#modal-title') || $('#modal-title').textContent !== '会议记忆')
    return;
  if (!state.meetings.length) {
    body.innerHTML = `<div class="empty-state large-empty">${icon('notes')}<h3>重要的内容，留得住</h3><p>开始一场会话后，转录会保存在这里。<br/>结束时会生成纪要，方便下次接着聊。</p></div>`;
    return;
  }
  body.innerHTML = `<div class="meetings-layout"><nav class="meeting-list">${state.meetings.slice().reverse().map(m => `<button data-meeting="${esc(m.id)}"><strong>${esc(m.title || '未命名会话')}</strong><small>${date(m.startedAt)} · ${(m.turns || []).length} 条</small></button>`).join('')}</nav><div id="meeting-detail" class="meeting-detail"></div></div>`;
  renderMeeting(state.meetings[state.meetings.length - 1].id);
}

export function renderMeeting(id: string) {
  const meeting = state.meetings.find(m => m.id === id);
  if (!meeting)
    return;
  document.querySelectorAll('[data-meeting]').forEach(el => el.classList.toggle('active', (el as HTMLElement).dataset.meeting === id));
  $('#meeting-detail').innerHTML = `<div class="meeting-detail-head"><h3>${esc(meeting.title || '未命名会话')}</h3><div><button class="icon-button" data-export-meeting="${esc(id)}" title="导出会议">${icon('download')}</button><button class="icon-button danger" data-delete-meeting="${esc(id)}" title="删除会议">${icon('trash')}</button></div></div><p class="muted">${date(meeting.startedAt)}${meeting.endedAt ? ` — ${clock(meeting.endedAt)}` : ' · 当前会议，可继续记录'}</p>${!meeting.endedAt ? '<div class="callout">停止听写后，30 分钟内可以继续同一场会议。需要开始一份新记录时，可结束并清空当前转录。<br/><button class="text-button" data-end-meeting>结束并清空当前转录</button></div>' : ''}<div class="markdown meeting-notes">${meeting.notes ? markdown(meeting.notes) : '<p class="muted">暂无纪要。停止听写后，具备足够内容的对话会生成纪要。</p>'}</div><details class="meeting-transcript"><summary>查看完整转录 · ${(meeting.turns || []).length} 条</summary>${(meeting.turns || []).map(t => `<p><strong>${t.channel === 'them' ? '对方' : '我'} <time>${clock(t.time)}</time></strong>${esc(t.text)}</p>`).join('')}</details>`;
}
