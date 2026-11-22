import { call } from '../../api/wails';
import { state } from '../../app/state';
import { icon } from '../../components/icons';
import { toast } from '../../components/toast';
import type { Settings } from '../../types';
import { esc } from '../../utils/format';
import { formatShortcut, parseShortcut } from '../../utils/shortcut';
import { renderStatus } from '../toolbar';

interface ShortcutMeta {
  key: string;
  name: string;
  desc: string;
  defaultVal: string;
}

const SHORTCUT_METAS: ShortcutMeta[] = [
  { key: 'say', name: '快速提问 (Say)', desc: '呼出快速对话并生成精简回答', defaultVal: 'CmdOrCtrl+Enter' },
  { key: 'assist', name: '深度助手 (Assist)', desc: '基于全屏或选中内容的深度推理', defaultVal: 'CmdOrCtrl+Shift+Enter' },
  { key: 'leetcode', name: '屏幕解题 (LeetCode)', desc: '截取屏幕算法题目并提供结构化解法', defaultVal: 'CmdOrCtrl+H' },
  { key: 'toggle', name: '显示 / 隐藏助手', desc: '快速展开或收起悬浮条', defaultVal: 'CmdOrCtrl+Shift+/' },
  { key: 'passthrough', name: '恢复鼠标交互 (穿透恢复)', desc: '开启鼠标穿透后，按此键立即恢复悬浮助手交互', defaultVal: 'CmdOrCtrl+Shift+I' },
  { key: 'quit', name: '退出应用', desc: '安全退出 Cue 桌面程序', defaultVal: 'CmdOrCtrl+Shift+X' },
];

function normalize(key: string): string {
  return formatShortcut(key).replace(/\s+/g, '').toUpperCase();
}

function shortcutError(err: unknown): string {
  return (err instanceof Error ? err.message : String(err))
    .replace(/CmdOrCtrl/gi, formatShortcut('CmdOrCtrl'))
    .replace(/OptionOrAlt/gi, formatShortcut('Alt'));
}

export function renderShortcutsPage(page: HTMLElement) {
  if (!state.draft.shortcuts) {
    state.draft.shortcuts = structuredClone(state.settings.shortcuts || {
      say: 'CmdOrCtrl+Enter',
      assist: 'CmdOrCtrl+Shift+Enter',
      leetcode: 'CmdOrCtrl+H',
      toggle: 'CmdOrCtrl+Shift+/',
      quit: 'CmdOrCtrl+Shift+X',
      passthrough: 'CmdOrCtrl+Shift+I',
    });
  }

  const current = state.draft.shortcuts;

  page.innerHTML = `
    <div class="shortcuts-container">
      <div class="shortcuts-header card glass">
        <div class="shortcuts-header-text">
          <h4>${icon('shield')} 全局快捷键设置</h4>
          <p class="section-desc">修改全局快捷键并立即生效；若按键冲突或被系统占用将自动回滚，恢复穿透交互能力受严密保护。</p>
        </div>
        <div class="shortcuts-header-actions">
          <button id="reset-shortcuts-btn" class="secondary-button btn-sm">恢复默认</button>
          <button id="apply-shortcuts-btn" class="primary-button btn-sm">应用按键</button>
        </div>
      </div>

      <div id="shortcuts-conflict-banner" class="callout callout-danger" style="display: none;"></div>

      <div class="shortcuts-list">
        ${SHORTCUT_METAS.map(item => `
          <div class="shortcut-item card glass" data-shortcut-key="${item.key}">
            <div class="shortcut-info">
              <strong>${esc(item.name)}</strong>
              <small>${esc(item.desc)}</small>
            </div>
            <div class="shortcut-input-wrapper">
              <input type="text" class="shortcut-input" data-shortcut="${item.key}" value="${esc(formatShortcut(current[item.key] || item.defaultVal))}" spellcheck="false" autocomplete="off" />
              <span class="shortcut-item-error" data-error-for="${item.key}"></span>
            </div>
          </div>
        `).join('')}
      </div>
      <p class="field-hint">说明：快捷键需点击上方“应用按键”单独向系统注册生效；${state.status.platform === 'darwin' ? '支持 ⌘（Command）、⌃（Control）、⇧（Shift）、⌥（Option），也可输入对应英文名称' : '支持 Ctrl、Shift、Alt'}，各按键用 + 连接。底部“保存设置”不会自动覆盖系统热键。</p>
    </div>
  `;

  const banner = page.querySelector<HTMLDivElement>('#shortcuts-conflict-banner');
  const applyBtn = page.querySelector<HTMLButtonElement>('#apply-shortcuts-btn');
  const resetBtn = page.querySelector<HTMLButtonElement>('#reset-shortcuts-btn');
  const inputs = page.querySelectorAll<HTMLInputElement>('input[data-shortcut]');

  function checkConflicts(): boolean {
    const values: Record<string, string> = {};
    const seen: Record<string, string> = {};
    let hasConflict = false;
    let emptyKey: string | null = null;

    inputs.forEach(input => {
      const k = input.dataset.shortcut!;
      const val = input.value.trim();
      values[k] = val;
      const errorEl = page.querySelector<HTMLSpanElement>(`[data-error-for="${k}"]`);
      if (errorEl) errorEl.textContent = '';
      input.classList.remove('input-error');

      if (!val) {
        emptyKey = k;
      }
    });

    if (emptyKey) {
      if (banner) {
        banner.style.display = 'block';
        banner.textContent = '快捷键不能为空，请填写完整按键组合。';
      }
      if (applyBtn) applyBtn.disabled = true;
      return false;
    }

    // 检查冲突
    for (const [k, v] of Object.entries(values)) {
      const norm = normalize(v);
      if (seen[norm]) {
        hasConflict = true;
        const otherKey = seen[norm];
        const otherMeta = SHORTCUT_METAS.find(m => m.key === otherKey)?.name || otherKey;
        const currentMeta = SHORTCUT_METAS.find(m => m.key === k)?.name || k;

        const err1 = page.querySelector<HTMLSpanElement>(`[data-error-for="${k}"]`);
        const err2 = page.querySelector<HTMLSpanElement>(`[data-error-for="${otherKey}"]`);
        if (err1) err1.textContent = `与 [${otherMeta}] 冲突`;
        if (err2) err2.textContent = `与 [${currentMeta}] 冲突`;

        const in1 = page.querySelector<HTMLInputElement>(`input[data-shortcut="${k}"]`);
        const in2 = page.querySelector<HTMLInputElement>(`input[data-shortcut="${otherKey}"]`);
        in1?.classList.add('input-error');
        in2?.classList.add('input-error');

        if (banner) {
          banner.style.display = 'block';
          banner.textContent = `按键冲突：[${currentMeta}] 与 [${otherMeta}] 使用了相同快捷键 (${formatShortcut(v)})，请调整后再应用。`;
        }
      } else {
        seen[norm] = k;
      }
    }

    if (!hasConflict) {
      if (banner) banner.style.display = 'none';
      if (applyBtn) applyBtn.disabled = false;
    } else {
      if (applyBtn) applyBtn.disabled = true;
    }

    return !hasConflict;
  }

  inputs.forEach(input => {
    input.addEventListener('input', () => {
      const k = input.dataset.shortcut!;
      if (state.draft.shortcuts) {
        state.draft.shortcuts[k] = parseShortcut(input.value, state.settings.shortcuts?.[k]);
      }
      checkConflicts();
    });
  });

  // 应用快捷键（互斥保护 + 异步结束不覆盖已切换页面）
  applyBtn?.addEventListener('click', async () => {
    if (!checkConflicts()) return;
    const currentShortcuts: Record<string, string> = {};
    inputs.forEach(input => {
      const key = input.dataset.shortcut!;
      currentShortcuts[key] = parseShortcut(input.value, state.settings.shortcuts?.[key]);
    });

    applyBtn.disabled = true;
    if (resetBtn) resetBtn.disabled = true;
    applyBtn.textContent = '正在注册…';
    try {
      const updated = await call<Settings>('UpdateShortcuts', currentShortcuts);
      // 关键单向同步：同时更新 settings 与 draft，避免普通保存相互覆盖
      state.settings.shortcuts = updated.shortcuts;
      state.draft.shortcuts = structuredClone(updated.shortcuts);
      renderStatus();
      toast('快捷键已更新并在系统生效');
      if (state.settingsTab === 'shortcuts' && page.isConnected) {
        renderShortcutsPage(page);
      }
    } catch (err: unknown) {
      toast(`快捷键更新失败: ${shortcutError(err)}`, true);
      if (state.settingsTab === 'shortcuts' && page.isConnected) {
        renderShortcutsPage(page);
      }
    } finally {
      if (applyBtn.isConnected) {
        applyBtn.disabled = false;
        applyBtn.textContent = '应用按键';
      }
      if (resetBtn?.isConnected) {
        resetBtn.disabled = false;
      }
    }
  });

  // 恢复默认快捷键（互斥保护 + 异步结束不覆盖已切换页面）
  resetBtn?.addEventListener('click', async () => {
    resetBtn.disabled = true;
    if (applyBtn) applyBtn.disabled = true;
    resetBtn.textContent = '正在恢复…';
    try {
      const updated = await call<Settings>('ResetShortcuts');
      state.settings.shortcuts = updated.shortcuts;
      state.draft.shortcuts = structuredClone(updated.shortcuts);
      renderStatus();
      toast('已恢复为默认快捷键');
      if (state.settingsTab === 'shortcuts' && page.isConnected) {
        renderShortcutsPage(page);
      }
    } catch (err: unknown) {
      toast(`恢复默认快捷键失败: ${shortcutError(err)}`, true);
    } finally {
      if (resetBtn.isConnected) {
        resetBtn.disabled = false;
        resetBtn.textContent = '恢复默认';
      }
      if (applyBtn?.isConnected) {
        applyBtn.disabled = false;
      }
    }
  });
}
