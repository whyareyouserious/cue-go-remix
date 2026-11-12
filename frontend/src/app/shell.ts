import { preview } from '../api/wails';
import { icon } from '../components/icons';
import { $ } from '../utils/dom';

export function mountShell() {
  $('#app').innerHTML = `
  <div class="desktop collapsed ${preview ? 'preview' : ''}" data-hit-region hidden>
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
        <div id="warning-banner" class="notice warning" hidden></div><div id="license-grace" class="notice warning" hidden role="status"></div>
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
  <section id="license-panel" class="license-panel glass" data-hit-region aria-labelledby="license-title" aria-busy="true">
    <header class="modal-header"><div><h2 id="license-title">激活 Cue</h2><p>实时转录对话，快速得到回答</p></div><button id="license-quit" class="icon-button" aria-label="退出 Cue">${icon('close')}</button></header>
    <p id="license-message" role="status">正在检查本机授权</p>
    <form id="license-form"><div class="field"><label for="license-key">Cue 卡密</label><input id="license-key" type="password" placeholder="输入 Cue 专用卡密" maxlength="128" autocomplete="off" spellcheck="false" /></div><button id="license-activate" class="primary-button" type="submit" disabled>激活</button></form>
    <div class="license-actions"><button id="license-trial" class="secondary-button" disabled>领取 24 小时试用</button><button id="license-retry" class="text-button" disabled>重新验证</button></div>
    <p class="field-hint">每台电脑可领取一次 Cue 试用，点击后开始计时。卡密首次激活后绑定本机，换电脑需联系管理员。</p>
    <p id="license-preview-note" class="field-hint" ${preview ? '' : 'hidden'}>界面预览 · 授权为模拟状态，不连接真实服务。</p>
  </section>
  <div id="modal-root"></div>
  <div id="toast-region" role="status" aria-live="polite"></div>
`;
}
