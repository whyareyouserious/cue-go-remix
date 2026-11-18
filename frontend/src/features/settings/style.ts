import { state } from '../../app/state';
import { esc } from '../../utils/format';

export function renderStylePage(page: HTMLElement) {
  page.innerHTML = `<h3>用适合你的方式回答</h3><div class="field"><label for="answer-language">回答 / 转写语言</label><select id="answer-language" data-setting="language">${[['zh', '中文'], ['en', 'English'], ['auto', '自动'], ['ja', '日本語'], ['ko', '한국어'], ['es', 'Español']].map(([v, label]) => `<option value="${v}" ${state.draft.language === v ? 'selected' : ''}>${label}</option>`).join('')}</select></div><div class="field"><label for="ai-rules">回答风格 <span>最多 2,000 字符</span></label><textarea id="ai-rules" data-setting="aiRules" rows="5" maxlength="2000" placeholder="例如：简洁自然，先给结论；使用短句，必要时列出要点。">${esc(state.draft.aiRules)}</textarea><p class="field-hint">应用于日常回答；屏幕解题保持独立格式。</p></div><div class="field"><label for="background">个人背景 <span>可选</span></label><textarea id="background" data-setting="background" rows="5" maxlength="30000" placeholder="你的工作、项目经验或其他希望 Cue 了解的背景…">${esc(state.draft.background)}</textarea><p class="field-hint">随相关问题发送给所选模型。这里只写你愿意提供的信息。</p></div>`;
}
