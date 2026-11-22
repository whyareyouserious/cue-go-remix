import { call, copy } from '../../api/wails';
import { state } from '../../app/state';
import { icon } from '../../components/icons';
import { toast } from '../../components/toast';
import type { APIChannel, EndpointCheckResult } from '../../types';
import { esc } from '../../utils/format';
import { chatProtocolLabel } from '../../utils/chat-protocol';
import { channelKeyPreview } from '../../utils/api-key';
import { openAddChannelModal } from './add-channel-modal';
import { channelIsSaved, copyChannelKey, syncChannelDraft } from './channel-state';

export function renderChannelsPage(page: HTMLElement, renderParent: () => void) {
  let channels = state.draft.chatChannels ||= [];
  let activeId = state.draft.activeChatChannelId || '';
  let searchQuery = '';

  function updateDraft() {
    state.draft.chatChannels = channels;
    state.draft.activeChatChannelId = activeId;
    syncChannelDraft('chat');
  }

  function render() {
    const filtered = channels.filter(c => {
      if (!searchQuery) return true;
      const q = searchQuery.toLowerCase();
      return c.name.toLowerCase().includes(q) || c.baseUrl.toLowerCase().includes(q) || c.provider.toLowerCase().includes(q);
    });

    page.innerHTML = `
      <div class="channels-container">
        <!-- 顶部工具栏 (参考图 1 头部交互) -->
        <div class="channels-topbar card glass">
          <div class="channels-search-box">
            <span class="search-icon">${icon('sliders')}</span>
            <input type="text" id="channels-search" placeholder="搜索账号或基础地址..." value="${esc(searchQuery)}" spellcheck="false" autocomplete="off" />
          </div>

          <div class="channels-topbar-actions">
            <button type="button" id="channel-access-btn" data-settings-page="access" class="secondary-button btn-sm">本地助手授权</button>
            <span class="channels-count-badge">全部 ${channels.length}</span>
            <button type="button" id="add-channel-btn" class="primary-button add-btn" title="添加 API 账号与渠道">
              <span class="add-icon">+</span> 新增账号
            </button>
          </div>
        </div>

        <!-- 卡片内容区域 -->
        <div class="channels-content">
          ${channels.length === 0 ? `
            <div class="channels-empty-state card glass">
              <div class="empty-icon-wrap">🔑</div>
              <h4>暂未添加任何 API 账号</h4>
              <p class="section-desc">点击右上角「+ 新增账号」添加模型服务，保存设置后生效。</p>
              <div class="empty-actions">
                <button type="button" id="empty-add-btn" class="primary-button">
                  <span class="add-icon">+</span> 立即添加 API 账号
                </button>

              </div>
            </div>
          ` : `
            <div class="channels-grid">
              ${filtered.map(channel => {
                const isActive = channel.id === activeId;
                const isSaved = channelIsSaved('chat', channel);
                const domain = extractDomain(channel.baseUrl) || channel.name;
                const statusText = channel.status === 'success' ? '正常' : channel.status === 'failed' ? '异常' : '未检测';
                const statusClass = channel.status === 'success' ? 'status-good' : channel.status === 'failed' ? 'status-bad' : 'status-pending';
                const maskedKey = channelKeyPreview(channel) || '未提供密钥';

                return `
                  <div class="channel-card card glass ${isActive ? 'active-channel' : ''}" data-channel-id="${esc(channel.id)}">
                    <!-- 卡片顶部 Header -->
                    <div class="channel-card-header">
                      <div class="header-left">
                        <label class="channel-radio-label" title="选中后保存设置生效">
                          <input type="radio" name="active-channel-radio" class="channel-radio" ${isActive ? 'checked' : ''} data-activate-id="${esc(channel.id)}" />
                          <span class="channel-title" title="${esc(channel.name)}">${esc(domain)}</span>
                        </label>
                      </div>
                      <div class="header-badges">
                        ${isActive ? `<span class="badge badge-current">${isSaved ? '当前' : '待保存'}</span>` : ''}
                        <span class="badge badge-api">API</span>
                      </div>
                    </div>

                    <!-- 快捷动作胶囊 -->
                    <div class="channel-action-pills">
                      ${!isActive ? `
                        <button type="button" class="action-pill set-active-pill" data-activate-id="${esc(channel.id)}">
                          <span>🔗 设为当前</span>
                        </button>
                      ` : `
                        <span class="action-pill active-pill">
                          <span>${isSaved ? '✓ 正在使用' : '待保存后使用'}</span>
                        </span>
                      `}
                      <button type="button" class="action-pill stream-pill" data-toggle-stream="${esc(channel.id)}" title="点击切换流式传输">
                        <span>⚡ 流式: ${channel.stream ? '开启' : '关闭'}</span>
                      </button>
                    </div>

                    <!-- 核心属性列表 -->
                    <div class="channel-props-list">
                      <div class="prop-row">
                        <span class="prop-label">API Key:</span>
                        <span class="prop-value mono key-val">${esc(maskedKey)}</span>
                        <button type="button" class="icon-mini-btn copy-key-btn" data-copy-key="${esc(channel.id)}" title="复制 API Key">${icon('spark')}</button>
                      </div>

                      <div class="prop-row">
                        <span class="prop-label">供应商:</span>
                        <span class="prop-value">${esc(channel.name)}</span>
                        <span class="prop-tag">${esc(channel.provider)}</span>
                      </div>

                      <div class="prop-row">
                        <span class="prop-label">基础地址:</span>
                        <span class="prop-value prop-url" title="${esc(channel.baseUrl)}">${esc(channel.baseUrl || '官方默认')}</span>
                        <button type="button" class="icon-mini-btn copy-url-btn" data-copy-text="${esc(channel.baseUrl)}" title="复制服务地址">${icon('sliders')}</button>
                      </div>

                      <div class="prop-row">
                        <span class="prop-label">模型配置:</span>
                        <span class="prop-value model-badge-val">${esc(channel.model)}</span>
                      </div>
                    </div>

                    <!-- 连通性指标卡片区 (对应参考图 1 的数据区) -->
                    <div class="channel-metrics-grid">
                      <div class="metric-item">
                        <span class="metric-label">链路状态</span>
                        <span class="metric-val ${statusClass}">${statusText}</span>
                      </div>
                      <div class="metric-item">
                        <span class="metric-label">响应延迟</span>
                        <span class="metric-val">${channel.latency !== undefined ? channel.latency + ' ms' : '--'}</span>
                      </div>
                      <div class="metric-item">
                        <span class="metric-label">密钥配置</span>
                        <span class="metric-val ${channel.hasKey ? 'text-green' : 'text-mut'}">${channel.hasKey ? '已配置' : '无 Key'}</span>
                      </div>
                    </div>

                    <!-- 底部时间元信息 -->
                    <div class="channel-card-meta">
                      <small>🕒 添加时间: ${esc(channel.createdAt || '近期')}</small>
                      <small class="mode-tag">${chatProtocolLabel(channel)}</small>
                    </div>

                    <!-- 底部快捷图标操作栏 (参考图 1 底部按钮排) -->
                    <div class="channel-card-toolbar">
                      <button type="button" class="tool-btn test-btn" data-test-id="${esc(channel.id)}" aria-label="测试连接与延迟">
                        <span class="tool-icon">&gt;_</span>
                      </button>
                      <button type="button" class="tool-btn copy-btn" data-copy-text="${esc(channel.baseUrl)}" aria-label="复制服务地址">
                        <span class="tool-icon">📋</span>
                      </button>
                      <button type="button" class="tool-btn edit-btn" data-edit-id="${esc(channel.id)}" aria-label="编辑账号配置">
                        <span class="tool-icon">✏️</span>
                      </button>
                      <button type="button" class="tool-btn delete-btn" data-delete-id="${esc(channel.id)}" aria-label="删除账号（保存后生效）">
                        <span class="tool-icon">🗑️</span>
                      </button>
                    </div>
                  </div>
                `;
              }).join('')}
            </div>
          `}
        </div>
      </div>
    `;

    bindEvents();
  }

  function bindEvents() {
    const searchInput = page.querySelector<HTMLInputElement>('#channels-search');
    searchInput?.addEventListener('input', () => {
      const cursor = searchInput.selectionStart;
      searchQuery = searchInput.value;
      render();
      const replacement = page.querySelector<HTMLInputElement>('#channels-search');
      replacement?.focus();
      replacement?.setSelectionRange(cursor, cursor);
    });
    page.querySelector('#channel-access-btn')?.addEventListener('click', () => {
      state.settingsTab = 'access';
      renderParent();
    });
    const openAdd = () => openAddChannelModal(added => {
      channels.push(...added);
      if (!activeId && channels.length) activeId = channels[0].id;
      updateDraft();
      render();
    });
    page.querySelector('#add-channel-btn')?.addEventListener('click', openAdd);
    page.querySelector('#empty-add-btn')?.addEventListener('click', openAdd);

    page.querySelectorAll<HTMLElement>('[data-activate-id]').forEach(el => {
      el.addEventListener('click', () => {
        if (!channels.some(channel => channel.id === el.dataset.activateId)) return;
        activeId = el.dataset.activateId!;
        updateDraft();
        toast('已选择模型账号，保存设置后生效');
        render();
      });
    });
    page.querySelectorAll<HTMLButtonElement>('[data-toggle-stream]').forEach(btn => {
      btn.addEventListener('click', () => {
        const found = channels.find(channel => channel.id === btn.dataset.toggleStream);
        if (!found) return;
        found.stream = !found.stream;
        found.status = 'untested';
        found.latency = undefined;
        updateDraft();
        render();
      });
    });
    page.querySelectorAll<HTMLButtonElement>('[data-copy-text]').forEach(btn => {
      btn.addEventListener('click', () => {
        void copy(btn.dataset.copyText || '').then(() => toast('已复制到剪贴板')).catch(() => toast('复制失败', true));
      });
    });
    page.querySelectorAll<HTMLButtonElement>('[data-copy-key]').forEach(btn => {
      btn.addEventListener('click', () => {
        const found = channels.find(channel => channel.id === btn.dataset.copyKey);
        if (found) void copyChannelKey('chat', found);
      });
    });
    page.querySelectorAll<HTMLButtonElement>('[data-test-id]').forEach(btn => {
      btn.addEventListener('click', async () => {
        const found = channels.find(channel => channel.id === btn.dataset.testId);
        if (!found) return;
        const view = page.firstElementChild;
        btn.disabled = true;
        btn.innerHTML = '<span class="tool-icon">⏳</span>';
        try {
          const res = await call<EndpointCheckResult>('TestChannelEndpoint', 'chat', { ...found });
          found.status = res.success ? 'success' : 'failed';
          found.latency = res.latency;
          toast(res.message, !res.success);
        } catch (error) {
          found.status = 'failed';
          found.latency = undefined;
          toast(`检测失败：${error instanceof Error ? error.message : String(error)}`, true);
        } finally {
          if (page.firstElementChild === view) render();
        }
      });
    });
    page.querySelectorAll<HTMLButtonElement>('[data-delete-id]').forEach(btn => {
      btn.addEventListener('click', () => {
        channels = channels.filter(channel => channel.id !== btn.dataset.deleteId);
        if (!channels.some(channel => channel.id === activeId)) activeId = channels[0]?.id || '';
        updateDraft();
        toast('账号已从草稿移除，保存设置后生效');
        render();
      });
    });
    page.querySelectorAll<HTMLButtonElement>('[data-edit-id]').forEach(btn => {
      btn.addEventListener('click', () => {
        const found = channels.find(channel => channel.id === btn.dataset.editId);
        if (!found) return;
        openAddChannelModal(edited => {
          channels = channels.map(channel => channel.id === found.id ? edited[0] : channel);
          updateDraft();
          render();
        }, found);
      });
    });
  }

  function extractDomain(urlStr: string): string {
    if (!urlStr) return '';
    try {
      const u = new URL(urlStr);
      return u.hostname;
    } catch {
      return urlStr.replace(/^https?:\/\//, '').split('/')[0];
    }
  }

  render();
}
