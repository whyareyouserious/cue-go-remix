import { call, copy } from '../../api/wails';
import { icon } from '../../components/icons';
import { toast } from '../../components/toast';
import type { RecentError } from '../../types';
import { esc } from '../../utils/format';

const CATEGORY_NAMES: Record<string, string> = {
  'protocol-mismatch': '聊天接口协议不兼容，请检查 Responses / Chat Completions 选择',
  'http-401': 'API Key 无效 / 未授权 (HTTP 401)',
  'http-403': '无权访问 / 权限拒绝 (HTTP 403)',
  'http-404': '地址错误 / 模型未找到 (HTTP 404)',
  'http-429': '请求频次受限 / 配额不足 (HTTP 429)',
  'http-5xx': '上游服务错误 (HTTP 5xx)',
  'timeout': '网络或服务响应超时',
  'network-error': '网络连接失败 / 端口拒绝',
  'permission-denied': '系统权限缺失 / 被拒绝',
  'shortcut-conflict': '快捷键冲突或注册失败',
  'screen-protection-unavailable': '录屏共享排除不支持',
  'audio-runtime-warning': '音频运行时或模型异常',
  'applink-warning': '本地助手通信告警',
  'cancelled': '用户或系统已取消',
  'general-error': '通用运行时错误',
  'general-warning': '通用运行告警',
};

export async function renderDiagnosticsPage(page: HTMLElement) {
  page.innerHTML = `
    <div class="diagnostics-container">
      <div class="diagnostics-header card glass">
        <div class="diagnostics-header-text">
          <h4>${icon('shield')} 诊断与排障</h4>
          <p class="section-desc">查看近期运行时事件分类、打开本机日志或导出脱敏诊断报告以协助排查问题。</p>
        </div>
        <div class="diagnostics-actions">
          <button id="open-logs-btn" class="secondary-button btn-sm">${icon('sliders')} 打开日志目录</button>
          <button id="copy-diag-btn" class="secondary-button btn-sm">复制诊断</button>
          <button id="export-diag-btn" class="primary-button btn-sm">${icon('spark')} 导出脱敏诊断</button>
        </div>
      </div>

      <div class="settings-section">
        <div class="recent-errors-header">
          <div class="recent-errors-title">
            <strong>近期运行状态与脱敏分类</strong>
            <small>仅记录固定事件类型与枚举代码，绝不留存密钥、正文或个人资料</small>
          </div>
          <button id="clear-errors-btn" class="secondary-button btn-sm">清空记录</button>
        </div>

        <div id="recent-errors-list" class="recent-errors-list">
          <div class="diagnostics-loading">正在读取近期记录…</div>
        </div>
      </div>

      <div class="callout">
        <strong>脱敏安全保障说明：</strong>
        <p style="margin: 4px 0 0; font-size: 11.5px; line-height: 1.5; color: var(--tx-mut);">
          导出的诊断报告仅包含真实版本 (0.1.0)、操作系统与架构、运行时间、脱敏提供商代码与白名单日志行；严格屏蔽 API Key、对话与会议转录正文、个人定位与项目履历。
        </p>
      </div>
    </div>
  `;

  // 打开日志目录
  page.querySelector('#open-logs-btn')?.addEventListener('click', async () => {
    try {
      await call('OpenLogsDir');
      toast('已打开日志目录');
    } catch (err: unknown) {
      toast(`打开日志目录失败: ${err instanceof Error ? err.message : String(err)}`, true);
    }
  });

  // 读取与渲染近期错误列表
  const listEl = page.querySelector<HTMLDivElement>('#recent-errors-list');
  async function loadErrors() {
    if (!listEl) return;
    try {
      const errors = await call<RecentError[]>('GetRecentErrors');
      if (!errors || errors.length === 0) {
        listEl.innerHTML = '<div class="empty-state">近期无运行异常与错误记录，运行状态良好。</div>';
        return;
      }
      listEl.innerHTML = errors.map(err => {
        const timeStr = new Date(err.time).toLocaleTimeString();
        const catName = CATEGORY_NAMES[err.category] || err.category;
        const isErr = err.level === 'error';
        return `
          <div class="recent-error-row">
            <span class="error-badge ${isErr ? 'badge-error' : 'badge-warn'}">${isErr ? '错误' : '告警'}</span>
            <span class="error-time">${timeStr}</span>
            <span class="error-cat">${esc(catName)}</span>
            <code class="error-code">${esc(err.category)}</code>
          </div>
        `;
      }).join('');
    } catch {
      listEl.innerHTML = '<div class="empty-state">读取近期记录失败</div>';
    }
  }

  void loadErrors();

  // 清空错误列表
  page.querySelector('#clear-errors-btn')?.addEventListener('click', async () => {
    try {
      await call('ClearRecentErrors');
      toast('已清空近期错误记录');
      void loadErrors();
    } catch (err: unknown) {
      toast(`清空失败: ${err instanceof Error ? err.message : String(err)}`, true);
    }
  });

  // 复制诊断
  page.querySelector('#copy-diag-btn')?.addEventListener('click', async () => {
    try {
      const text = await call<string>('ExportDiagnostics');
      await copy(text);
      toast('已复制脱敏诊断至剪贴板');
    } catch {
      toast('复制失败，请重试', true);
    }
  });

  // 真实写盘导出脱敏诊断报告
  const exportBtn = page.querySelector<HTMLButtonElement>('#export-diag-btn');
  exportBtn?.addEventListener('click', async () => {
    exportBtn.disabled = true;
    exportBtn.textContent = '正在保存…';
    try {
      const savedPath = await call<string>('SaveDiagnosticsToFile');
      if (savedPath) {
        toast(`诊断文件已保存至: ${savedPath}`);
      }
    } catch (err: unknown) {
      toast(`导出失败: ${err instanceof Error ? err.message : String(err)}`, true);
    } finally {
      if (exportBtn.isConnected) {
        exportBtn.disabled = false;
        exportBtn.innerHTML = `${icon('spark')} 导出脱敏诊断`;
      }
    }
  });
}
