import { call } from '../api/wails';
import { run, toast } from '../components/toast';
import { addAnswer, ask, clearLiveQuestion, dismissQuestion, scheduleAnswers, updateLiveQuestion } from '../features/chat';
import { showConsent } from '../features/consent';
import { applyLicense } from '../features/license';
import { renderModels } from '../features/models';
import { openSettings } from '../features/settings/index';
import { renderSlides } from '../features/slides';
import { renderStatus, toggleCollapsed } from '../features/toolbar';
import { renderInterim, renderTranscript } from '../features/transcript';
import { type CueEvent, type LicenseStatus, type ModelInfo, type Slide, type Status, type Turn } from '../types';
import { $ } from '../utils/dom';
import { state } from './state';

export function handleEvent(event: CueEvent) {
  if (!event || !event.type)
    return;
  switch (event.type) {
    case 'license-status':
      state.licenseRevision++;
      applyLicense(event.data as LicenseStatus);
      break;
    case 'status':
      if (event.data && typeof event.data === 'object')
        state.status = { ...state.status, ...event.data as Partial<Status> };
      if (!state.status.listening) {
        state.interim = {};
        state.liveQuestion = undefined;
        renderInterim();
      }
      renderStatus();
      renderTranscript();
      break;
    case 'error':
      toast(event.text || '操作失败，请检查配置后重试。', true);
      break;
    case 'answer-start':
      addAnswer(event.id || `answer-${Date.now()}`, event.text || '');
      break;
    case 'thinking': {
      const answer = state.answers.get(event.id || state.activeAnswer);
      if (answer)
        $('.answer-state', answer.element).textContent = '正在思考…';
      break;
    }
    case 'answer-token': {
      const id = event.id || state.activeAnswer;
      if (!state.answers.has(id))
        addAnswer(id, '');
      state.answers.get(id)!.text += event.text || '';
      state.answers.get(id)!.dirty = true;
      scheduleAnswers();
      break;
    }
    case 'answer-done': {
      const answer = state.answers.get(event.id || state.activeAnswer);
      if (answer) {
        if (event.text)
          answer.text = event.text;
        answer.done = true;
        answer.dirty = true;
      }
      state.status.busy = false;
      scheduleAnswers();
      renderStatus();
      break;
    }
    case 'answer-error': {
      const answer = state.answers.get(event.id || state.activeAnswer);
      if (answer) {
        answer.stopped = true;
        answer.done = true;
        const error = document.createElement('p');
        error.className = 'answer-error';
        error.textContent = event.text || '生成失败';
        answer.element.append(error);
        $('.answer-state', answer.element).textContent = '已停止';
        if (!answer.text)
          $('.answer-content', answer.element).replaceChildren();
      }
      else
        toast(event.text || '生成失败', true);
      state.status.busy = false;
      renderStatus();
      break;
    }
    case 'transcript': {
      const turn = event.data as Turn;
      if (!turn || !turn.text)
        break;
      const existing = state.status.transcript.findIndex(t => t.id === turn.id);
      if (existing >= 0)
        state.status.transcript[existing] = turn;
      else
        state.status.transcript.push(turn);
      state.status.transcript = state.status.transcript.slice(-200);
      state.interim[turn.channel] = '';
      if (turn.channel === 'them' && existing < 0) {
        updateLiveQuestion(turn.text, true);
      }
      else if (turn.channel === 'you' && state.fromTranscript && !state.manualInput && !state.liveQuestion) {
        // 与原版一致：短暂应答不清空问题，持续说话后才软清除。
        if (!state.userSpeechStart)
          state.userSpeechStart = Date.now();
        $('#composer').classList.add('stt-dimmed');
        window.clearTimeout(state.softClearTimer);
        state.softClearTimer = window.setTimeout(() => {
          if (state.fromTranscript && !state.manualInput && !state.liveQuestion && Date.now() - state.userSpeechStart > 2000)
            dismissQuestion();
        }, 800);
      }
      renderTranscript();
      renderInterim();
      break;
    }
    case 'interim': {
      const channel = event.channel || 'you';
      state.interim[channel] = event.text || '';
      if (channel === 'them') {
        if (event.text?.trim())
          updateLiveQuestion(event.text);
        else
          clearLiveQuestion();
      }
      renderInterim();
      break;
    }
    case 'slides':
      state.status.slides = (event.data as Slide[]) || [];
      renderStatus();
      if (document.querySelector('#slides-content'))
        renderSlides();
      break;
    case 'models':
      state.models = (event.data as ModelInfo[]) || [];
      if (document.querySelector('#model-list'))
        renderModels();
      break;
    case 'download': {
      const progress = event.data as {
        id: string;
        downloaded: number;
        bytes: number;
      };
      const model = state.models.find(m => m.id === progress?.id);
      if (model) {
        model.downloaded = progress.downloaded;
        model.downloading = progress.downloaded < progress.bytes;
      }
      if (document.querySelector('#model-list'))
        renderModels();
      break;
    }
    case 'shortcut':
      if (!state.licenseStatus.allowed)
        break;
      if (event.text === 'toggle' || event.text === 'hide')
        void run(toggleCollapsed);
      else if (event.text === 'settings')
        openSettings();
      else
        void run(() => ask(event.text || 'assist'));
      break;
    case 'consent':
      if (!state.licenseStatus.allowed) {
        void call('Consent', event.id, false);
        break;
      }
      void run(async () => {
        await call('SetPassthrough', false);
        showConsent(event);
      });
      break;
  }
}
