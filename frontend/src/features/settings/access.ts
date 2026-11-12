import { call } from '../../api/wails';
import { run } from '../../components/toast';
import type { Caller } from '../../types';
import { $ } from '../../utils/dom';
import { esc } from '../../utils/format';

export function renderAccessPage(page: HTMLElement) {
  page.innerHTML = '<h3>本地助手授权</h3><p class="section-description">其他本机程序需要你的授权才能读取状态或控制会话。页面内容的读取需要单独授权。</p><div id="caller-list">正在读取授权记录…</div>';
  void run(async () => {
    const callers = await call<Caller[]>('ListCallers');
    if (!document.querySelector('#caller-list'))
      return;
    $('#caller-list').innerHTML = callers?.length ? callers.map(c => `<div class="caller-row"><div><strong>${esc(c.name || c.id)}</strong><small>${esc(c.id)} · ${esc(c.scope || (c.scopes || []).join('、'))}</small></div><button class="text-button danger" data-revoke="${esc(c.id)}">撤销</button></div>`).join('') : '<div class="empty-state">还没有授权过其他本机程序。</div>';
  });
}
