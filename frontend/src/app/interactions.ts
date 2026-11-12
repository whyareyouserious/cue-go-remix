import { call, copy, openLink } from '../api/wails';
import { confirmAction } from '../components/confirm';
import { closeModal } from '../components/modal';
import { run, toast } from '../components/toast';
import { ask, dismissQuestion, setQuestion } from '../features/chat';
import { showGuide } from '../features/guide';
import { licenseAction } from '../features/license';
import { renderMeeting, showMeetings } from '../features/meetings';
import { loadModels } from '../features/models';
import { openSettings, readSettingsPage, renderSettingsPage } from '../features/settings/index';
import { showSlides } from '../features/slides';
import { renderStatus, setOpacityOpen, toggleCollapsed, toggleSidebar } from '../features/toolbar';
import { renderInterim, renderTranscript } from '../features/transcript';
import { type Settings, type Status } from '../types';
import { $ } from '../utils/dom';
import { resizeQuestion } from './layout';
import { state } from './state';

export function bindInteractions() {
  $('#composer').addEventListener('submit', event => { event.preventDefault(); void run(() => ask(state.fromTranscript && !state.manualInput ? 'answerThis' : 'ask', $<HTMLTextAreaElement>('#question').value)); });
  $('#question').addEventListener('input', () => {
    state.questionRevision++; if (state.liveQuestion)
      state.liveQuestion.suppressed = true; state.manualInput = !!$<HTMLTextAreaElement>('#question').value; state.fromTranscript = false; $('#question-source').hidden = true; $('#composer').classList.remove('stt-ready'); $('#send').classList.remove('ready'); resizeQuestion();
  });
  $('#question').addEventListener('keydown', event => {
    const e = event as KeyboardEvent;
    if (e.isComposing)
      return;
    if (e.key === 'Enter' && !e.shiftKey && !e.metaKey && !e.ctrlKey) {
      e.preventDefault();
      $<HTMLFormElement>('#composer').requestSubmit();
    }
    if ((e.metaKey || e.ctrlKey) && e.key === 'z' && !$<HTMLTextAreaElement>('#question').value && state.lastQuestion) {
      e.preventDefault();
      state.manualInput = false;
      setQuestion(state.lastQuestion, true);
    }
  });
  $('#question-reset').addEventListener('click', dismissQuestion);
  $('#cancel-answer').addEventListener('click', () => void run(() => call('Cancel')));
  $('#settings-toggle').addEventListener('click', () => {
    if (state.activeModal === 'settings')
      closeModal();
    else
      openSettings();
  });
  $('#more-settings').addEventListener('click', () => openSettings());
  $('#help').addEventListener('click', () => {
    if (state.activeModal === 'guide')
      closeModal();
    else
      showGuide();
  });
  $('#collapse-toggle').addEventListener('click', () => void run(toggleCollapsed));
  $('#quit').addEventListener('click', () => void run(() => call('Quit')));
  $('#sidebar-toggle').addEventListener('click', toggleSidebar);
  $('#close-sidebar').addEventListener('click', toggleSidebar);
  $('#session-toggle').addEventListener('click', () => void run(async () => {
    const button = $<HTMLButtonElement>('#session-toggle');
    button.disabled = true;
    try {
      await call(state.status.listening ? 'StopListening' : 'StartListening');
      state.status = await call<Status>('GetStatus');
      renderTranscript();
    }
    finally {
      renderStatus();
    }
  }));
  $('#clear-transcript').addEventListener('click', () => void run(async () => {
    if (state.status.transcript.length && !await confirmAction('结束当前会议记录并清空实时转录？已保存的会议仍可在“会议记忆”中查看。'))
      return;
    await call('ClearTranscript');
    state.status.transcript = [];
    state.interim = {};
    state.liveQuestion = undefined;
    setQuestion('');
    state.manualInput = false;
    renderTranscript();
    renderInterim();
  }));
  $('#smart-toggle').addEventListener('click', () => void run(async () => { state.settings = await call<Settings>('SaveSettings', { ...state.settings, smart: !state.settings.smart }); renderStatus(); }));
  $('#opacity-toggle').addEventListener('click', () => setOpacityOpen($('#opacity-popover').hidden));
  $('#quick-opacity').addEventListener('input', () => { const value = Number($<HTMLInputElement>('#quick-opacity').value) / 100; document.documentElement.style.setProperty('--cue-opacity', String(value)); $('#opacity-value').textContent = `${Math.round(value * 100)}%`; });
  $('#quick-opacity').addEventListener('change', () => void run(async () => { state.settings = await call<Settings>('SaveSettings', { ...state.settings, opacity: Number($<HTMLInputElement>('#quick-opacity').value) / 100 }); renderStatus(); }));
  document.addEventListener('keydown', event => {
    if (event.isComposing || !state.licenseStatus.allowed)
      return;
    if ((event.metaKey || event.ctrlKey) && event.key === ',') {
      event.preventDefault();
      openSettings();
    }
    if ((event.metaKey || event.ctrlKey) && event.shiftKey && event.key.toLowerCase() === 'a') {
      event.preventDefault();
      $<HTMLFormElement>('#composer').requestSubmit();
    }
    if (event.key === 'Escape') {
      const confirmCancel = document.querySelector<HTMLButtonElement>('[data-confirm-cancel]');
      if (confirmCancel) {
        confirmCancel.click();
        return;
      }
      if ($('#modal-root').childElementCount)
        closeModal();
      else {
        setOpacityOpen(false);
        dismissQuestion();
      }
    }
    if (event.key === 'Tab' && ($('#modal-root').childElementCount || document.querySelector('.confirm-backdrop'))) {
      const container = document.querySelector('.confirm-backdrop') || $('#modal-root');
      const focusable = Array.from(container.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), textarea, select, a[href]')).filter(e => e.offsetParent !== null);
      if (!focusable.length)
        return;
      if (event.shiftKey && document.activeElement === focusable[0]) {
        event.preventDefault();
        focusable.at(-1)!.focus();
      }
      else if (!event.shiftKey && document.activeElement === focusable.at(-1)) {
        event.preventDefault();
        focusable[0].focus();
      }
    }
  });
  document.addEventListener('click', event => {
    const target = (event.target as Element).closest<HTMLElement>('button, a');
    if (!target)
      return;
    if (target.tagName === 'A' && target.closest('.markdown')) {
      event.preventDefault();
      void run(() => openLink((target as HTMLAnchorElement).href));
      return;
    }
    const data = target.dataset;
    if (data.settingsPage) {
      readSettingsPage();
      state.settingsTab = data.settingsPage;
      renderSettingsPage();
    }
    if ('openMeetings' in data)
      void run(showMeetings);
    if ('openSlides' in data)
      showSlides();
    if (data.mode)
      void run(() => ask(data.mode!));
    if (data.copyAnswer)
      void run(async () => { await copy(state.answers.get(data.copyAnswer!)?.text || ''); toast('回答已复制'); });
    if (data.turn) {
      const turn = state.status.transcript.find(t => t.id === data.turn);
      if (turn) {
        if (state.liveQuestion)
          state.liveQuestion.suppressed = true;
        state.manualInput = false;
        setQuestion(turn.text, true);
        $<HTMLTextAreaElement>('#question').focus();
      }
    }
    if (data.meeting)
      renderMeeting(data.meeting);
    if (data.exportMeeting)
      void run(async () => {
        const path = await call<string>('ExportMeeting', data.exportMeeting); if (path)
          toast(`已导出至 ${path}`);
      });
    if (data.deleteMeeting)
      void run(async () => {
        if (!await confirmAction('永久删除这场会议及其转录、纪要？'))
          return; await call('DeleteMeeting', data.deleteMeeting); await showMeetings();
      });
    if ('endMeeting' in data)
      void run(async () => {
        if (!await confirmAction('结束当前会议并清空实时转录？已保存的历史会议不会删除。'))
          return; await call('ClearTranscript'); state.status.transcript = []; state.interim = {}; state.liveQuestion = undefined; setQuestion(''); state.manualInput = false; renderTranscript(); renderInterim(); await showMeetings();
      });
    if (data.modelSelect) {
      $<HTMLInputElement>('#whisper-model').value = data.modelSelect;
      state.draft.whisperModel = data.modelSelect;
      toast('已选择模型，保存设置后生效');
    }
    if (data.modelDownload)
      void run(async () => { await call('DownloadModel', data.modelDownload); await loadModels(); });
    if (data.modelCancel)
      void run(async () => { await call('CancelDownload', data.modelCancel); await loadModels(); });
    if (data.modelImport)
      void run(async () => { await call('ImportModel', data.modelImport); await loadModels(); });
    if (data.modelDelete)
      void run(async () => {
        if (!await confirmAction(`删除模型 ${data.modelDelete}？之后可以重新下载。`))
          return; await call('DeleteModel', data.modelDelete); await loadModels();
      });
    if (data.revoke)
      void run(async () => { await call('RevokeCaller', data.revoke); renderSettingsPage(); });
  });
  $('#license-form').addEventListener('submit', event => { event.preventDefault(); void licenseAction('ActivateLicense', $<HTMLInputElement>('#license-key').value); });
  $('#license-trial').addEventListener('click', () => void licenseAction('StartTrial'));
  $('#license-retry').addEventListener('click', () => void licenseAction('RetryLicense'));
  $('#license-quit').addEventListener('click', () => void run(() => call('Quit')));
}
