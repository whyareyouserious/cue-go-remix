import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { call, connect, copy, openLink, preview } from './api';
import { icon } from './icons';
import { initialSettings, type Caller, type CueEvent, type Endpoint, type Meeting, type ModelInfo, type Settings, type Slide, type Status, type Turn } from './types';
import './style.css';

const $ = <T extends HTMLElement = HTMLElement>(selector: string, root: ParentNode = document): T => {
  const node = root.querySelector<T>(selector);
  if (!node) throw new Error(`界面元素不存在：${selector}`);
  return node;
};
const esc = (value: unknown) => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!));
const date = (n: number) => new Date(n).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
const clock = (n: number) => new Date(n).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
const size = (n: number) => n >= 1e9 ? `${(n / 1e9).toFixed(2)} GB` : `${Math.round(n / 1e6)} MB`;
const markdown = (text: string) => DOMPurify.sanitize(marked.parse(text, { async: false, breaks: true, gfm: true }), {
  ALLOWED_TAGS: ['p', 'br', 'hr', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'strong', 'em', 'del', 's', 'blockquote', 'pre', 'code', 'ul', 'ol', 'li', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'a'],
  ALLOWED_ATTR: ['href', 'title', 'start', 'colspan', 'rowspan'],
  ALLOW_DATA_ATTR: false,
  ALLOW_ARIA_ATTR: false,
});

let settings = structuredClone(initialSettings);
let status: Status = { platform: '', listening: false, busy: false, dataDir: '', transcript: [], slides: [], warnings: [] };
let connected = false;
let collapsed = true;
let sidebar = false;
let manualInput = false;
let fromTranscript = false;
let lastQuestion = '';
let activeAnswer = '';
let renderPending = false;
let settingsTab = 'chat';
let draft = structuredClone(settings);
let models: ModelInfo[] = [];
let meetings: Meeting[] = [];
let interim: Record<string, string> = {};
let liveQuestion: { prefix: string; suppressed: boolean } | undefined;
let questionRevision = 0;
let pendingConsent = '';
type ModalKind = 'settings' | 'guide' | 'other';
let activeModal: ModalKind | undefined;
let restoreFocus: HTMLElement | null = null;
let userSpeechStart = 0;
let softClearTimer: number | undefined;
const answers = new Map<string, { text: string; element: HTMLElement; done: boolean; stopped?: boolean; dirty: boolean }>();
const providers: Record<string, { name: string; url: string; model: string; smart: string }> = {
  custom: { name: '第三方中转 / 自定义', url: '', model: '', smart: '' },
  openai: { name: 'OpenAI', url: 'https://api.openai.com/v1', model: 'gpt-4o-mini', smart: 'gpt-4o' },
  anthropic: { name: 'Anthropic', url: 'https://api.anthropic.com', model: 'claude-haiku-4-5-20251001', smart: 'claude-sonnet-4-5-20250929' },
  gemini: { name: 'Google Gemini', url: 'https://generativelanguage.googleapis.com/v1beta', model: 'gemini-2.5-flash', smart: 'gemini-2.5-pro' },
  azure: { name: 'Azure AI', url: '', model: '', smart: '' },
  deepseek: { name: 'DeepSeek', url: 'https://api.deepseek.com', model: 'deepseek-chat', smart: 'deepseek-reasoner' },
  groq: { name: 'Groq', url: 'https://api.groq.com/openai/v1', model: 'llama-3.1-8b-instant', smart: 'llama-3.3-70b-versatile' },
  cerebras: { name: 'Cerebras', url: 'https://api.cerebras.ai/v1', model: 'gpt-oss-120b', smart: 'gpt-oss-120b' },
  minimax: { name: 'MiniMax', url: 'https://api.minimax.io/v1', model: 'MiniMax-M2.5', smart: 'MiniMax-M2.5' },
  ollama: { name: 'Ollama（本机）', url: 'http://127.0.0.1:11434/v1', model: 'llama3.2', smart: 'llama3.2' },
};

$('#app').innerHTML = `
  <div class="desktop collapsed ${preview ? 'preview' : ''}" data-hit-region>
    <header id="toolbar" class="toolbar" data-hit-region aria-label="窗口工具栏">
      <div class="drag-pill drag-region" title="拖动窗口"><span class="drag-dots" aria-hidden="true"></span><span>拖动</span></div>
      <button id="help" class="tb-logo icon-button" title="使用指南" aria-label="使用指南" aria-expanded="false">${icon('badge-question-mark')}</button>
      <button id="settings-toggle" class="tb-settings icon-button" title="设置（⌘ / Ctrl + ,）" aria-label="设置" aria-expanded="false">${icon('settings')}</button>
      <span class="tb-divider"></span>
      <button id="collapse-toggle" class="tb-hide collapsed" title="显示面板" aria-label="显示面板" aria-expanded="false">${icon('chevron')}<span>显示</span></button>
      <span class="tb-divider"></span>
      <button id="session-toggle" class="tb-stop" disabled>${icon('play')}<span>开始会话</span></button>
      <span class="tb-divider"></span>
      <div class="tb-opacity-wrap"><button id="opacity-toggle" class="tb-opacity icon-button" title="窗口透明度" aria-label="调整透明度" aria-expanded="false">${icon('eclipse')}</button>
        <div id="opacity-popover" class="popover" data-hit-region hidden><label for="quick-opacity">透明度 <output id="opacity-value">100%</output></label><input id="quick-opacity" type="range" min="20" max="100" value="100" /></div>
      </div>
      <span class="tb-divider"></span>
      <button id="quit" class="tb-quit icon-button" title="退出 Cue" aria-label="退出 Cue">${icon('close')}</button>
    </header>
    <main id="workspace" class="workspace" hidden>
      <div class="workspace-drag-area" data-hit-region aria-hidden="true"></div>
      <span id="listen-dot" class="listen-dot"></span>
      <span id="connection-status" class="status-label">正在连接</span>
      <section id="panel" class="assistant-panel glass" data-hit-region aria-label="AI 助手">
        <div id="preview-note" class="notice" ${preview ? '' : 'hidden'}>界面预览 · 未连接 AI 或音频设备，配置仅保存在当前页面内存。</div>
        <div id="warning-banner" class="notice warning" hidden></div>
        <div id="messages" class="messages" aria-live="polite" aria-relevant="additions text"></div>
        <div class="composer-area">
          <div id="action-row" class="quick-actions"><button data-mode="say" class="act act-primary">${icon('wand-sparkles')}我该怎么说？ <kbd id="say-key">⌘ ↵</kbd></button><span class="sep">•</span><button data-mode="assist" class="act act-secondary">${icon('screen')}智能辅助 <kbd id="assist-key">⌘ ⇧ ↵</kbd></button><span class="sep">•</span><button data-mode="recap" class="act">${icon('refresh-cw')}回顾</button></div>
          <div id="interim" class="interim" aria-live="polite"></div>
          <form id="composer" class="composer">
            <div id="question-source" class="question-source" hidden>↵ 回答对方的问题 <button type="button" id="question-reset">清空</button></div>
            <textarea id="question" rows="1" placeholder="询问屏幕或对话内容，或按 ⌘ ⇧ ↵ 智能辅助" aria-label="输入问题" spellcheck="false"></textarea>
            <div id="composer-bottom" class="composer-bottom"><button id="smart-toggle" class="smart-pill" type="button" aria-pressed="false">${icon('zap')}智能</button><button id="sidebar-toggle" class="history-btn" type="button" aria-pressed="false">${icon('history')}转录历史</button><button id="more-settings" class="more-btn icon-button" type="button" title="设置" aria-label="更多设置">${icon('more-horizontal')}</button><span id="model-label" class="model-label" hidden></span><span class="spacer"></span><button id="cancel-answer" class="cancel-answer" type="button" hidden>${icon('stop')}停止</button><button id="send" class="send-button" type="submit" disabled title="发送（Enter），换行（Shift + Enter）" aria-label="发送">${icon('play')}</button></div>
          </form>
        </div>
      </section>
      <aside id="transcript-panel" class="transcript-panel" data-hit-region aria-label="转录历史" hidden>
        <div class="transcript-heading"><span>转录历史</span><button id="close-sidebar" class="icon-button" aria-label="关闭转录历史">${icon('close')}</button></div>
        <div id="transcript-list" class="transcript-list"></div>
        <div class="transcript-footer"><span id="transcript-count">0 条</span><button id="clear-transcript" class="text-button" title="结束当前记录并清空转录" aria-label="清空当前转录">清空历史</button></div>
        <span id="audio-description" hidden></span>
      </aside>
    </main>
  </div>
  <div id="modal-root"></div>
  <div id="toast-region" role="status" aria-live="polite"></div>
`;

let layoutScheduled = false;
let lastLayout = '';
let layoutQueue = Promise.resolve();
let screenWidth = 1000;
let screenHeight = 800;
const layoutObserver = new ResizeObserver(scheduleLayout);
function scheduleLayout() {
  if (layoutScheduled) return;
  layoutScheduled = true;
  requestAnimationFrame(() => {
    layoutScheduled = false;
    if (!connected) return;
    const regions = Array.from(document.querySelectorAll<HTMLElement>('[data-hit-region]')).flatMap(element => {
      layoutObserver.observe(element);
      if (!element.getClientRects().length) return [];
      const r = element.getBoundingClientRect();
      return r.width && r.height ? [{ x: Math.floor(r.x), y: Math.floor(r.y), width: Math.ceil(r.width), height: Math.ceil(r.height) }] : [];
    });
    const width = Math.min(screenWidth, !collapsed && sidebar ? 1000 : 700);
    const height = Math.min(screenHeight, Math.max(64, ...regions.map(r => r.y + r.height + 14)));
    const signature = JSON.stringify([width, height, regions]);
    if (signature === lastLayout) return;
    lastLayout = signature;
    layoutQueue = layoutQueue.then(async () => {
      const bounds = await call<{ maxWidth: number; maxHeight: number } | undefined>('SetOverlayLayout', width, height, regions);
      if (!bounds || !(bounds.maxWidth > 0) || !(bounds.maxHeight > 0)) return;
      if (screenWidth === bounds.maxWidth && screenHeight === bounds.maxHeight) return;
      screenWidth = bounds.maxWidth; screenHeight = bounds.maxHeight;
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
new MutationObserver(scheduleLayout).observe($('#app'), { childList: true, subtree: true, attributes: true, attributeFilter: ['hidden', 'class'] });
window.addEventListener('resize', scheduleLayout);

function toast(message: string, error = false) {
  const node = document.createElement('div');
  node.className = `toast ${error ? 'toast-error' : ''}`;
  node.dataset.hitRegion = '';
  node.textContent = message;
  $('#toast-region').append(node);
  window.setTimeout(() => node.remove(), error ? 7500 : 3500);
}

async function run<T>(action: () => Promise<T>): Promise<T | undefined> {
  try { return await action(); } catch (error) { toast(error instanceof Error ? error.message : String(error), true); }
}

function recordUIEvent(event: 'settings-open' | 'provider-select' | 'key-focus' | 'settings-save' | 'panel-toggle' | 'sidebar-toggle', value: string) {
  // 仅记录固定操作名和类别；日志失败不影响交互，不传入任何表单内容。
  void call('RecordUIEvent', event, value).catch(() => {});
}

function confirmAction(message: string): Promise<boolean> {
  return new Promise(resolve => {
    const previous = document.activeElement as HTMLElement | null;
    const overlay = document.createElement('div');
    overlay.className = 'confirm-backdrop';
    overlay.innerHTML = `<section class="confirm-dialog" data-hit-region role="alertdialog" aria-modal="true" aria-labelledby="confirm-title"><h3 id="confirm-title">确认操作</h3><p>${esc(message)}</p><div><button class="secondary-button" data-confirm-cancel>取消</button><button class="primary-button" data-confirm-accept>确认</button></div></section>`;
    const done = (accepted: boolean) => { overlay.remove(); if (previous?.isConnected) previous.focus(); scheduleLayout(); resolve(accepted); };
    overlay.querySelector('[data-confirm-cancel]')!.addEventListener('click', () => done(false));
    overlay.querySelector('[data-confirm-accept]')!.addEventListener('click', () => done(true));
    document.body.append(overlay);
    scheduleLayout();
    (overlay.querySelector('[data-confirm-cancel]') as HTMLElement).focus();
  });
}

function renderStatus() {
  const label = $('#connection-status');
  label.className = `status-label ${status.listening ? 'is-live' : ''}`;
  label.textContent = !connected ? '桌面服务未连接' : status.listening ? '正在转录' : preview ? '预览模式' : '准备就绪 · 转录关闭';
  $('#session-toggle').innerHTML = `${icon(status.listening ? 'stop' : 'play')}<span>${status.listening ? '结束会话' : '开始会话'}</span>`;
  $('#session-toggle').classList.toggle('is-live', status.listening);
  $('#listen-dot').classList.toggle('is-live', status.listening);
  $<HTMLButtonElement>('#session-toggle').disabled = !connected;
  $<HTMLButtonElement>('#send').disabled = !connected || status.busy;
  $('#cancel-answer').hidden = !status.busy;
  $('#smart-toggle').setAttribute('aria-pressed', String(settings.smart));
  $('#composer').classList.toggle('listening', status.listening);
  $('#model-label').textContent = (settings.smart ? settings.chat.smartModel : settings.chat.model) || '请设置模型';
  $('#audio-description').textContent = `麦克风 · ${settings.meetingAudio ? '系统音频开启' : '系统音频关闭'}`;
  $('#say-key').textContent = status.platform === 'darwin' ? '⌘ ↵' : 'Ctrl ↵';
  $('#assist-key').textContent = status.platform === 'darwin' ? '⌘ ⇧ ↵' : 'Ctrl ⇧ ↵';
  $<HTMLTextAreaElement>('#question').placeholder = `询问屏幕或对话内容，或按 ${status.platform === 'darwin' ? '⌘' : 'Ctrl'} ⇧ ↵ 智能辅助`;
  const slideCount = document.querySelector('#slide-count');
  if (slideCount) slideCount.textContent = String((status.slides || []).length);
  const warnings = status.warnings || [];
  $('#warning-banner').textContent = warnings.join('；');
  $('#warning-banner').hidden = warnings.length === 0;
  document.documentElement.style.setProperty('--cue-opacity', String(Math.min(1, Math.max(.2, settings.opacity || 1))));
  $<HTMLInputElement>('#quick-opacity').value = String(Math.round(settings.opacity * 100));
  $('#opacity-value').textContent = `${Math.round(settings.opacity * 100)}%`;
  scheduleLayout();
}

function renderTranscript() {
  const turns = status.transcript || [];
  const list = $('#transcript-list');
  const follow = list.scrollHeight - list.scrollTop - list.clientHeight < 70;
  $('#transcript-count').textContent = `${turns.length} 条`;
  if (!turns.length) {
    list.innerHTML = '<div class="transcript-empty">开始会话后，对话记录会显示在这里。</div>';
    return;
  }
  list.innerHTML = turns.map(t => `<button class="turn turn-${t.channel === 'them' ? 'them' : 'you'}" data-turn="${esc(t.id)}" title="点击，针对这句话提问"><span class="turn-meta"><strong>${t.channel === 'them' ? '对方' : '我'}</strong><time>${clock(t.time)}</time></span><span class="turn-text">${esc(t.text)}</span></button>`).join('');
  if (follow) list.scrollTop = list.scrollHeight;
}

function setQuestion(text: string, fromSTT = false) {
  questionRevision++;
  window.clearTimeout(softClearTimer);
  userSpeechStart = 0;
  $('#composer').classList.remove('stt-dimmed');
  $<HTMLTextAreaElement>('#question').value = text;
  fromTranscript = fromSTT;
  $('#question-source').hidden = !fromSTT;
  $('#composer').classList.toggle('stt-ready', fromSTT && !!text);
  $('#send').classList.toggle('ready', fromSTT && !!text);
  resizeQuestion();
  if (fromSTT) lastQuestion = text;
}

async function ask(mode: string, text = '') {
  if (status.busy) { toast('上一条回答正在生成，可以先停止再提问。'); return; }
  if (mode === 'ask' && !text.trim()) { $<HTMLTextAreaElement>('#question').focus(); return; }
  if ((mode === 'say' || mode === 'recap') && !(status.transcript || []).length) { toast('还没有对话记录，先开始会话并说几句话。'); return; }
  const clearsQuestion = mode === 'ask' || mode === 'answerThis';
  const previous = { text: $<HTMLTextAreaElement>('#question').value, fromTranscript, manualInput };
  const submittedPhrase = liveQuestion;
  const wasSuppressed = submittedPhrase?.suppressed;
  if (clearsQuestion) {
    // 发送的是当前可见文本；同一句后续修正仍显示字幕，但不能再次填回问题。
    if (submittedPhrase) submittedPhrase.suppressed = true;
    setQuestion(''); manualInput = false;
  }
  const revision = questionRevision;
  try { await call('Ask', { mode, text: text.trim() }); }
  catch (error) {
    if (clearsQuestion && questionRevision === revision) {
      setQuestion(previous.text, previous.fromTranscript); manualInput = previous.manualInput;
      if (liveQuestion === submittedPhrase && submittedPhrase) submittedPhrase.suppressed = !!wasSuppressed;
    }
    throw error;
  }
}

function updateLiveQuestion(text: string, final = false) {
  const value = text.trim();
  liveQuestion ||= { prefix: fromTranscript ? $<HTMLTextAreaElement>('#question').value.trim() : '', suppressed: manualInput };
  if (!manualInput && !liveQuestion.suppressed && value && !/^[?!.,;:\-…？！。，；：]+$/.test(value)) {
    setQuestion([liveQuestion.prefix, value].filter(Boolean).join(' '), true);
  }
  if (final) liveQuestion = undefined;
}

function clearLiveQuestion() {
  if (liveQuestion && !manualInput && !liveQuestion.suppressed) {
    setQuestion(liveQuestion.prefix, !!liveQuestion.prefix);
  }
  liveQuestion = undefined;
}

function dismissQuestion() {
  if (liveQuestion) liveQuestion.suppressed = true;
  setQuestion(''); manualInput = false;
}

function addAnswer(id: string, question: string) {
  setCollapsed(false);
  const element = document.createElement('article');
  element.className = 'answer';
  element.innerHTML = `<div class="response-sep">${clock(Date.now())}</div>${question ? `<div class="user-question">${esc(question)}</div>` : ''}<div class="answer-header"><span class="answer-state">正在思考…</span><button class="icon-button copy-answer" data-copy-answer="${esc(id)}" title="复制回答" aria-label="复制回答" hidden>${icon('copy')}</button></div><div class="markdown answer-content"><span class="thinking-dots"><i></i><i></i><i></i></span></div>`;
  $('#messages').append(element);
  answers.set(id, { text: '', element, done: false, dirty: false });
  if (answers.size > 20) { const oldest = answers.keys().next().value!; answers.get(oldest)!.element.remove(); answers.delete(oldest); }
  activeAnswer = id;
  status.busy = true;
  renderStatus();
  $('#messages').scrollTop = $('#messages').scrollHeight;
}

function renderAnswers() {
  renderPending = false;
  const list = $('#messages');
  const follow = list.scrollHeight - list.scrollTop - list.clientHeight < 100;
  for (const answer of answers.values()) {
    if (!answer.text || !answer.dirty) continue;
    answer.dirty = false;
    const content = $('.answer-content', answer.element);
    content.innerHTML = markdown(answer.text);
    $('.answer-state', answer.element).textContent = answer.stopped ? '已停止' : answer.done ? '已完成' : '正在回答';
    $('.copy-answer', answer.element).hidden = false;
    content.querySelectorAll('pre').forEach(pre => {
      const button = document.createElement('button');
      button.type = 'button'; button.className = 'copy-code'; button.textContent = '复制代码';
      button.addEventListener('click', () => void run(async () => { await copy(pre.querySelector('code')?.textContent || ''); toast('代码已复制'); }));
      pre.append(button);
    });
  }
  if (follow) list.scrollTop = list.scrollHeight;
}

function scheduleAnswers() {
  if (!renderPending) { renderPending = true; requestAnimationFrame(renderAnswers); }
}

function handleEvent(event: CueEvent) {
  if (!event || !event.type) return;
  switch (event.type) {
    case 'status':
      if (event.data && typeof event.data === 'object') status = { ...status, ...event.data as Partial<Status> };
      if (!status.listening) { interim = {}; liveQuestion = undefined; renderInterim(); }
      renderStatus(); renderTranscript(); break;
    case 'error': toast(event.text || '操作失败，请检查配置后重试。', true); break;
    case 'answer-start': addAnswer(event.id || `answer-${Date.now()}`, event.text || ''); break;
    case 'thinking': {
      const answer = answers.get(event.id || activeAnswer);
      if (answer) $('.answer-state', answer.element).textContent = '正在思考…';
      break;
    }
    case 'answer-token': {
      const id = event.id || activeAnswer;
      if (!answers.has(id)) addAnswer(id, '');
      answers.get(id)!.text += event.text || '';
      answers.get(id)!.dirty = true;
      scheduleAnswers(); break;
    }
    case 'answer-done': {
      const answer = answers.get(event.id || activeAnswer);
      if (answer) { if (event.text) answer.text = event.text; answer.done = true; answer.dirty = true; }
      status.busy = false; scheduleAnswers(); renderStatus(); break;
    }
    case 'answer-error': {
      const answer = answers.get(event.id || activeAnswer);
      if (answer) {
        answer.stopped = true; answer.done = true;
        const error = document.createElement('p'); error.className = 'answer-error'; error.textContent = event.text || '生成失败';
        answer.element.append(error); $('.answer-state', answer.element).textContent = '已停止';
        if (!answer.text) $('.answer-content', answer.element).replaceChildren();
      } else toast(event.text || '生成失败', true);
      status.busy = false; renderStatus(); break;
    }
    case 'transcript': {
      const turn = event.data as Turn;
      if (!turn || !turn.text) break;
      const existing = status.transcript.findIndex(t => t.id === turn.id);
      if (existing >= 0) status.transcript[existing] = turn; else status.transcript.push(turn);
      status.transcript = status.transcript.slice(-200);
      interim[turn.channel] = '';
      if (turn.channel === 'them' && existing < 0) {
        updateLiveQuestion(turn.text, true);
      } else if (turn.channel === 'you' && fromTranscript && !manualInput && !liveQuestion) {
        // 与原版一致：短暂应答不清空问题，持续说话后才软清除。
        if (!userSpeechStart) userSpeechStart = Date.now();
        $('#composer').classList.add('stt-dimmed');
        window.clearTimeout(softClearTimer);
        softClearTimer = window.setTimeout(() => {
          if (fromTranscript && !manualInput && !liveQuestion && Date.now() - userSpeechStart > 2000) dismissQuestion();
        }, 800);
      }
      renderTranscript(); renderInterim(); break;
    }
    case 'interim': {
      const channel = event.channel || 'you';
      interim[channel] = event.text || '';
      if (channel === 'them') {
        if (event.text?.trim()) updateLiveQuestion(event.text);
        else clearLiveQuestion();
      }
      renderInterim(); break;
    }
    case 'slides': status.slides = (event.data as Slide[]) || []; renderStatus(); if (document.querySelector('#slides-content')) renderSlides(); break;
    case 'models': models = (event.data as ModelInfo[]) || []; if (document.querySelector('#model-list')) renderModels(); break;
    case 'download': {
      const progress = event.data as { id: string; downloaded: number; bytes: number };
      const model = models.find(m => m.id === progress?.id);
      if (model) { model.downloaded = progress.downloaded; model.downloading = progress.downloaded < progress.bytes; }
      if (document.querySelector('#model-list')) renderModels(); break;
    }
    case 'shortcut':
      if (event.text === 'toggle' || event.text === 'hide') void run(toggleCollapsed);
      else if (event.text === 'settings') openSettings();
      else void run(() => ask(event.text || 'assist'));
      break;
    case 'consent':
      void run(async () => {
        await call('SetPassthrough', false);
        showConsent(event);
      });
      break;
  }
}

function renderInterim() {
  $('#interim').innerHTML = Object.entries(interim).filter(([, text]) => text).map(([channel, text]) => `<p><span>${channel === 'them' ? '对方' : '我'} · 正在说</span>${esc(text)}</p>`).join('');
}

function setCollapsed(value: boolean) {
  if (collapsed !== value) recordUIEvent('panel-toggle', value ? 'collapsed' : 'expanded');
  collapsed = value;
  $('#workspace').hidden = collapsed;
  $('#collapse-toggle').setAttribute('aria-expanded', String(!collapsed));
  $('#collapse-toggle').setAttribute('aria-label', collapsed ? '显示面板' : '隐藏面板');
  $('#collapse-toggle').title = collapsed ? '显示面板' : '隐藏面板';
  $('#collapse-toggle span').textContent = collapsed ? '显示' : '隐藏';
  $('#collapse-toggle').classList.toggle('collapsed', collapsed);
  $('.desktop').classList.toggle('collapsed', collapsed);
  if (!collapsed) resizeQuestion();
  scheduleLayout();
}
async function toggleCollapsed() { setCollapsed(!collapsed); }

function toggleSidebar() {
  sidebar = !sidebar;
  recordUIEvent('sidebar-toggle', sidebar ? 'open' : 'closed');
  $('#transcript-panel').hidden = !sidebar;
  $('#sidebar-toggle').setAttribute('aria-pressed', String(sidebar));
  $('.desktop').classList.toggle('sidebar-open', sidebar);
  scheduleLayout();
}

function resizeQuestion() {
  const input = $<HTMLTextAreaElement>('#question');
  input.style.height = '22px';
  input.style.height = `${Math.min(140, Math.max(22, input.scrollHeight))}px`;
  scheduleLayout();
}

function setActiveModal(kind?: ModalKind) {
  activeModal = kind;
  $('#settings-toggle').setAttribute('aria-expanded', String(kind === 'settings'));
  $('#help').setAttribute('aria-expanded', String(kind === 'guide'));
}

function modal(title: string, subtitle = '', wide = false, kind: ModalKind = 'other') {
  setOpacityOpen(false);
  if (pendingConsent) { const id = pendingConsent; pendingConsent = ''; void run(() => call('Consent', id, false)); }
  setActiveModal(kind);
  restoreFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const root = $('#modal-root');
  root.innerHTML = `<div class="modal-backdrop" data-hit-region><section class="modal glass ${wide ? 'modal-wide' : ''}" data-hit-region role="dialog" aria-modal="true" aria-labelledby="modal-title"><header class="modal-header"><div><h2 id="modal-title">${esc(title)}</h2>${subtitle ? `<p>${esc(subtitle)}</p>` : ''}</div><button class="icon-button" id="modal-close" title="关闭" aria-label="关闭">${icon('close')}</button></header><div id="modal-body" class="modal-body"></div><footer id="modal-footer" class="modal-footer" hidden></footer></section></div>`;
  $('#modal-close').addEventListener('click', closeModal);
  $('.modal-backdrop').addEventListener('click', event => { if (event.target === event.currentTarget) closeModal(); });
  // 宿主此时可能只有菜单条高度，聚焦不能滚动页面并扰乱尺寸上报。
  $<HTMLButtonElement>('#modal-close').focus({ preventScroll: true });
  return $('#modal-body');
}
function closeModal() {
  if (pendingConsent) { const id = pendingConsent; pendingConsent = ''; void run(() => call('Consent', id, false)); }
  $('#modal-root').replaceChildren();
  setActiveModal();
  if (restoreFocus?.isConnected) restoreFocus.focus({ preventScroll: true });
}

function endpointForm(endpoint: Endpoint, speech: boolean) {
  const available = speech ? ['sherpa', 'openai', 'custom', 'groq', 'gemini', 'deepgram', 'local'] : ['cerebras', 'openai', 'anthropic', 'gemini', 'groq', 'custom', 'ollama', 'minimax', 'deepseek', 'azure'];
  const options = available.map(p => `<button type="button" data-provider-value="${p}" aria-pressed="${endpoint.provider === p}">${p === 'custom' ? '第三方中转' : p === 'sherpa' ? '本地实时（中文）' : p === 'local' ? 'Whisper（分段）' : p === 'ollama' ? 'Ollama' : p === 'gemini' ? 'Gemini' : providers[p]?.name || 'Deepgram'}</button>`).join('');
  const local = ['local', 'sherpa'].includes(endpoint.provider);
  return `<div class="field"><span class="field-label" id="provider-label">${speech ? '语音转写服务' : '对话模型服务'}</span><input id="endpoint-provider" data-endpoint="provider" type="hidden" value="${esc(endpoint.provider)}"/><div class="provider-segments" role="group" aria-labelledby="provider-label">${options}</div></div>
    ${!local ? `<div class="field"><label for="endpoint-url">Base URL <span>服务地址</span></label><input id="endpoint-url" data-endpoint="baseUrl" type="url" value="${esc(endpoint.baseUrl)}" placeholder="https://your-gateway.example/v1" autocomplete="off" spellcheck="false"/><p class="field-hint">填入服务商提供的接口地址。自定义中转使用 OpenAI 兼容协议。</p></div>
    <div class="field"><label for="endpoint-key">API Key <span>${endpoint.hasKey ? '已保存，留空继续使用' : '保存在本机'}</span></label><div class="password-row"><input id="endpoint-key" data-endpoint="apiKey" type="password" value="${esc(endpoint.apiKey)}" placeholder="${endpoint.hasKey ? '••••••••（已保存）' : '输入服务商提供的 API Key'}" autocomplete="new-password" spellcheck="false"/><button id="key-visibility" type="button" class="icon-button" aria-label="显示密钥">${icon('eye')}</button></div><label class="check-label"><input id="clear-endpoint-key" type="checkbox" ${endpoint.clearKey ? 'checked' : ''}/>清除已保存密钥</label></div>
    <div class="field-grid"><div class="field"><label for="endpoint-model">${speech ? '转写模型' : '快速模型'}</label><input id="endpoint-model" data-endpoint="model" value="${esc(endpoint.model)}" placeholder="${speech ? 'whisper-1' : '服务商提供的模型名称'}" spellcheck="false"/></div>${!speech ? `<div class="field"><label for="endpoint-smart">深度思考模型</label><input id="endpoint-smart" data-endpoint="smartModel" value="${esc(endpoint.smartModel)}" placeholder="服务商提供的模型名称" spellcheck="false"/></div>` : ''}</div>
    ${!speech ? `<label class="switch-row"><span><strong>流式回答</strong><small>逐步显示回答；中转不支持时可关闭</small></span><input id="endpoint-stream" type="checkbox" role="switch" ${endpoint.stream ? 'checked' : ''}/></label>` : `<p class="field-hint">语音使用自己的接口与密钥，不会自动复用聊天密钥。</p>${['openai', 'deepgram', 'gemini'].includes(endpoint.provider) ? `<label class="switch-row"><span><strong>实时转写</strong><small>逐步更新文字；需要提供商支持实时语音连接</small></span><input id="endpoint-stream" type="checkbox" role="switch" ${endpoint.stream ? 'checked' : ''}/></label><p class="field-hint">${endpoint.provider === 'gemini' ? '实时模式需要 Gemini Live 模型（例如 gemini-3.5-transcribe-live），普通 Flash 模型用于分段转写。' : endpoint.provider === 'openai' ? '实时模式建议使用 gpt-4o-mini-transcribe；whisper-1 通常只提供最终文字。' : 'nova-3 支持实时和分段转写。'}</p>` : ''}`}` : `<div class="callout">${endpoint.provider === 'sherpa' ? '在本机实时识别中文，说话时持续更新字幕，停顿后确认整句。输出统一为简体中文，无需 API Key；运行时与模型随应用提供。' : '音频留在本机，由 whisper.cpp 分段转写。请在“本地模型”页准备模型和运行时；本地模式不会自动切换到云端。'}</div>`}
    ${speech ? `<label class="switch-row"><span><strong>采集系统音频</strong><small>捕获会议中对方的声音，与麦克风分开记录</small></span><input data-setting="meetingAudio" type="checkbox" role="switch" ${draft.meetingAudio ? 'checked' : ''}/></label><p class="field-hint">macOS 开启后需要屏幕录制权限，并显示系统录屏指示。设备或权限失败会显示错误。</p>` : '<p class="field-hint">模型名称由你的服务商决定。截图问答需要支持图片的模型。</p>'}`;
}

function openSettings(tab = 'chat') {
  if (pendingConsent) return;
  if (['chat', 'speech', 'style', 'appearance'].includes(tab)) recordUIEvent('settings-open', tab);
  draft = structuredClone(settings); settingsTab = tab;
  modal('设置', '', true, 'settings');
  $('#modal-body').innerHTML = `<div class="settings-layout"><nav class="settings-nav" aria-label="设置分类">${[
    ['chat', 'spark', '密钥'], ['speech', 'mic', '音频'], ['style', 'sliders', '风格'], ['appearance', 'eclipse', '外观'],
  ].map(([id, glyph, title]) => `<button data-tab="${id}" class="${tab === id ? 'active' : ''}">${icon(glyph)}${title}</button>`).join('')}</nav><div id="settings-page" class="settings-page"></div></div>`;
  $('#modal-footer').hidden = false;
  $('#modal-footer').innerHTML = '<span id="settings-notice">更改将在保存后生效</span><button class="primary-button" id="save-settings">保存设置</button>';
  document.querySelectorAll<HTMLButtonElement>('[data-tab]').forEach(button => button.addEventListener('click', () => { readSettingsPage(); settingsTab = button.dataset.tab!; recordUIEvent('settings-open', settingsTab); renderSettingsPage(); }));
  $('#save-settings').addEventListener('click', () => void run(saveSettings));
  renderSettingsPage();
}

function renderSettingsPage() {
  const activeTab = settingsTab === 'models' ? 'speech' : settingsTab === 'access' ? 'chat' : settingsTab;
  document.querySelectorAll('[data-tab]').forEach(button => button.classList.toggle('active', (button as HTMLElement).dataset.tab === activeTab));
  const page = $('#settings-page');
  if (settingsTab === 'chat' || settingsTab === 'speech') {
    page.innerHTML = `${endpointForm(draft[settingsTab], settingsTab === 'speech')}<button class="secondary-button" data-settings-page="${settingsTab === 'chat' ? 'access' : 'models'}">${settingsTab === 'chat' ? '本地助手授权' : 'Whisper 本地模型管理'}</button>`;
    page.querySelectorAll<HTMLButtonElement>('[data-provider-value]').forEach(button => button.addEventListener('click', () => {
      readSettingsPage();
      const endpoint = draft[settingsTab as 'chat' | 'speech'];
      const provider = button.dataset.providerValue!;
      recordUIEvent('provider-select', provider);
      if (endpoint.provider === provider) return;
      const preset = providers[provider];
      endpoint.provider = provider; endpoint.baseUrl = preset?.url || (provider === 'deepgram' ? 'https://api.deepgram.com/v1' : '');
      endpoint.model = settingsTab === 'speech' ? ({ openai: 'whisper-1', custom: 'whisper-1', groq: 'whisper-large-v3', gemini: 'gemini-2.5-flash', deepgram: 'nova-3', local: '' }[provider] || '') : preset?.model || '';
      endpoint.smartModel = preset?.smart || ''; endpoint.apiKey = ''; endpoint.hasKey = false; endpoint.clearKey = true;
      if (settingsTab === 'speech') endpoint.stream = provider === 'sherpa';
      renderSettingsPage();
      $('#settings-notice').textContent = ['local', 'sherpa'].includes(provider) ? '已切换本地服务，保存后生效' : '已切换服务，请填写对应密钥后保存';
    }));
    const endpointKind = settingsTab;
    document.querySelector('#endpoint-key')?.addEventListener('focus', () => recordUIEvent('key-focus', endpointKind));
    document.querySelector('#key-visibility')?.addEventListener('click', () => { const key = $<HTMLInputElement>('#endpoint-key'); key.type = key.type === 'password' ? 'text' : 'password'; });
    if (settingsTab === 'speech') document.querySelector('#endpoint-stream')?.addEventListener('change', () => {
      const enabled = $<HTMLInputElement>('#endpoint-stream').checked;
      const input = $<HTMLInputElement>('#endpoint-model');
      if (draft.speech.provider === 'gemini' && (input.value === 'gemini-2.5-flash' || input.value === 'gemini-3.5-transcribe-live')) input.value = enabled ? 'gemini-3.5-transcribe-live' : 'gemini-2.5-flash';
      if (draft.speech.provider === 'openai' && input.value === 'whisper-1' && enabled) input.value = 'gpt-4o-mini-transcribe';
    });
  } else if (settingsTab === 'models') {
    page.innerHTML = `<h3>Whisper 本地转写</h3><p class="section-description">先准备 whisper-server 运行时，再下载或导入模型。中文建议使用多语言模型，例如 base；带 .en 的模型只适合英语。</p><div class="field"><label for="whisper-path">whisper-server 路径</label><input id="whisper-path" data-setting="whisperPath" value="${esc(draft.whisperPath)}" placeholder="留空使用随应用提供的运行时" spellcheck="false"/></div><div class="field-grid"><div class="field"><label for="whisper-model">当前模型</label><input id="whisper-model" data-setting="whisperModel" value="${esc(draft.whisperModel)}" placeholder="base" spellcheck="false"/></div><div class="field"><label for="whisper-threads">计算线程 <span>0 为自动</span></label><input id="whisper-threads" data-setting="whisperThreads" type="number" min="0" max="64" value="${draft.whisperThreads}"/></div></div><div class="models-heading"><h4>模型库</h4><button id="refresh-models" class="text-button">刷新</button></div><div id="model-list" class="model-list">正在读取模型…</div>`;
    $('#refresh-models').addEventListener('click', () => void run(loadModels));
    void run(loadModels);
  } else if (settingsTab === 'style') {
    page.innerHTML = `<h3>用适合你的方式回答</h3><div class="field"><label for="answer-language">回答 / 转写语言</label><select id="answer-language" data-setting="language">${[['zh', '中文'], ['en', 'English'], ['auto', '自动'], ['ja', '日本語'], ['ko', '한국어'], ['es', 'Español']].map(([v, label]) => `<option value="${v}" ${draft.language === v ? 'selected' : ''}>${label}</option>`).join('')}</select></div><div class="field"><label for="ai-rules">回答风格 <span>最多 2,000 字符</span></label><textarea id="ai-rules" data-setting="aiRules" rows="5" maxlength="2000" placeholder="例如：简洁自然，先给结论；使用短句，必要时列出要点。">${esc(draft.aiRules)}</textarea><p class="field-hint">应用于日常回答；屏幕解题保持独立格式。</p></div><div class="field"><label for="background">个人背景 <span>可选</span></label><textarea id="background" data-setting="background" rows="5" maxlength="30000" placeholder="你的工作、项目经验或其他希望 Cue 了解的背景…">${esc(draft.background)}</textarea><p class="field-hint">随相关问题发送给所选模型。这里只写你愿意提供的信息。</p></div>`;
  } else if (settingsTab === 'appearance') {
    page.innerHTML = `<h3>保持专注，按需出现</h3><div class="field"><label for="setting-opacity">窗口透明度</label><input id="setting-opacity" data-setting="opacity" type="range" min="0.2" max="1" step="0.01" value="${draft.opacity}"/></div><label class="switch-row"><span><strong>屏幕共享排除</strong><small>请求系统在录屏或共享时排除 Cue 窗口</small></span><input data-setting="contentProtection" type="checkbox" role="switch" ${draft.contentProtection ? 'checked' : ''}/></label><p class="field-hint">效果取决于系统与录屏工具，macOS 无法保证所有捕获方式都排除窗口。更改后可能需要重启。</p><div class="switch-row"><span><strong>鼠标穿透</strong><small>让鼠标点击直接到达窗口下方的应用</small></span><button id="enable-passthrough" class="secondary-button">开启穿透</button></div><p class="field-hint">开启后，用 ${status.platform === 'darwin' ? 'Cmd' : 'Ctrl'} + Shift + I 恢复交互。快捷键注册失败时无法开启。</p><label class="switch-row"><span><strong>跟踪会议页面</strong><small>听写期间检查屏幕变化，生成页面文字记录</small></span><input data-setting="slidesEnabled" type="checkbox" role="switch" ${draft.slidesEnabled ? 'checked' : ''}/></label><div class="field"><label for="slides-interval">页面检查间隔 <span>毫秒</span></label><input id="slides-interval" data-setting="slidesInterval" type="number" min="1500" max="15000" step="500" value="${draft.slidesInterval}"/></div><p class="field-hint">页面描述请求会发送截图至所选模型，并可能产生费用。仅在开启后采集，记录保留在本次运行的内存中。</p><div class="data-location"><span>本地数据目录</span><code>${esc(status.dataDir)}</code></div>`;
    $('#enable-passthrough').addEventListener('click', () => void run(async () => { await call('SetPassthrough', true); closeModal(); toast(`已开启鼠标穿透，按 ${status.platform === 'darwin' ? 'Cmd' : 'Ctrl'} + Shift + I 恢复交互。`); }));
  } else {
    page.innerHTML = '<h3>本地助手授权</h3><p class="section-description">其他本机程序需要你的授权才能读取状态或控制会话。页面内容的读取需要单独授权。</p><div id="caller-list">正在读取授权记录…</div>';
    void run(async () => {
      const callers = await call<Caller[]>('ListCallers');
      if (!document.querySelector('#caller-list')) return;
      $('#caller-list').innerHTML = callers?.length ? callers.map(c => `<div class="caller-row"><div><strong>${esc(c.name || c.id)}</strong><small>${esc(c.id)} · ${esc(c.scope || (c.scopes || []).join('、'))}</small></div><button class="text-button danger" data-revoke="${esc(c.id)}">撤销</button></div>`).join('') : '<div class="empty-state">还没有授权过其他本机程序。</div>';
    });
  }
  if (settingsTab === 'appearance') page.insertAdjacentHTML('beforeend', `<div class="settings-tools"><button class="secondary-button" data-open-meetings>会议记录</button><button class="secondary-button" data-open-slides>页面记录 <span id="slide-count">${(status.slides || []).length}</span></button><button class="secondary-button" data-mode="leetcode">屏幕解题</button></div>`);
}

function readSettingsPage() {
  const page = document.querySelector('#settings-page');
  if (!page) return;
  if (settingsTab === 'chat' || settingsTab === 'speech') {
    const endpoint = draft[settingsTab];
    const old = settings[settingsTab];
    page.querySelectorAll<HTMLInputElement>('[data-endpoint]').forEach(input => { (endpoint as unknown as Record<string, unknown>)[input.dataset.endpoint!] = input.value.trim(); });
    const clear = page.querySelector<HTMLInputElement>('#clear-endpoint-key');
    if (clear) endpoint.clearKey = clear.checked;
    if (endpoint.baseUrl !== old.baseUrl || endpoint.provider !== old.provider) endpoint.clearKey = true;
    const stream = page.querySelector<HTMLInputElement>('#endpoint-stream');
    if (stream) endpoint.stream = stream.checked;
  }
  page.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>('[data-setting]').forEach(input => {
    const key = input.dataset.setting!;
    let value: string | boolean | number = input.value;
    if (input instanceof HTMLInputElement) {
      if (input.type === 'checkbox') value = input.checked;
      if (input.type === 'number' || input.type === 'range') value = Number(input.value);
    }
    (draft as unknown as Record<string, unknown>)[key] = value;
  });
}

async function saveSettings() {
  recordUIEvent('settings-save', settingsTab);
  readSettingsPage();
  for (const endpoint of [draft.chat, draft.speech]) {
    if (!['local', 'sherpa'].includes(endpoint.provider) && endpoint.baseUrl) {
      let url: URL;
      try { url = new URL(endpoint.baseUrl); } catch { throw new Error('Base URL 格式不正确，请填写完整地址。'); }
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) throw new Error('服务地址只支持 HTTP / HTTPS，且不能包含账号密码。');
    }
    if (endpoint.apiKey.trim()) endpoint.clearKey = false;
  }
  if (!draft.chat.model.trim()) throw new Error('请填写快速模型名称。');
  if (!draft.chat.smartModel.trim()) draft.chat.smartModel = draft.chat.model;
  const save = $<HTMLButtonElement>('#save-settings'); save.disabled = true; save.textContent = '正在保存…';
  try {
    settings = await call<Settings>('SaveSettings', draft);
    renderStatus(); closeModal(); toast('设置已保存');
  } finally { if (save.isConnected) { save.disabled = false; save.textContent = '保存设置'; } }
}

async function loadModels() {
  models = (await call<ModelInfo[]>('ListModels')) || [];
  if (document.querySelector('#model-list')) renderModels();
}
function renderModels() {
  $('#model-list').innerHTML = models.length ? models.map(m => {
    const percent = Math.min(100, Math.max(0, Math.round(m.downloaded / Math.max(1, m.bytes) * 100)));
    return `<div class="model-row"><div class="model-title"><div><strong>${esc(m.id)}</strong><small>${size(m.bytes)}${m.id.includes('.en') ? ' · 英语' : ' · 多语言'}${m.installed ? ' · 已安装' : ''}</small></div><div class="model-actions">${m.installed ? `<button data-model-select="${esc(m.id)}" class="text-button">选用</button><button data-model-delete="${esc(m.id)}" class="icon-button danger" title="删除模型">${icon('trash')}</button>` : m.downloading ? `<button data-model-cancel="${esc(m.id)}" class="text-button">取消</button>` : `<button data-model-import="${esc(m.id)}" class="icon-button" title="导入模型">${icon('upload')}</button><button data-model-download="${esc(m.id)}" class="icon-button" title="下载模型">${icon('download')}</button>`}</div></div>${m.downloading ? `<div class="model-progress"><progress value="${percent}" max="100"></progress><span>${percent}% · ${size(m.downloaded)}</span></div>` : ''}</div>`;
  }).join('') : '<div class="empty-state">暂无模型目录，请点击刷新。</div>';
}

async function showMeetings() {
  const body = modal('会议记忆', '本机保存的对话与会后纪要', true);
  body.innerHTML = '<div class="empty-state">正在读取会议…</div>';
  meetings = (await call<Meeting[]>('ListMeetings')) || [];
  if (!document.querySelector('#modal-title') || $('#modal-title').textContent !== '会议记忆') return;
  if (!meetings.length) { body.innerHTML = `<div class="empty-state large-empty">${icon('notes')}<h3>重要的内容，留得住</h3><p>开始一场会话后，转录会保存在这里。<br/>结束时会生成纪要，方便下次接着聊。</p></div>`; return; }
  body.innerHTML = `<div class="meetings-layout"><nav class="meeting-list">${meetings.slice().reverse().map(m => `<button data-meeting="${esc(m.id)}"><strong>${esc(m.title || '未命名会话')}</strong><small>${date(m.startedAt)} · ${(m.turns || []).length} 条</small></button>`).join('')}</nav><div id="meeting-detail" class="meeting-detail"></div></div>`;
  renderMeeting(meetings[meetings.length - 1].id);
}

function renderMeeting(id: string) {
  const meeting = meetings.find(m => m.id === id);
  if (!meeting) return;
  document.querySelectorAll('[data-meeting]').forEach(el => el.classList.toggle('active', (el as HTMLElement).dataset.meeting === id));
  $('#meeting-detail').innerHTML = `<div class="meeting-detail-head"><h3>${esc(meeting.title || '未命名会话')}</h3><div><button class="icon-button" data-export-meeting="${esc(id)}" title="导出会议">${icon('download')}</button><button class="icon-button danger" data-delete-meeting="${esc(id)}" title="删除会议">${icon('trash')}</button></div></div><p class="muted">${date(meeting.startedAt)}${meeting.endedAt ? ` — ${clock(meeting.endedAt)}` : ' · 当前会议，可继续记录'}</p>${!meeting.endedAt ? '<div class="callout">停止听写后，30 分钟内可以继续同一场会议。需要开始一份新记录时，可结束并清空当前转录。<br/><button class="text-button" data-end-meeting>结束并清空当前转录</button></div>' : ''}<div class="markdown meeting-notes">${meeting.notes ? markdown(meeting.notes) : '<p class="muted">暂无纪要。停止听写后，具备足够内容的对话会生成纪要。</p>'}</div><details class="meeting-transcript"><summary>查看完整转录 · ${(meeting.turns || []).length} 条</summary>${(meeting.turns || []).map(t => `<p><strong>${t.channel === 'them' ? '对方' : '我'} <time>${clock(t.time)}</time></strong>${esc(t.text)}</p>`).join('')}</details>`;
}

function showSlides() {
  const body = modal('页面记录', '会议中的页面变化与内容摘要');
  body.innerHTML = '<div id="slides-content"></div>';
  renderSlides();
}
function renderSlides() {
  const slides = status.slides || [];
  $('#slides-content').innerHTML = slides.length ? slides.map((slide, i) => `<article class="slide-card"><span class="eyebrow">页面 ${i + 1} · ${clock(slide.time)}</span><div class="markdown">${markdown(slide.text)}</div></article>`).join('') : `<div class="empty-state large-empty">${icon('screen')}<h3>让页面，也成为上下文</h3><p>在设置中开启“跟踪会议页面”，<br/>开始会话后自动记录变化的页面内容。</p><button class="secondary-button" id="configure-slides">前往设置</button></div>`;
  document.querySelector('#configure-slides')?.addEventListener('click', () => openSettings('appearance'));
}

function showGuide() {
  if (pendingConsent) return;
  const body = modal('欢迎使用 cue', '实时转录对话，快速得到回答', false, 'guide');
  body.innerHTML = `<div class="onboarding"><div class="guide-step"><strong>1. 配置模型与语音</strong><p>在设置中填写 API Key、第三方中转地址和模型名。聊天与语音可分别配置，也可选择 Whisper 本地转写。</p></div><div class="guide-step"><strong>2. 开始会话</strong><p>点击工具条的“开始会话”。开启系统音频后可转录对方的声音。点击“显示”展开回答面板。</p></div><div class="guide-step"><strong>3. 按 Enter 回答</strong><p>对方说的话会填入输入框，按 Enter 或发送按钮获得回答；手工输入时不会被转录覆盖。转录历史按需展开。</p></div><div class="guide-shortcuts"><span>我该怎么说？ <kbd>${status.platform === 'darwin' ? '⌘' : 'Ctrl'} ↵</kbd></span><span>智能辅助 <kbd>${status.platform === 'darwin' ? '⌘' : 'Ctrl'} ⇧ ↵</kbd></span><span>屏幕解题 <kbd>${status.platform === 'darwin' ? '⌘' : 'Ctrl'} H</kbd></span></div><p class="field-hint">截图和云端语音会发送到所选服务。录屏排除效果取决于系统与捕获工具。</p></div>`;
  $('#modal-footer').hidden = false;
  $('#modal-footer').innerHTML = '<button id="guide-later" class="text-button">稍后配置</button><button id="guide-start" class="primary-button">配置模型</button>';
  const acknowledge = async (configure: boolean) => {
    if (connected) settings = await call<Settings>('SaveSettings', { ...settings, onboarded: true });
    if (configure) openSettings(); else closeModal();
  };
  $('#guide-later').addEventListener('click', () => void run(() => acknowledge(false)));
  $('#guide-start').addEventListener('click', () => void run(() => acknowledge(true)));
}

function showConsent(event: CueEvent) {
  const info = event.data as { caller?: string; scope?: string } | undefined;
  const body = modal('本机程序请求访问', '只有你允许后，对方才能使用对应能力');
  pendingConsent = event.id || '';
  body.innerHTML = `<div class="consent-content">${icon('shield')}<h3>${esc(info?.caller || '一个本机程序')}</h3><p>${esc(event.text || `请求权限：${info?.scope || '读取状态'}`)}</p><p class="field-hint">此名称由请求方提供，不代表已验证程序身份。授权可在设置中撤销。</p></div>`;
  const respond = async (allowed: boolean) => {
    const current = $('.modal');
    pendingConsent = '';
    try { await call('Consent', event.id || '', allowed); }
    catch (error) { if (current.isConnected) pendingConsent = event.id || ''; throw error; }
    if (current.isConnected) closeModal();
  };
  $('#modal-footer').hidden = false;
  $('#modal-footer').innerHTML = '<button id="consent-deny" class="secondary-button">拒绝</button><button id="consent-allow" class="primary-button">允许</button>';
  $('#consent-deny').addEventListener('click', () => void run(() => respond(false)));
  $('#consent-allow').addEventListener('click', () => void run(() => respond(true)));
}

$('#composer').addEventListener('submit', event => { event.preventDefault(); void run(() => ask(fromTranscript && !manualInput ? 'answerThis' : 'ask', $<HTMLTextAreaElement>('#question').value)); });
$('#question').addEventListener('input', () => { questionRevision++; if (liveQuestion) liveQuestion.suppressed = true; manualInput = !!$<HTMLTextAreaElement>('#question').value; fromTranscript = false; $('#question-source').hidden = true; $('#composer').classList.remove('stt-ready'); $('#send').classList.remove('ready'); resizeQuestion(); });
$('#question').addEventListener('keydown', event => {
  const e = event as KeyboardEvent;
  if (e.isComposing) return;
  if (e.key === 'Enter' && !e.shiftKey && !e.metaKey && !e.ctrlKey) { e.preventDefault(); $<HTMLFormElement>('#composer').requestSubmit(); }
  if ((e.metaKey || e.ctrlKey) && e.key === 'z' && !$<HTMLTextAreaElement>('#question').value && lastQuestion) { e.preventDefault(); manualInput = false; setQuestion(lastQuestion, true); }
});
$('#question-reset').addEventListener('click', dismissQuestion);
$('#cancel-answer').addEventListener('click', () => void run(() => call('Cancel')));
$('#settings-toggle').addEventListener('click', () => { if (activeModal === 'settings') closeModal(); else openSettings(); });
$('#more-settings').addEventListener('click', () => openSettings());
$('#help').addEventListener('click', () => { if (activeModal === 'guide') closeModal(); else showGuide(); });
$('#collapse-toggle').addEventListener('click', () => void run(toggleCollapsed));
$('#quit').addEventListener('click', () => void run(() => call('Quit')));
$('#sidebar-toggle').addEventListener('click', toggleSidebar);
$('#close-sidebar').addEventListener('click', toggleSidebar);
$('#session-toggle').addEventListener('click', () => void run(async () => {
  const button = $<HTMLButtonElement>('#session-toggle'); button.disabled = true;
  try { await call(status.listening ? 'StopListening' : 'StartListening'); status = await call<Status>('GetStatus'); renderTranscript(); } finally { renderStatus(); }
}));
$('#clear-transcript').addEventListener('click', () => void run(async () => {
  if (status.transcript.length && !await confirmAction('结束当前会议记录并清空实时转录？已保存的会议仍可在“会议记忆”中查看。')) return;
  await call('ClearTranscript'); status.transcript = []; interim = {}; liveQuestion = undefined; setQuestion(''); manualInput = false; renderTranscript(); renderInterim();
}));
$('#smart-toggle').addEventListener('click', () => void run(async () => { settings = await call<Settings>('SaveSettings', { ...settings, smart: !settings.smart }); renderStatus(); }));
function setOpacityOpen(open: boolean) {
  if (open && pendingConsent) return;
  if (open && activeModal) closeModal();
  $('#opacity-popover').hidden = !open;
  $('#opacity-toggle').setAttribute('aria-expanded', String(open));
}
$('#opacity-toggle').addEventListener('click', () => setOpacityOpen($('#opacity-popover').hidden));
$('#quick-opacity').addEventListener('input', () => { const value = Number($<HTMLInputElement>('#quick-opacity').value) / 100; document.documentElement.style.setProperty('--cue-opacity', String(value)); $('#opacity-value').textContent = `${Math.round(value * 100)}%`; });
$('#quick-opacity').addEventListener('change', () => void run(async () => { settings = await call<Settings>('SaveSettings', { ...settings, opacity: Number($<HTMLInputElement>('#quick-opacity').value) / 100 }); renderStatus(); }));

document.addEventListener('keydown', event => {
  if (event.isComposing) return;
  if ((event.metaKey || event.ctrlKey) && event.key === ',') { event.preventDefault(); openSettings(); }
  if ((event.metaKey || event.ctrlKey) && event.shiftKey && event.key.toLowerCase() === 'a') { event.preventDefault(); $<HTMLFormElement>('#composer').requestSubmit(); }
  if (event.key === 'Escape') {
    const confirmCancel = document.querySelector<HTMLButtonElement>('[data-confirm-cancel]');
    if (confirmCancel) { confirmCancel.click(); return; }
    if ($('#modal-root').childElementCount) closeModal();
    else { setOpacityOpen(false); dismissQuestion(); }
  }
  if (event.key === 'Tab' && ($('#modal-root').childElementCount || document.querySelector('.confirm-backdrop'))) {
    const container = document.querySelector('.confirm-backdrop') || $('#modal-root');
    const focusable = Array.from(container.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), textarea, select, a[href]')).filter(e => e.offsetParent !== null);
    if (!focusable.length) return;
    if (event.shiftKey && document.activeElement === focusable[0]) { event.preventDefault(); focusable.at(-1)!.focus(); }
    else if (!event.shiftKey && document.activeElement === focusable.at(-1)) { event.preventDefault(); focusable[0].focus(); }
  }
});

document.addEventListener('click', event => {
  const target = (event.target as Element).closest<HTMLElement>('button, a');
  if (!target) return;
  if (target.tagName === 'A' && target.closest('.markdown')) { event.preventDefault(); void run(() => openLink((target as HTMLAnchorElement).href)); return; }
  const data = target.dataset;
  if (data.settingsPage) { readSettingsPage(); settingsTab = data.settingsPage; renderSettingsPage(); }
  if ('openMeetings' in data) void run(showMeetings);
  if ('openSlides' in data) showSlides();
  if (data.mode) void run(() => ask(data.mode!));
  if (data.copyAnswer) void run(async () => { await copy(answers.get(data.copyAnswer!)?.text || ''); toast('回答已复制'); });
  if (data.turn) { const turn = status.transcript.find(t => t.id === data.turn); if (turn) { if (liveQuestion) liveQuestion.suppressed = true; manualInput = false; setQuestion(turn.text, true); $<HTMLTextAreaElement>('#question').focus(); } }
  if (data.meeting) renderMeeting(data.meeting);
  if (data.exportMeeting) void run(async () => { const path = await call<string>('ExportMeeting', data.exportMeeting); if (path) toast(`已导出至 ${path}`); });
  if (data.deleteMeeting) void run(async () => { if (!await confirmAction('永久删除这场会议及其转录、纪要？')) return; await call('DeleteMeeting', data.deleteMeeting); await showMeetings(); });
  if ('endMeeting' in data) void run(async () => { if (!await confirmAction('结束当前会议并清空实时转录？已保存的历史会议不会删除。')) return; await call('ClearTranscript'); status.transcript = []; interim = {}; liveQuestion = undefined; setQuestion(''); manualInput = false; renderTranscript(); renderInterim(); await showMeetings(); });
  if (data.modelSelect) { $<HTMLInputElement>('#whisper-model').value = data.modelSelect; draft.whisperModel = data.modelSelect; toast('已选择模型，保存设置后生效'); }
  if (data.modelDownload) void run(async () => { await call('DownloadModel', data.modelDownload); await loadModels(); });
  if (data.modelCancel) void run(async () => { await call('CancelDownload', data.modelCancel); await loadModels(); });
  if (data.modelImport) void run(async () => { await call('ImportModel', data.modelImport); await loadModels(); });
  if (data.modelDelete) void run(async () => { if (!await confirmAction(`删除模型 ${data.modelDelete}？之后可以重新下载。`)) return; await call('DeleteModel', data.modelDelete); await loadModels(); });
  if (data.revoke) void run(async () => { await call('RevokeCaller', data.revoke); renderSettingsPage(); });
});

async function start() {
  try {
    await connect(handleEvent);
    [settings, status] = await Promise.all([call<Settings>('GetSettings'), call<Status>('GetStatus')]);
    status.transcript ||= []; status.slides ||= []; status.warnings ||= [];
    connected = true; renderStatus(); renderTranscript();
    scheduleLayout();
  } catch (error) {
    renderStatus();
    $('#warning-banner').hidden = false;
    $('#warning-banner').textContent = `无法连接 Cue 桌面服务：${error instanceof Error ? error.message : String(error)}。请从桌面应用启动；浏览器预览请使用 ?preview=1。`;
    setCollapsed(false);
  }
}
void start();
