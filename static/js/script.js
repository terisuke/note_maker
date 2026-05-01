document.addEventListener('DOMContentLoaded', () => {
  const state = {
    profileId: '',
    sessionId: '',
    nextQuestion: null,
    completedBrief: null,
  };

  const el = {
    modelStatus: document.getElementById('model-status'),
    username: document.getElementById('style-username'),
    limit: document.getElementById('style-limit'),
    analyzeStyle: document.getElementById('analyze-style-btn'),
    styleResult: document.getElementById('style-result'),
    profileId: document.getElementById('profile-id'),
    guideId: document.getElementById('guide-id'),
    articleCount: document.getElementById('article-count'),
    guidePreview: document.getElementById('guide-preview'),
    startInterview: document.getElementById('start-interview-btn'),
    interviewArea: document.getElementById('interview-area'),
    questionLog: document.getElementById('question-log'),
    answerInput: document.getElementById('answer-input'),
    submitAnswer: document.getElementById('submit-answer-btn'),
    skipDeepDive: document.getElementById('skip-deep-dive-btn'),
    briefResult: document.getElementById('brief-result'),
    briefPreview: document.getElementById('brief-preview'),
    generateDraft: document.getElementById('generate-draft-btn'),
    draftStatus: document.getElementById('draft-status'),
    draftResult: document.getElementById('draft-result'),
    evaluationSummary: document.getElementById('evaluation-summary'),
    previewContent: document.getElementById('preview-content'),
    markdownOutput: document.getElementById('markdown-output'),
    copy: document.getElementById('copy-btn'),
    loading: document.getElementById('loading'),
    loadingText: document.getElementById('loading-text'),
    errorArea: document.getElementById('error-message-area'),
  };

  checkModels();

  el.analyzeStyle.addEventListener('click', analyzeStyle);
  el.startInterview.addEventListener('click', startInterview);
  el.submitAnswer.addEventListener('click', submitAnswer);
  el.skipDeepDive.addEventListener('click', skipDeepDive);
  el.generateDraft.addEventListener('click', generateDraft);
  el.copy.addEventListener('click', copyMarkdown);

  document.querySelectorAll('.tab-btn').forEach((button) => {
    button.addEventListener('click', () => setActiveTab(button.dataset.tab));
  });

  async function checkModels() {
    try {
      const models = await requestJSON('/api/models');
      el.modelStatus.textContent = models.includes('gemma4:31b')
        ? 'model: gemma4:31b'
        : `model: ${models[0] || 'none'}`;
    } catch (_) {
      el.modelStatus.textContent = 'model: unavailable';
    }
  }

  async function analyzeStyle() {
    clearError();
    const username = el.username.value.trim();
    if (!username) {
      showError('Noteユーザー名を入力してください');
      return;
    }
    setLoading(true, 'Note記事を取得し、文体を分析しています...');
    try {
      const data = await requestJSON('/api/author-style/analyze', {
        method: 'POST',
        body: {
          username,
          limit: Number(el.limit.value),
        },
      });
      state.profileId = data.profile_id;
      el.profileId.textContent = data.profile_id;
      el.guideId.textContent = data.guide_id;
      el.articleCount.textContent = String(data.article_count);
      el.guidePreview.textContent = data.guide_markdown;
      el.styleResult.classList.remove('hidden');
      el.startInterview.disabled = false;
      el.startInterview.focus();
    } catch (error) {
      showError(`文体分析に失敗しました: ${error.message}`);
    } finally {
      setLoading(false);
    }
  }

  async function startInterview() {
    clearError();
    if (!state.profileId) {
      showError('先に文体分析を完了してください');
      return;
    }
    setLoading(true, '取材セッションを開始しています...');
    try {
      const data = await requestJSON('/api/brief-sessions', {
        method: 'POST',
        body: { style_profile_id: state.profileId },
      });
      state.sessionId = data.session_id;
      state.nextQuestion = data.next_question;
      el.interviewArea.classList.remove('hidden');
      el.questionLog.innerHTML = '';
      renderQuestion(data.next_question);
    } catch (error) {
      showError(`取材開始に失敗しました: ${error.message}`);
    } finally {
      setLoading(false);
    }
  }

  async function submitAnswer() {
    clearError();
    const content = el.answerInput.value.trim();
    if (!content) {
      showError('回答を入力してください');
      return;
    }
    appendLog('answer', content);
    el.answerInput.value = '';
    await sendAnswer({ content });
  }

  async function skipDeepDive() {
    clearError();
    await sendAnswer({ skip_deep_dive: true });
  }

  async function sendAnswer(payload) {
    setLoading(true, '回答を保存し、次の質問を準備しています...');
    try {
      const data = await requestJSON(`/api/brief-sessions/${state.sessionId}/answers`, {
        method: 'POST',
        body: payload,
      });
      if (data.completed) {
        state.completedBrief = data.brief;
        state.nextQuestion = null;
        el.briefPreview.textContent = JSON.stringify(data.brief, null, 2);
        el.briefResult.classList.remove('hidden');
        el.generateDraft.disabled = false;
        appendLog('system', '記事ブリーフが完成しました。');
        return;
      }
      state.nextQuestion = data.next_question;
      renderQuestion(data.next_question);
    } catch (error) {
      showError(`回答の保存に失敗しました: ${error.message}`);
    } finally {
      setLoading(false);
    }
  }

  async function generateDraft() {
    clearError();
    if (!state.profileId || !state.sessionId) {
      showError('文体分析と取材を完了してください');
      return;
    }
    setLoading(true, 'ローカルLLMで下書きを生成しています...');
    el.draftStatus.textContent = 'Gemma4 31B の処理には数分かかる場合があります。';
    try {
      const data = await requestJSON('/api/drafts', {
        method: 'POST',
        body: {
          style_profile_id: state.profileId,
          session_id: state.sessionId,
        },
      });
      renderDraft(data);
    } catch (error) {
      showError(`下書き生成に失敗しました: ${error.message}`);
    } finally {
      setLoading(false);
    }
  }

  function renderQuestion(question) {
    if (!question) {
      return;
    }
    appendLog(question.flow_type === 'deep_dive_follow_up' ? 'deep-dive' : 'question', question.text);
    el.skipDeepDive.classList.toggle('hidden', question.flow_type !== 'deep_dive_follow_up');
  }

  function appendLog(kind, text) {
    const item = document.createElement('div');
    item.className = `log-item ${kind}`;
    item.textContent = text;
    el.questionLog.appendChild(item);
    el.questionLog.scrollTop = el.questionLog.scrollHeight;
  }

  function renderDraft(data) {
    const evaluation = data.evaluation;
    const passed = evaluation?.Passed ?? evaluation?.passed;
    const comparison = evaluation?.Comparison ?? evaluation?.comparison;
    const failures = evaluation?.Failures ?? evaluation?.failures ?? [];
    const score = comparison?.score ?? comparison?.Score ?? 0;

    el.evaluationSummary.className = `evaluation ${passed ? 'passed' : 'failed'}`;
    el.evaluationSummary.innerHTML = `
      <strong>${passed ? 'PASS' : 'NEEDS REVIEW'}</strong>
      <span>style score: ${Number(score).toFixed(1)}</span>
      ${failures.length ? `<p>${failures.join('<br>')}</p>` : ''}
    `;
    el.markdownOutput.value = data.draft;
    el.previewContent.innerHTML = marked.parse(data.draft);
    el.draftResult.classList.remove('hidden');
    setActiveTab('preview');
  }

  async function requestJSON(url, options = {}) {
    const response = await fetch(url, {
      method: options.method || 'GET',
      headers: { 'Content-Type': 'application/json' },
      body: options.body ? JSON.stringify(options.body) : undefined,
    });
    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      throw new Error(errorData?.error?.message || `HTTP ${response.status}`);
    }
    return response.json();
  }

  function setActiveTab(tabId) {
    document.querySelectorAll('.tab-btn').forEach((button) => {
      button.classList.toggle('active', button.dataset.tab === tabId);
    });
    document.querySelectorAll('.tab-pane').forEach((pane) => {
      pane.classList.toggle('active', pane.id === `${tabId}-tab`);
    });
  }

  function copyMarkdown() {
    navigator.clipboard.writeText(el.markdownOutput.value).then(() => {
      const original = el.copy.textContent;
      el.copy.textContent = 'コピーしました';
      setTimeout(() => {
        el.copy.textContent = original;
      }, 1600);
    });
  }

  function setLoading(active, text = '') {
    el.loading.classList.toggle('hidden', !active);
    if (text) {
      el.loadingText.textContent = text;
    }
  }

  function showError(message) {
    const error = document.createElement('div');
    error.className = 'error-message';
    error.textContent = message;
    el.errorArea.appendChild(error);
  }

  function clearError() {
    el.errorArea.innerHTML = '';
  }
});
