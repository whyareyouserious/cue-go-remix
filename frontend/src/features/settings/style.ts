import { state } from '../../app/state';
import { icon } from '../../components/icons';
import { toast } from '../../components/toast';
import type { ProjectExperience, Scenario } from '../../types';
import { esc } from '../../utils/format';
import { readSettingsPage } from './form';

export function renderStylePage(page: HTMLElement) {
  if (!state.draft.scenarios || state.draft.scenarios.length === 0) {
    state.draft.scenarios = [
      {
        id: 'default',
        name: '默认场景',
        profile: { role: '', industry: '', years: '', skills: '', strengths: '', goals: '', projects: [] },
        preferences: { length: '', tone: '', structure: '', examples: '', custom: '' },
      },
    ];
  }
  if (!state.draft.activeScenarioId) {
    state.draft.activeScenarioId = state.draft.scenarios[0].id;
  }

  const currentScenario = state.draft.scenarios.find(s => s.id === state.draft.activeScenarioId) || state.draft.scenarios[0];
  const { profile, preferences } = currentScenario;

  page.innerHTML = `
    <div class="style-container">
      <div class="scenario-bar card glass">
        <div class="scenario-selector-group">
          <label for="scenario-select" class="scenario-label">${icon('spark')} 当前场景</label>
          <select id="scenario-select" class="scenario-select">
            ${state.draft.scenarios.map(s => `<option value="${esc(s.id)}" ${s.id === currentScenario.id ? 'selected' : ''}>${esc(s.name)}</option>`).join('')}
          </select>
        </div>
        <div class="scenario-actions">
          <button id="scenario-add" class="secondary-button btn-sm" title="新建场景">${icon('add')} 新建</button>
          <button id="scenario-rename" class="secondary-button btn-sm" title="重命名当前场景">重命名</button>
          <button id="scenario-delete" class="secondary-button btn-sm btn-danger" title="删除当前场景" ${state.draft.scenarios.length <= 1 ? 'disabled' : ''}>删除</button>
        </div>
      </div>

      <div id="rename-box" class="scenario-rename-box card glass" style="display: none;">
        <input id="rename-input" type="text" class="rename-input" maxlength="50" placeholder="请输入新的场景名称" value="${esc(currentScenario.name)}" />
        <button id="rename-confirm" class="primary-button btn-sm">保存名称</button>
        <button id="rename-cancel" class="secondary-button btn-sm">取消</button>
      </div>

      <div class="settings-section">
        <h4>${icon('shield')} 个人定位与背景</h4>
        <p class="section-desc">当前场景的事实信息，回答时作为背景参考（场景设置优先于全局规则；解题等模式保持独立）。</p>
        <div class="grid-2">
          <div class="field">
            <label for="profile-role">岗位 / 身份</label>
            <input id="profile-role" data-profile="role" type="text" maxlength="100" placeholder="如：架构师 / 资深研发 / 产品经理" value="${esc(profile.role || '')}" />
          </div>
          <div class="field">
            <label for="profile-industry">所属行业</label>
            <input id="profile-industry" data-profile="industry" type="text" maxlength="100" placeholder="如：金融科技 / 云原生 / 电商" value="${esc(profile.industry || '')}" />
          </div>
        </div>
        <div class="grid-2">
          <div class="field">
            <label for="profile-years">从业年限</label>
            <input id="profile-years" data-profile="years" type="text" maxlength="50" placeholder="如：5年+ / 10年" value="${esc(profile.years || '')}" />
          </div>
          <div class="field">
            <label for="profile-skills">技术栈 / 专长</label>
            <input id="profile-skills" data-profile="skills" type="text" maxlength="500" placeholder="如：Go, K8s, 高并发系统, DDD" value="${esc(profile.skills || '')}" />
          </div>
        </div>
        <div class="grid-2">
          <div class="field">
            <label for="profile-strengths">核心擅长</label>
            <input id="profile-strengths" data-profile="strengths" type="text" maxlength="500" placeholder="如：分布式高可用、性能排障" value="${esc(profile.strengths || '')}" />
          </div>
          <div class="field">
            <label for="profile-goals">业务 / 阶段目标</label>
            <input id="profile-goals" data-profile="goals" type="text" maxlength="500" placeholder="如：主导核心重构、晋升技术专家" value="${esc(profile.goals || '')}" />
          </div>
        </div>

        <div class="projects-section">
          <div class="projects-header">
            <label>核心项目职责与成果</label>
            <button id="project-add" class="secondary-button btn-sm">${icon('add')} 添加项目经历</button>
          </div>
          <div id="project-list" class="project-list">
            ${(profile.projects || []).map((proj, idx) => `
              <div class="project-item card glass" data-index="${idx}">
                <div class="project-row">
                  <input type="text" class="project-name" data-project="name" maxlength="100" placeholder="项目名称，如：核心支付网关" value="${esc(proj.name || '')}" />
                  <input type="text" class="project-role" data-project="role" maxlength="100" placeholder="担任角色，如：架构负责人" value="${esc(proj.role || '')}" />
                  <button class="icon-button project-del-btn" data-project-del="${idx}" title="删除此项目">${icon('close')}</button>
                </div>
                <textarea class="project-achievements" data-project="achievements" rows="2" maxlength="2000" placeholder="项目职责与量化成果，如：主导微服务治理，单机QPS提升300%，支撑千万级DAU">${esc(proj.achievements || '')}</textarea>
              </div>
            `).join('')}
          </div>
        </div>
      </div>

      <div class="settings-section">
        <h4>${icon('sliders')} 回答风格偏好</h4>
        <p class="section-desc">定制模型在此场景下的表达方式与行文规范（优先于全局规则）。</p>
        <div class="grid-2">
          <div class="field">
            <label for="pref-length">篇幅长度</label>
            <select id="pref-length" data-preference="length">
              <option value="" ${!preferences.length ? 'selected' : ''}>未指定（沿用全局规则 / 模型默认）</option>
              <option value="concise" ${preferences.length === 'concise' ? 'selected' : ''}>极简扼要（直击核心，避免赘述）</option>
              <option value="balanced" ${preferences.length === 'balanced' ? 'selected' : ''}>适中平衡（重点突出，详略得当）</option>
              <option value="detailed" ${preferences.length === 'detailed' ? 'selected' : ''}>详尽充分（深度解析，完整推导）</option>
            </select>
          </div>
          <div class="field">
            <label for="pref-tone">表达语气</label>
            <select id="pref-tone" data-preference="tone">
              <option value="" ${!preferences.tone ? 'selected' : ''}>未指定（沿用全局规则 / 模型默认）</option>
              <option value="professional" ${preferences.tone === 'professional' ? 'selected' : ''}>专业沉稳（客观、严谨、工程师视角）</option>
              <option value="direct" ${preferences.tone === 'direct' ? 'selected' : ''}>坦诚直接（开门见山，直陈利弊）</option>
              <option value="inspiring" ${preferences.tone === 'inspiring' ? 'selected' : ''}>启发引导（多角度启发，循循善诱）</option>
              <option value="casual" ${preferences.tone === 'casual' ? 'selected' : ''}>轻松自然（平易近人，口语化表达）</option>
            </select>
          </div>
        </div>
        <div class="grid-2">
          <div class="field">
            <label for="pref-structure">逻辑结构</label>
            <select id="pref-structure" data-preference="structure">
              <option value="" ${!preferences.structure ? 'selected' : ''}>未指定（沿用全局规则 / 模型默认）</option>
              <option value="conclusion-first" ${preferences.structure === 'conclusion-first' ? 'selected' : ''}>结论先行（先给答案，再做展开）</option>
              <option value="bullet-points" ${preferences.structure === 'bullet-points' ? 'selected' : ''}>条理分点（结构化列表，多用序号）</option>
              <option value="flexible" ${preferences.structure === 'flexible' ? 'selected' : ''}>自然流式（依主题灵活行文）</option>
            </select>
          </div>
          <div class="field">
            <label for="pref-examples">举例说明</label>
            <select id="pref-examples" data-preference="examples">
              <option value="" ${!preferences.examples ? 'selected' : ''}>未指定（沿用全局规则 / 模型默认）</option>
              <option value="moderate" ${preferences.examples === 'moderate' ? 'selected' : ''}>适度举例（在关键复杂处穿插案例）</option>
              <option value="always" ${preferences.examples === 'always' ? 'selected' : ''}>务必举例（必须附带具体实际示例）</option>
              <option value="never" ${preferences.examples === 'never' ? 'selected' : ''}>避免举例（仅阐述原理，无需举例）</option>
            </select>
          </div>
        </div>
        <div class="field">
          <label for="pref-custom">自定义补充风格</label>
          <input id="pref-custom" data-preference="custom" type="text" maxlength="1000" placeholder="如：多用类比解释概念；遇多方案时对比优缺点" value="${esc(preferences.custom || '')}" />
        </div>
      </div>

      <div class="settings-section compatibility-section">
        <h4>${icon('settings')} 全局语言与补充规则</h4>
        <div class="field">
          <label for="answer-language">回答 / 转写语言</label>
          <select id="answer-language" data-setting="language">
            ${[['zh', '中文'], ['en', 'English'], ['auto', '自动'], ['ja', '日本語'], ['ko', '한국어'], ['es', 'Español']].map(([v, label]) => `<option value="${v}" ${state.draft.language === v ? 'selected' : ''}>${label}</option>`).join('')}
          </select>
        </div>
        <div class="field">
          <label for="ai-rules">全局回答风格 <span>通用规则</span></label>
          <textarea id="ai-rules" data-setting="aiRules" rows="3" maxlength="2000" placeholder="例如：简洁自然，先给结论；必要时列出要点。">${esc(state.draft.aiRules || '')}</textarea>
        </div>
        <div class="field">
          <label for="background">全局背景说明 <span>通用参考</span></label>
          <textarea id="background" data-setting="background" rows="3" maxlength="30000" placeholder="全局适用的背景说明…">${esc(state.draft.background || '')}</textarea>
        </div>
      </div>
    </div>
  `;

  // 绑定场景切换事件（先读取当前表单，防丢草稿）
  const scenarioSelect = page.querySelector<HTMLSelectElement>('#scenario-select');
  scenarioSelect?.addEventListener('change', () => {
    readSettingsPage();
    state.draft.activeScenarioId = scenarioSelect.value;
    renderStylePage(page);
  });

  // 新建场景
  page.querySelector('#scenario-add')?.addEventListener('click', () => {
    readSettingsPage();
    const count = (state.draft.scenarios || []).length + 1;
    const newId = `scenario-${Date.now()}`;
    const newScenario: Scenario = {
      id: newId,
      name: `新场景 ${count}`,
      profile: { role: '', industry: '', years: '', skills: '', strengths: '', goals: '', projects: [] },
      preferences: { length: '', tone: '', structure: '', examples: '', custom: '' },
    };
    state.draft.scenarios.push(newScenario);
    state.draft.activeScenarioId = newId;
    renderStylePage(page);
  });

  // 原生行内重命名场景（杜绝 window.prompt）
  const renameBox = page.querySelector<HTMLDivElement>('#rename-box');
  const renameInput = page.querySelector<HTMLInputElement>('#rename-input');
  page.querySelector('#scenario-rename')?.addEventListener('click', () => {
    if (renameBox) {
      renameBox.style.display = 'flex';
      renameInput?.focus();
    }
  });

  page.querySelector('#rename-cancel')?.addEventListener('click', () => {
    if (renameBox) renameBox.style.display = 'none';
  });

  page.querySelector('#rename-confirm')?.addEventListener('click', () => {
    const val = renameInput?.value.trim();
    if (!val) {
      toast('场景名称不能为空', true);
      return;
    }
    readSettingsPage();
    const current = state.draft.scenarios.find(s => s.id === state.draft.activeScenarioId);
    if (current) {
      current.name = val;
      renderStylePage(page);
      toast('场景名称已更新');
    }
  });

  // 删除当前场景
  page.querySelector('#scenario-delete')?.addEventListener('click', () => {
    readSettingsPage();
    if (state.draft.scenarios.length <= 1) return;
    const idx = state.draft.scenarios.findIndex(s => s.id === state.draft.activeScenarioId);
    if (idx !== -1) {
      state.draft.scenarios.splice(idx, 1);
      state.draft.activeScenarioId = state.draft.scenarios[0].id;
      renderStylePage(page);
    }
  });

  // 添加项目经历
  page.querySelector('#project-add')?.addEventListener('click', () => {
    readSettingsPage();
    if (!currentScenario.profile.projects) {
      currentScenario.profile.projects = [];
    }
    currentScenario.profile.projects.push({ name: '', role: '', achievements: '' });
    renderStylePage(page);
  });

  // 删除单条项目经历
  page.querySelectorAll<HTMLButtonElement>('[data-project-del]').forEach(btn => {
    btn.addEventListener('click', () => {
      readSettingsPage();
      const idx = Number(btn.dataset.projectDel);
      if (currentScenario.profile.projects && idx >= 0 && idx < currentScenario.profile.projects.length) {
        currentScenario.profile.projects.splice(idx, 1);
        renderStylePage(page);
      }
    });
  });
}
