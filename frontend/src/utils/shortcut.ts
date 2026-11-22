import { state } from '../app/state';

const keyAliases: Record<string, string> = {
  cmdorctrl: 'CmdOrCtrl', cmd: 'CmdOrCtrl', command: 'CmdOrCtrl', '⌘': 'CmdOrCtrl',
  ctrl: 'Ctrl', control: 'Ctrl', '⌃': 'Ctrl',
  alt: 'Alt', option: 'Alt', optionoralt: 'Alt', '⌥': 'Alt',
  shift: 'Shift', '⇧': 'Shift',
  enter: 'Enter', return: 'Enter', '↵': 'Enter',
  super: 'Super', win: 'Super',
};

function shortcutKeys(accel: string): string[] {
  return accel.split('+').map(key => {
    const trimmed = key.trim();
    return keyAliases[trimmed.toLowerCase()] || trimmed;
  });
}

/** 将内部快捷键格式化为当前系统的按键名称；short 用于工具栏紧凑提示。 */
export function formatShortcut(accel: string, short = false): string {
  const isMac = state.status.platform === 'darwin';
  return shortcutKeys(accel).map(key => {
    if (key === 'CmdOrCtrl') key = isMac ? 'Cmd' : 'Ctrl';
    if (isMac) {
      const symbols: Record<string, string> = { Cmd: '⌘', Super: '⌘', Ctrl: '⌃', Alt: '⌥', Shift: '⇧' };
      return symbols[key] || (short && key === 'Enter' ? '↵' : key);
    }
    return key === 'Super' ? 'Win' : key;
  }).join(short ? ' ' : ' + ').trim();
}

/** 将编辑框的本机按键写法转为注册格式；等价时保留已保存值，避免仅浏览就产生修改。 */
export function parseShortcut(value: string, saved = ''): string {
  const accelerator = shortcutKeys(value).join('+');
  return formatShortcut(accelerator).toLowerCase() === formatShortcut(saved).toLowerCase()
    ? saved : accelerator;
}

/**
 * 获取指定动作当前已生效的快捷键，优先从已生效的 state.settings.shortcuts 读取
 */
export function getActiveShortcut(action: string, short = false): string {
  const current = state.settings.shortcuts?.[action] || {
    say: 'CmdOrCtrl+Enter',
    assist: 'CmdOrCtrl+Shift+Enter',
    leetcode: 'CmdOrCtrl+H',
    toggle: 'CmdOrCtrl+Shift+/',
    quit: 'CmdOrCtrl+Shift+X',
    passthrough: 'CmdOrCtrl+Shift+I',
  }[action] || '';
  return formatShortcut(current, short);
}
