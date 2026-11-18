import { call } from '../api/wails';

export function recordUIEvent(event: 'settings-open' | 'provider-select' | 'key-focus' | 'settings-save' | 'panel-toggle' | 'sidebar-toggle', value: string) {
  // 仅记录固定操作名和类别；日志失败不影响交互，不传入任何表单内容。
  void call('RecordUIEvent', event, value).catch(() => { });
}
