import { call, copy } from '../api/wails';
import { resizeQuestion } from '../app/layout';
import { state } from '../app/state';
import { icon } from '../components/icons';
import { run, toast } from '../components/toast';
import { $ } from '../utils/dom';
import { clock, esc } from '../utils/format';
import { markdown } from '../utils/markdown';
import { renderStatus, setCollapsed } from './toolbar';

export function setQuestion(text: string, fromSTT = false) {
  state.questionRevision++;
  window.clearTimeout(state.softClearTimer);
  state.userSpeechStart = 0;
  $('#composer').classList.remove('stt-dimmed');
  $<HTMLTextAreaElement>('#question').value = text;
  state.fromTranscript = fromSTT;
  $('#question-source').hidden = !fromSTT;
  $('#composer').classList.toggle('stt-ready', fromSTT && !!text);
  $('#send').classList.toggle('ready', fromSTT && !!text);
  resizeQuestion();
  if (fromSTT)
    state.lastQuestion = text;
}

export async function ask(mode: string, text = '') {
  if (state.status.busy) {
    toast('上一条回答正在生成，可以先停止再提问。');
    return;
  }
  if (mode === 'ask' && !text.trim()) {
    $<HTMLTextAreaElement>('#question').focus();
    return;
  }
  if ((mode === 'say' || mode === 'recap') && !(state.status.transcript || []).length) {
    toast('还没有对话记录，先开始会话并说几句话。');
    return;
  }
  const clearsQuestion = mode === 'ask' || mode === 'answerThis';
  const previous = { text: $<HTMLTextAreaElement>('#question').value, fromTranscript: state.fromTranscript, manualInput: state.manualInput };
  const submittedPhrase = state.liveQuestion;
  const wasSuppressed = submittedPhrase?.suppressed;
  if (clearsQuestion) {
    // 发送的是当前可见文本；同一句后续修正仍显示字幕，但不能再次填回问题。
    if (submittedPhrase)
      submittedPhrase.suppressed = true;
    setQuestion('');
    state.manualInput = false;
  }
  const revision = state.questionRevision;
  try {
    await call('Ask', { mode, text: text.trim() });
  }
  catch (error) {
    if (clearsQuestion && state.questionRevision === revision) {
      setQuestion(previous.text, previous.fromTranscript);
      state.manualInput = previous.manualInput;
      if (state.liveQuestion === submittedPhrase && submittedPhrase)
        submittedPhrase.suppressed = !!wasSuppressed;
    }
    throw error;
  }
}

export function updateLiveQuestion(text: string, final = false) {
  const value = text.trim();
  state.liveQuestion ||= { prefix: state.fromTranscript ? $<HTMLTextAreaElement>('#question').value.trim() : '', suppressed: state.manualInput };
  if (!state.manualInput && !state.liveQuestion.suppressed && value && !/^[?!.,;:\-…？！。，；：]+$/.test(value)) {
    setQuestion([state.liveQuestion.prefix, value].filter(Boolean).join(' '), true);
  }
  if (final)
    state.liveQuestion = undefined;
}

export function clearLiveQuestion() {
  if (state.liveQuestion && !state.manualInput && !state.liveQuestion.suppressed) {
    setQuestion(state.liveQuestion.prefix, !!state.liveQuestion.prefix);
  }
  state.liveQuestion = undefined;
}

export function dismissQuestion() {
  if (state.liveQuestion)
    state.liveQuestion.suppressed = true;
  setQuestion('');
  state.manualInput = false;
}

export function addAnswer(id: string, question: string) {
  setCollapsed(false);
  const element = document.createElement('article');
  element.className = 'answer';
  element.innerHTML = `<div class="response-sep">${clock(Date.now())}</div>${question ? `<div class="user-question">${esc(question)}</div>` : ''}<div class="answer-header"><span class="answer-state">正在思考…</span><button class="icon-button copy-answer" data-copy-answer="${esc(id)}" title="复制回答" aria-label="复制回答" hidden>${icon('copy')}</button></div><div class="markdown answer-content"><span class="thinking-dots"><i></i><i></i><i></i></span></div>`;
  $('#messages').append(element);
  state.answers.set(id, { text: '', element, done: false, dirty: false });
  if (state.answers.size > 20) {
    const oldest = state.answers.keys().next().value!;
    state.answers.get(oldest)!.element.remove();
    state.answers.delete(oldest);
  }
  state.activeAnswer = id;
  state.status.busy = true;
  renderStatus();
  $('#messages').scrollTop = $('#messages').scrollHeight;
}

export function renderAnswers() {
  state.renderPending = false;
  const list = $('#messages');
  const follow = list.scrollHeight - list.scrollTop - list.clientHeight < 100;
  for (const answer of state.answers.values()) {
    if (!answer.text || !answer.dirty)
      continue;
    answer.dirty = false;
    const content = $('.answer-content', answer.element);
    content.innerHTML = markdown(answer.text);
    $('.answer-state', answer.element).textContent = answer.stopped ? '已停止' : answer.done ? '已完成' : '正在回答';
    $('.copy-answer', answer.element).hidden = false;
    content.querySelectorAll('pre').forEach(pre => {
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'copy-code';
      button.textContent = '复制代码';
      button.addEventListener('click', () => void run(async () => { await copy(pre.querySelector('code')?.textContent || ''); toast('代码已复制'); }));
      pre.append(button);
    });
  }
  if (follow)
    list.scrollTop = list.scrollHeight;
}

export function scheduleAnswers() {
  if (!state.renderPending) {
    state.renderPending = true;
    requestAnimationFrame(renderAnswers);
  }
}
