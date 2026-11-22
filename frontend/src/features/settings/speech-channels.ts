import { call, copy } from '../../api/wails';
import { state } from '../../app/state';
import { icon } from '../../components/icons';
import { toast } from '../../components/toast';
import type { APIChannel, EndpointCheckResult } from '../../types';
import { esc } from '../../utils/format';
import { channelKeyPreview } from '../../utils/api-key';
import { openAddSpeechModal } from './add-speech-modal';
import { openModelsModal } from './models-modal';
import { channelIsSaved, copyChannelKey, localSpeechChannels, syncChannelDraft } from './channel-state';

export function renderSpeechChannelsPage(page: HTMLElement, renderParent: () => void) {
  let apiChannels = state.draft.speechChannels ||= [];
  let activeId = state.draft.activeSpeechChannelId || '';
  let localEngines = localSpeechChannels();
  let currentCategory: 'all' | 'local' | 'api' = 'all';
  let searchQuery = '';

  function updateDraft() {
    state.draft.speechChannels = apiChannels;
    state.draft.activeSpeechChannelId = activeId;
    syncChannelDraft('speech');
  }

  function render() {
    const allChannels = [...localEngines, ...apiChannels];

    const filtered = allChannels.filter(c => {
      const isLocal = c.provider === 'sherpa' || c.provider === 'local';
      if (currentCategory === 'local' && !isLocal) return false;
      if (currentCategory === 'api' && isLocal) return false;
      if (!searchQuery) return true;
      const q = searchQuery.toLowerCase();
      return c.name.toLowerCase().includes(q) || c.baseUrl.toLowerCase().includes(q) || c.provider.toLowerCase().includes(q);
    });

    const localCount = localEngines.length;
    const apiCount = apiChannels.length;

    page.innerHTML = `
      <div class="channels-container">
        <!-- 顶部“本地和 API”分类筛选栏 (用户核心指定设计) -->
        <div class="speech-tabs-bar card glass">
          <div class="speech-category-pills">
            <button type="button" class="tab-pill ${currentCategory === 'all' ? 'active' : ''}" data-speech-cat="all">
              全部服务 (${allChannels.length})
            </button>
            <button type="button" class="tab-pill ${currentCategory === 'local' ? 'active' : ''}" data-speech-cat="local">
              <span class="pill-icon">💻</span> 本地离线引擎 (${localCount})
            </button>
            <button type="button" class="tab-pill ${currentCategory === 'api' ? 'active' : ''}" data-speech-cat="api">
              <span class="pill-icon">☁️</span> 云端语音 API (${apiCount})
            </button>
          </div>

          <!-- 系统音频采集快速开关 -->
          <div class="speech-top-options">
            <label class="meeting-audio-chip" title="捕获会议中对方的声音，与麦克风分开记录">
              <input type="checkbox" id="toggle-meeting-audio" ${state.draft.meetingAudio ? 'checked' : ''} />
              <span>🎙️ 采集系统音频</span>
            </label>
          </div>
        </div>

        <!-- 顶部操作条 -->
        <div class="channels-topbar card glass">
          <div class="channels-search-box">
            <span class="search-icon">${icon('sliders')}</span>
            <input type="text" id="speech-search" placeholder="搜索本地引擎或语音 API..." value="${esc(searchQuery)}" spellcheck="false" autocomplete="off" />
          </div>

          <div class="channels-topbar-actions">
            <button type="button" id="manage-local-models-btn" class="secondary-button btn-sm open-models-btn" title="打开 Whisper 本地模型管理">
              ${icon('spark')} 本地模型管理
            </button>
            <button type="button" id="add-speech-btn" class="primary-button add-btn" title="添加云端语音服务">
              <span class="add-icon">+</span> 新增云端 API
            </button>
          </div>
        </div>

        <!-- 卡片内容区域 -->
        <div class="channels-content">
          ${filtered.length === 0 ? `
            <div class="channels-empty-state card glass">
              <div class="empty-icon-wrap">🎙️</div>
              <h4>未找到匹配的语音服务</h4>
              <p class="section-desc">当前分类下暂无已添加的语音卡片，点击右上角「+ 新增云端 API」添加首个服务。</p>
            </div>
          ` : `
            <div class="channels-grid">
              ${filtered.map(channel => {
                const isActive = channel.id === activeId;
                const isSaved = channelIsSaved('speech', channel);
                const isLocal = channel.provider === 'sherpa' || channel.provider === 'local';
                const statusText = channel.status === 'success' ? '正常' : channel.status === 'failed' ? '异常' : '未检测';
                const statusClass = channel.status === 'success' ? 'status-good' : channel.status === 'failed' ? 'status-bad' : 'status-pending';
                const maskedKey = isLocal ? '无需 API Key' : channelKeyPreview(channel) || '未提供密钥';

                return `
                  <div class="channel-card card glass ${isActive ? 'active-channel' : ''} ${isLocal ? 'local-engine-card' : ''}" data-speech-id="${esc(channel.id)}">
                    <!-- 卡片顶部 Header -->
                    <div class="channel-card-header">
                      <div class="header-left">
                        <label class="channel-radio-label" title="选中后保存设置生效">
                          <input type="radio" name="active-speech-radio" class="channel-radio" ${isActive ? 'checked' : ''} data-activate-speech="${esc(channel.id)}" />
                          <span class="channel-title" title="${esc(channel.name)}">${esc(channel.name)}</span>
                        </label>
                      </div>
                      <div class="header-badges">
                        ${isActive ? `<span class="badge badge-current">${isSaved ? '当前' : '待保存'}</span>` : ''}
                        <span class="badge ${isLocal ? 'badge-local' : 'badge-api'}">${isLocal ? 'LOCAL 离线' : 'SPEECH API'}</span>
                      </div>
                    </div>

                    <!-- 快捷动作胶囊 -->
                    <div class="channel-action-pills">
                      ${!isActive ? `
                        <button type="button" class="action-pill set-active-pill" data-activate-speech="${esc(channel.id)}">
                          <span>🔗 设为当前</span>
                        </button>
                      ` : `
                        <span class="action-pill active-pill">
                          <span>${isSaved ? '✓ 正在使用' : '待保存后使用'}</span>
                        </span>
                      `}
                      <span class="action-pill ${channel.stream ? 'stream-pill' : ''}">
                        <span>⚡ ${channel.stream ? '实时流式转写' : '分段转写'}</span>
                      </span>
                      ${isLocal ? `<span class="action-pill local-pill"><span>🔒 局域离线隐私</span></span>` : ''}
                    </div>

                    <!-- 核心属性列表 -->
                    <div class="channel-props-list">
                      <div class="prop-row">
                        <span class="prop-label">${isLocal ? '运行凭证:' : 'API Key:'}</span>
                        <span class="prop-value mono ${isLocal ? 'text-green' : 'key-val'}">${esc(maskedKey)}</span>
                        ${!isLocal && (channel.apiKey || channel.hasKey) ? `
                          <button type="button" class="icon-mini-btn copy-key-btn" data-copy-key="${esc(channel.id)}" title="复制 API Key">${icon('spark')}</button>
                        ` : ''}
                      </div>

                      <div class="prop-row">
                        <span class="prop-label">${isLocal ? '引擎类型:' : '服务商:'}</span>
                        <span class="prop-value">${esc(channel.provider === 'sherpa' ? 'Sherpa-onnx (中文流式)' : channel.provider === 'local' ? 'whisper.cpp (分段转写)' : channel.name)}</span>
                        <span class="prop-tag">${esc(channel.provider)}</span>
                      </div>

                      <div class="prop-row">
                        <span class="prop-label">${isLocal ? '运行时:' : '基础地址:'}</span>
                        <span class="prop-value prop-url" title="${esc(channel.baseUrl)}">${esc(isLocal ? '本机运行时，点击自检确认' : channel.baseUrl || '官方默认')}</span>
                        ${!isLocal && channel.baseUrl ? `
                          <button type="button" class="icon-mini-btn copy-url-btn" data-copy-text="${esc(channel.baseUrl)}" title="复制服务地址">${icon('sliders')}</button>
                        ` : ''}
                      </div>

                      <div class="prop-row">
                        <span class="prop-label">转写模型:</span>
                        <span class="prop-value model-badge-val">${esc(channel.model)}</span>
                      </div>
                    </div>

                    <!-- 性能指标卡片区 (对应参考图 1 数据区) -->
                    <div class="channel-metrics-grid">
                      <div class="metric-item">
                        <span class="metric-label">运行状态</span>
                        <span class="metric-val ${statusClass}">${statusText}</span>
                      </div>
                      <div class="metric-item">
                        <span class="metric-label">响应延迟</span>
                        <span class="metric-val">${channel.latency !== undefined ? channel.latency + ' ms' : '--'}</span>
                      </div>
                      <div class="metric-item">
                        <span class="metric-label">转写模式</span>
                        <span class="metric-val ${channel.stream ? 'text-green' : ''}">${channel.stream ? '实时更新' : '整句分段'}</span>
                      </div>
                    </div>

                    <!-- 底部时间元信息 -->
                    <div class="channel-card-meta">
                      <small>🕒 来源: ${esc(channel.createdAt || '近期')}</small>
                      <small class="mode-tag">${isLocal ? '系统内置' : '标准连接'}</small>
                    </div>

                    <!-- 底部快捷图标操作栏 -->
                    <div class="channel-card-toolbar">
                      <button type="button" class="tool-btn test-btn" data-test-speech="${esc(channel.id)}" aria-label="${isLocal ? '自检本地引擎' : '测试语音服务连通性'}">
                        <span class="tool-icon">&gt;_</span>
                      </button>
                      ${!isLocal ? `
                        <button type="button" class="tool-btn copy-btn" data-copy-text="${esc(channel.baseUrl)}" aria-label="复制服务地址">
                          <span class="tool-icon">📋</span>
                        </button>
                        <button type="button" class="tool-btn edit-btn" data-edit-speech="${esc(channel.id)}" aria-label="编辑语音配置">
                          <span class="tool-icon">✏️</span>
                        </button>
                        <button type="button" class="tool-btn delete-btn" data-delete-speech="${esc(channel.id)}" aria-label="删除账号（保存后生效）">
                          <span class="tool-icon">🗑️</span>
                        </button>
                      ` : `
                        <button type="button" class="tool-btn open-models-btn" data-action="manage-models" aria-label="管理本地模型">
                          <span class="tool-icon">📦</span>
                        </button>
                      `}
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
    page.querySelectorAll<HTMLButtonElement>('[data-speech-cat]').forEach(btn => {
      btn.addEventListener('click', () => { currentCategory = btn.dataset.speechCat as 'all' | 'local' | 'api'; render(); });
    });
    const searchInput = page.querySelector<HTMLInputElement>('#speech-search');
    searchInput?.addEventListener('input', () => {
      const cursor = searchInput.selectionStart;
      searchQuery = searchInput.value;
      render();
      const replacement = page.querySelector<HTMLInputElement>('#speech-search');
      replacement?.focus();
      replacement?.setSelectionRange(cursor, cursor);
    });
    page.querySelector<HTMLInputElement>('#toggle-meeting-audio')?.addEventListener('change', event => {
      state.draft.meetingAudio = (event.target as HTMLInputElement).checked;
      toast('采集选项已修改，保存设置后生效');
    });
    page.querySelector('#add-speech-btn')?.addEventListener('click', () => {
      openAddSpeechModal(added => {
        apiChannels.push(...added);
        if (!activeId && apiChannels.length) activeId = apiChannels[0].id;
        updateDraft();
        render();
      });
    });
    page.querySelectorAll<HTMLElement>('.open-models-btn').forEach(el => {
      el.addEventListener('click', (event) => {
        event.preventDefault();
        event.stopPropagation();
        openModelsModal(() => {
          // 保留筛选条件，同时同步弹窗刚修改的模型名称；设置仍由保存按钮统一提交。
          localEngines = localSpeechChannels();
          render();
        });
      });
    });
    page.querySelectorAll<HTMLElement>('[data-activate-speech]').forEach(el => {
      el.addEventListener('click', () => {
        if (![...localEngines, ...apiChannels].some(channel => channel.id === el.dataset.activateSpeech)) return;
        activeId = el.dataset.activateSpeech!;
        updateDraft();
        toast('已选择语音服务，保存设置后生效');
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
        const found = apiChannels.find(channel => channel.id === btn.dataset.copyKey);
        if (found) void copyChannelKey('speech', found);
      });
    });
    page.querySelectorAll<HTMLButtonElement>('[data-test-speech]').forEach(btn => {
      btn.addEventListener('click', async () => {
        const found = [...localEngines, ...apiChannels].find(channel => channel.id === btn.dataset.testSpeech);
        if (!found) return;
        const view = page.firstElementChild;
        btn.disabled = true;
        btn.innerHTML = '<span class="tool-icon">⏳</span>';
        try {
          const ep = { ...found, whisperModel: state.draft.whisperModel, whisperPath: state.draft.whisperPath };
          const res = await call<EndpointCheckResult>('TestChannelEndpoint', 'speech', ep);
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
    page.querySelectorAll<HTMLButtonElement>('[data-delete-speech]').forEach(btn => {
      btn.addEventListener('click', () => {
        apiChannels = apiChannels.filter(channel => channel.id !== btn.dataset.deleteSpeech);
        if (activeId === btn.dataset.deleteSpeech) activeId = apiChannels[0]?.id || '';
        updateDraft();
        toast('语音账号已从草稿移除，保存设置后生效');
        render();
      });
    });
    page.querySelectorAll<HTMLButtonElement>('[data-edit-speech]').forEach(btn => {
      btn.addEventListener('click', () => {
        const found = apiChannels.find(channel => channel.id === btn.dataset.editSpeech);
        if (!found) return;
        openAddSpeechModal(edited => {
          apiChannels = apiChannels.map(channel => channel.id === found.id ? edited[0] : channel);
          updateDraft();
          render();
        }, found);
      });
    });
  }

  render();
}
