document.addEventListener('DOMContentLoaded', () => {
  const configStorageKey = 'note-maker-config-v1';
  const defaultQuestions = [
    { id: 'theme', text: '記事の中心テーマは何ですか？', flow_type: 'main', target_field: 'theme' },
    { id: 'opening_episode', text: '記事の導入に置く具体的な体験や場面は何ですか？', flow_type: 'main', target_field: 'opening_episode' },
    { id: 'reader', text: 'この記事を届けたい読者は誰ですか？', flow_type: 'main', target_field: 'reader' },
    { id: 'expected_reader_action', text: '読後に読者へどんな変化や行動を起こしてほしいですか？', flow_type: 'main', target_field: 'expected_reader_action' },
    { id: 'must_include', text: '記事に必ず含める論点、事実、手順は何ですか？', flow_type: 'main', target_field: 'must_include' },
    { id: 'personal_context', text: '著者本人の経験、肩書き、失敗、価値観など、記事に入れるべき属人的な文脈は何ですか？', flow_type: 'main', target_field: 'personal_context' },
    { id: 'exclusions', text: '記事に含めないこと、避けたい表現、断言しないことは何ですか？', flow_type: 'main', target_field: 'exclusions' },
    { id: 'target_length_structure', text: '目標文字数と記事構成を指定してください。例: 3000字前後、導入・背景・実装・検証・提案・結論。', flow_type: 'main', target_field: 'target_length_structure' },
    { id: 'tone_stance', text: '記事のトーンや立場はどうしますか？内省、技術解説、実用、物語性の比重も指定してください。', flow_type: 'main', target_field: 'tone_stance' },
  ];

  const config = loadConfig();
  const state = {
    profileId: '',
    sessionId: '',
    nextQuestion: null,
    completedBrief: null,
    personas: [],
    formats: [],
    answerAbortController: null,
    draftAbortController: null,
  };

  const el = {
    modelStatus: document.getElementById('model-status'),
    personaSelect: document.getElementById('persona-select'),
    formatSelect: document.getElementById('format-select'),
    modeSummary: document.getElementById('mode-summary'),
    styleModel: document.getElementById('style-model'),
    briefModel: document.getElementById('brief-model'),
    draftModel: document.getElementById('draft-model'),
    verifyModel: document.getElementById('verify-model'),
    questionConfigList: document.getElementById('question-config-list'),
    addQuestion: document.getElementById('add-question-btn'),
    resetQuestions: document.getElementById('reset-questions-btn'),
    username: document.getElementById('style-username'),
    limit: document.getElementById('style-limit'),
    analyzeStyle: document.getElementById('analyze-style-btn'),
    usePresetStyle: document.getElementById('use-preset-style-btn'),
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
    cancelAnswer: document.getElementById('cancel-answer-btn'),
    skipDeepDive: document.getElementById('skip-deep-dive-btn'),
    briefResult: document.getElementById('brief-result'),
    briefPreview: document.getElementById('brief-preview'),
    generateDraft: document.getElementById('generate-draft-btn'),
    cancelDraft: document.getElementById('cancel-draft-btn'),
    draftStatus: document.getElementById('draft-status'),
    draftResult: document.getElementById('draft-result'),
    evaluationSummary: document.getElementById('evaluation-summary'),
    verificationSummary: document.getElementById('verification-summary'),
    previewContent: document.getElementById('preview-content'),
    markdownOutput: document.getElementById('markdown-output'),
    copy: document.getElementById('copy-btn'),
    loading: document.getElementById('loading'),
    loadingText: document.getElementById('loading-text'),
    errorArea: document.getElementById('error-message-area'),
  };

  renderQuestionConfig();
  initializeModeControls();
  checkModels();

  el.personaSelect.addEventListener('change', onPersonaChange);
  el.formatSelect.addEventListener('change', onFormatChange);
  el.styleModel.addEventListener('change', saveModelConfig);
  el.briefModel.addEventListener('change', saveModelConfig);
  el.draftModel.addEventListener('change', saveModelConfig);
  el.verifyModel.addEventListener('change', saveModelConfig);
  el.addQuestion.addEventListener('click', addQuestion);
  el.resetQuestions.addEventListener('click', resetQuestions);
  el.analyzeStyle.addEventListener('click', analyzeStyle);
  el.usePresetStyle.addEventListener('click', usePresetStyle);
  el.startInterview.addEventListener('click', startInterview);
  el.submitAnswer.addEventListener('click', submitAnswer);
  el.cancelAnswer.addEventListener('click', () => state.answerAbortController?.abort());
  el.skipDeepDive.addEventListener('click', skipDeepDive);
  el.generateDraft.addEventListener('click', generateDraft);
  el.cancelDraft.addEventListener('click', () => state.draftAbortController?.abort());
  el.copy.addEventListener('click', copyMarkdown);

  document.querySelectorAll('.tab-btn').forEach((button) => {
    button.addEventListener('click', () => setActiveTab(button.dataset.tab));
  });

  async function initializeModeControls() {
    try {
      const [personas, formats] = await Promise.all([
        requestJSON('/api/personas'),
        requestJSON('/api/formats'),
      ]);
      state.personas = personas;
      state.formats = formats;
      populatePersonaSelect();
      populateFormatSelect();
      applyPersonaDefaults(false);
      renderModeSummary();
    } catch (error) {
      showError(`書き分け設定の取得に失敗しました: ${error.message}`);
    }
  }

  async function checkModels() {
    try {
      const models = await requestJSON('/api/models');
      populateModelSelects(models);
      el.modelStatus.textContent = models.includes('gemma4:31b')
        ? 'model: gemma4:31b'
        : `model: ${models[0] || 'none'}`;
    } catch (_) {
      populateModelSelects([]);
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
          style_model: el.styleModel.value,
        },
      });
      applyStyleResult(data);
      el.startInterview.focus();
    } catch (error) {
      showError(`文体分析に失敗しました: ${error.message}`);
    } finally {
      setLoading(false);
    }
  }

  async function usePresetStyle() {
    clearError();
    setLoading(true, '選択中の書き手プリセットを準備しています...');
    try {
      const data = await requestJSON('/api/author-style/seed', {
        method: 'POST',
        body: {
          persona_id: currentPersonaId(),
        },
      });
      applyStyleResult(data);
      el.startInterview.focus();
    } catch (error) {
      showError(`プリセット文体の準備に失敗しました: ${error.message}`);
    } finally {
      setLoading(false);
    }
  }

  function applyStyleResult(data) {
    state.profileId = data.profile_id;
    el.profileId.textContent = data.profile_id;
    el.guideId.textContent = data.guide_id;
    el.articleCount.textContent = String(data.article_count);
    el.guidePreview.textContent = data.guide_markdown;
    el.styleResult.classList.remove('hidden');
    el.startInterview.disabled = false;
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
        body: {
          style_profile_id: state.profileId,
          persona_id: currentPersonaId(),
          output_format_id: currentFormatId(),
          brief_model: el.briefModel.value,
          questions: currentQuestions(),
        },
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
    if (payload.skip_deep_dive) {
      setLoading(true, '回答を保存し、次の質問を準備しています...');
      try {
        const data = await requestJSON(`/api/brief-sessions/${state.sessionId}/answers`, {
          method: 'POST',
          body: { ...payload, brief_model: el.briefModel.value },
        });
        applyInterviewResult(data);
      } catch (error) {
        showError(`回答の保存に失敗しました: ${error.message}`);
      } finally {
        setLoading(false);
      }
      return;
    }

    let streamedQuestion = '';
    let streamedQuestionItem = null;
    state.answerAbortController = new AbortController();
    setAnswerStreaming(true);
    appendLog('system', '回答を保存しています。');
    try {
      await requestSSE(`/api/brief-sessions/${state.sessionId}/answers`, {
        method: 'POST',
        body: { ...payload, brief_model: el.briefModel.value },
        signal: state.answerAbortController.signal,
        onEvent(event, data) {
          if (event === 'status') {
            if (data.status === 'follow_up_generation_started') {
              appendLog('system', '深掘り質問を生成しています。');
            }
            return;
          }
          if (event === 'chunk') {
            streamedQuestion += data.text || '';
            if (!streamedQuestionItem) {
              streamedQuestionItem = appendLog('deep-dive', '');
            }
            streamedQuestionItem.textContent = streamedQuestion;
            el.questionLog.scrollTop = el.questionLog.scrollHeight;
            return;
          }
          if (event === 'result') {
            applyInterviewResult(data, { questionAlreadyRendered: Boolean(streamedQuestionItem) });
            if (streamedQuestionItem && data.next_question?.text) {
              streamedQuestionItem.textContent = data.next_question.text;
            }
            return;
          }
          if (event === 'error') {
            throw new Error(data.message || data.detail || 'stream error');
          }
        },
      });
    } catch (error) {
      if (error.name === 'AbortError') {
        appendLog('system', '処理を停止しました。途中までの内容は画面に残しています。');
      } else {
        showError(`回答の保存に失敗しました: ${error.message}`);
      }
    } finally {
      setAnswerStreaming(false);
      state.answerAbortController = null;
    }
  }

  function applyInterviewResult(data, options = {}) {
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
    if (!options.questionAlreadyRendered) {
      renderQuestion(data.next_question);
    }
    el.skipDeepDive.classList.toggle('hidden', data.next_question?.flow_type !== 'deep_dive_follow_up');
  }

  async function generateDraft() {
    clearError();
    if (!state.profileId || !state.sessionId) {
      showError('文体分析と取材を完了してください');
      return;
    }
    state.draftAbortController = new AbortController();
    setDraftStreaming(true);
    el.draftStatus.textContent = 'Evo X2 の OpenAI互換APIで下書きを生成しています。';
    let draftBuffer = '';
    el.markdownOutput.value = '';
    el.previewContent.innerHTML = '';
    el.evaluationSummary.textContent = '';
    el.verificationSummary.textContent = '';
    el.draftResult.classList.remove('hidden');
    setActiveTab('markdown');
    try {
      await requestSSE('/api/drafts', {
        method: 'POST',
        body: {
          style_profile_id: state.profileId,
          session_id: state.sessionId,
          persona_id: currentPersonaId(),
          output_format_id: currentFormatId(),
          draft_model: el.draftModel.value,
          verify_model: el.verifyModel.value,
        },
        signal: state.draftAbortController.signal,
        onEvent(event, data) {
          if (event === 'status') {
            el.draftStatus.textContent = draftStatusText(data.status, data.elapsed_ms);
            return;
          }
          if (event === 'heartbeat') {
            el.draftStatus.textContent = draftStatusText('running', data.elapsed_ms);
            return;
          }
          if (event === 'chunk') {
            draftBuffer += data.text || '';
            el.markdownOutput.value = draftBuffer;
            el.previewContent.innerHTML = marked.parse(draftBuffer);
            return;
          }
          if (event === 'result') {
            renderDraft(data);
            return;
          }
          if (event === 'error') {
            throw new Error(data.message || data.detail || 'stream error');
          }
          if (event === 'done') {
            el.draftStatus.textContent = draftDoneText(data);
          }
        },
      });
    } catch (error) {
      if (error.name === 'AbortError') {
        el.draftStatus.textContent = '停止しました。途中まで生成されたMarkdownは残しています。';
      } else {
        showError(`下書き生成に失敗しました: ${error.message}`);
      }
    } finally {
      setDraftStreaming(false);
      state.draftAbortController = null;
    }
  }

  function renderQuestion(question) {
    if (!question) {
      return;
    }
    appendLog(question.flow_type === 'deep_dive_follow_up' ? 'deep-dive' : 'question', question.text);
    el.skipDeepDive.classList.toggle('hidden', question.flow_type !== 'deep_dive_follow_up');
  }

  function populateModelSelects(models) {
    const available = models.length ? models : ['gemma4:31b'];
    const defaults = {
      style: config.models.style || 'gemma4:latest',
      brief: config.models.brief || 'gemma4:e2b',
      draft: config.models.draft || 'gemma4:31b',
      verify: config.models.verify || 'gemma4:latest',
    };
    setOptions(el.styleModel, available, defaults.style);
    setOptions(el.briefModel, available, defaults.brief);
    setOptions(el.draftModel, available, defaults.draft);
    setOptions(el.verifyModel, available, defaults.verify);
    saveModelConfig();
  }

  function populatePersonaSelect() {
    el.personaSelect.innerHTML = '';
    state.personas.forEach((persona) => {
      const option = document.createElement('option');
      option.value = persona.id;
      option.textContent = persona.display_name;
      option.selected = persona.id === config.mode.persona;
      el.personaSelect.appendChild(option);
    });
    if (!el.personaSelect.value && state.personas[0]) {
      el.personaSelect.value = state.personas[0].id;
    }
  }

  function populateFormatSelect() {
    el.formatSelect.innerHTML = '';
    state.formats.forEach((format) => {
      const option = document.createElement('option');
      option.value = format.id;
      option.textContent = format.display_name;
      option.selected = format.id === config.mode.format;
      el.formatSelect.appendChild(option);
    });
  }

  function onPersonaChange() {
    config.mode.persona = currentPersonaId();
    applyPersonaDefaults(true);
    saveConfig();
    renderModeSummary();
  }

  function onFormatChange() {
    config.mode.format = currentFormatId();
    saveConfig();
    renderModeSummary();
    renderQuestionConfig();
  }

  function applyPersonaDefaults(forceFormat) {
    const persona = currentPersona();
    if (!persona) {
      return;
    }
    const noteSource = (persona.sources || []).find((source) => source.kind === 'note' && source.ref);
    if (noteSource && el.username.value.trim() === 'cor_instrument') {
      el.username.value = noteSource.ref;
    }
    if ((forceFormat || !el.formatSelect.value) && persona.default_format) {
      el.formatSelect.value = persona.default_format;
      config.mode.format = persona.default_format;
    }
    renderQuestionConfig();
  }

  function renderModeSummary() {
    const persona = currentPersona();
    const format = currentFormat();
    if (!persona || !format) {
      el.modeSummary.textContent = '';
      return;
    }
    const firstPerson = persona.voice_notes?.first_person?.join(' / ') || '';
    const sources = (persona.sources || []).map((source) => source.kind).join(' + ');
    el.modeSummary.innerHTML = `
      <strong>${escapeHTML(persona.display_name)} × ${escapeHTML(format.display_name)}</strong>
      <span>${escapeHTML(persona.description || '')}</span>
      <span>一人称: ${escapeHTML(firstPerson || '文体ガイド優先')} / source: ${escapeHTML(sources || 'manual')}</span>
    `;
  }

  function setOptions(select, models, selected) {
    const values = models.includes(selected) ? models : [selected, ...models];
    select.innerHTML = '';
    values.forEach((model) => {
      const option = document.createElement('option');
      option.value = model;
      option.textContent = model;
      option.selected = model === selected;
      select.appendChild(option);
    });
  }

  function saveModelConfig() {
    config.models = {
      style: el.styleModel.value,
      brief: el.briefModel.value,
      draft: el.draftModel.value,
      verify: el.verifyModel.value,
    };
    saveConfig();
  }

  function renderQuestionConfig() {
    el.questionConfigList.innerHTML = '';
    config.questions.forEach((question, index) => {
      const row = document.createElement('div');
      row.className = 'question-config-row';

      const input = document.createElement('input');
      input.type = 'text';
      input.value = question.text;
      input.addEventListener('input', () => {
        config.questions[index].text = input.value;
        saveConfig();
      });

      const remove = document.createElement('button');
      remove.type = 'button';
      remove.className = 'secondary-btn';
      remove.textContent = '削除';
      remove.disabled = isFixedQuestion(question.id);
      remove.addEventListener('click', () => {
        config.questions.splice(index, 1);
        saveConfig();
        renderQuestionConfig();
      });

      row.append(input, remove);
      el.questionConfigList.appendChild(row);
    });
  }

  function addQuestion() {
    config.questions.push({
      id: `custom_${Date.now()}`,
      text: '追加で聞きたい質問を入力してください',
      flow_type: 'main',
      target_field: 'custom',
    });
    saveConfig();
    renderQuestionConfig();
  }

  function resetQuestions() {
    config.questions = cloneQuestions(defaultQuestions);
    saveConfig();
    renderQuestionConfig();
  }

  function currentQuestions() {
    return [...config.questions, ...formatQuestions(currentFormatId())]
      .map((question) => ({
        id: question.id,
        text: question.text.trim(),
        flow_type: question.flow_type || 'main',
        target_field: question.target_field || 'custom',
      }))
      .filter((question) => question.id && question.text);
  }

  function formatQuestions(formatId) {
    if (formatId === 'markdown_blog') {
      return [
        { id: 'cor_blog_purpose', text: '会社ブログとしての主目的は何ですか？例: 技術知見の報告、実装判断の共有、社員へのビジョン共有。', flow_type: 'main', target_field: 'custom' },
        { id: 'cor_blog_category', text: 'カテゴリは ai / engineering / founder / lab のどれにしますか？理由も教えてください。', flow_type: 'main', target_field: 'custom' },
        { id: 'cor_blog_metadata', text: 'slug候補、タグ3-5個、featuredの有無、画像パスがあれば指定してください。', flow_type: 'main', target_field: 'custom' },
        { id: 'cor_blog_evidence', text: '本文に入れる具体的な実装、検証結果、数値、意思決定の根拠は何ですか？', flow_type: 'main', target_field: 'custom' },
        { id: 'cor_blog_next_action', text: '社員や読者に、この記事を読んだ後どんな判断や行動をしてほしいですか？', flow_type: 'main', target_field: 'custom' },
      ];
    }
    if (formatId === 'zenn_article' || formatId === 'qiita_article') {
      return [
        { id: 'target_stack', text: '対象技術スタック、バージョン、実行環境は何ですか？', flow_type: 'main', target_field: 'custom' },
        { id: 'prerequisite_knowledge', text: '読者に前提として求める知識と、説明を厚くする箇所はどこですか？', flow_type: 'main', target_field: 'custom' },
        { id: 'code_examples', text: '必ず入れるコード例、コマンド、設定ファイルは何ですか？', flow_type: 'main', target_field: 'custom' },
        { id: 'references', text: '参照すべき公式ドキュメント、記事、リポジトリURLはありますか？', flow_type: 'main', target_field: 'custom' },
      ];
    }
    if (formatId === 'homepage_section') {
      return [
        { id: 'target_conversion', text: 'このHTMLセクションで読者に起こしてほしい行動は何ですか？', flow_type: 'main', target_field: 'custom' },
        { id: 'primary_cta', text: 'CTAの文言とリンク先は何にしますか？', flow_type: 'main', target_field: 'custom' },
        { id: 'brand_voice', text: '会社サイトとして守りたい言い回し、避けたい表現はありますか？', flow_type: 'main', target_field: 'custom' },
      ];
    }
    return [];
  }

  function currentPersonaId() {
    return el.personaSelect.value || config.mode.persona || 'terisuke';
  }

  function currentFormatId() {
    return el.formatSelect.value || config.mode.format || 'note_article';
  }

  function currentPersona() {
    return state.personas.find((persona) => persona.id === currentPersonaId());
  }

  function currentFormat() {
    return state.formats.find((format) => format.id === currentFormatId());
  }

  function isFixedQuestion(id) {
    return defaultQuestions.some((question) => question.id === id);
  }

  function loadConfig() {
    const fallback = {
      mode: { persona: 'terisuke', format: 'note_article' },
      models: { style: 'gemma4:e2b', brief: 'qwen3.6:27b', draft: 'gemma4:31b', verify: 'gemma4:latest' },
      questions: cloneQuestions(defaultQuestions),
    };
    try {
      const saved = JSON.parse(localStorage.getItem(configStorageKey) || '{}');
      return {
        models: { ...fallback.models, ...(saved.models || {}) },
        mode: { ...fallback.mode, ...(saved.mode || {}) },
        questions: Array.isArray(saved.questions) && saved.questions.length
          ? saved.questions
          : fallback.questions,
      };
    } catch (_) {
      return fallback;
    }
  }

  function saveConfig() {
    localStorage.setItem(configStorageKey, JSON.stringify(config));
  }

  function cloneQuestions(questions) {
    return questions.map((question) => ({ ...question }));
  }

  function appendLog(kind, text) {
    const item = document.createElement('div');
    item.className = `log-item ${kind}`;
    item.textContent = text;
    el.questionLog.appendChild(item);
    el.questionLog.scrollTop = el.questionLog.scrollHeight;
    return item;
  }

  function escapeHTML(value) {
    return String(value)
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#039;');
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
    renderVerification(data.verification);
    el.markdownOutput.value = data.draft;
    el.previewContent.innerHTML = marked.parse(data.draft);
    el.draftResult.classList.remove('hidden');
    setActiveTab('preview');
  }

  function renderVerification(verification) {
    if (!verification || !(verification.performed ?? verification.Performed)) {
      el.verificationSummary.className = 'evaluation';
      el.verificationSummary.textContent = 'final verification: not run';
      return;
    }
    const passed = verification.passed ?? verification.Passed;
    const summary = verification.summary ?? verification.Summary ?? '';
    const report = verification.report ?? verification.Report ?? '';
    const failures = verification.failures ?? verification.Failures ?? [];
    el.verificationSummary.className = `evaluation ${passed ? 'passed' : 'failed'}`;
    el.verificationSummary.innerHTML = `
      <strong>VERIFY ${passed ? 'PASS' : 'NEEDS REVIEW'}</strong>
      ${summary ? `<span>${escapeHTML(summary)}</span>` : ''}
      ${failures.length ? `<p>${failures.map(escapeHTML).join('<br>')}</p>` : ''}
      ${report ? `<pre>${escapeHTML(report)}</pre>` : ''}
    `;
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

  async function requestSSE(url, options = {}) {
    const response = await fetch(url, {
      method: options.method || 'GET',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
      },
      body: options.body ? JSON.stringify(options.body) : undefined,
      signal: options.signal,
    });
    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      throw new Error(errorData?.error?.message || `HTTP ${response.status}`);
    }
    if (!response.body) {
      throw new Error('stream response body is unavailable');
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    while (true) {
      const { value, done } = await reader.read();
      if (done) {
        break;
      }
      buffer += decoder.decode(value, { stream: true });
      const parts = buffer.split('\n\n');
      buffer = parts.pop() || '';
      for (const part of parts) {
        dispatchSSEBlock(part, options.onEvent);
      }
    }
    buffer += decoder.decode();
    if (buffer.trim()) {
      dispatchSSEBlock(buffer, options.onEvent);
    }
  }

  function dispatchSSEBlock(block, onEvent) {
    const lines = block.split('\n');
    let event = 'message';
    const dataLines = [];
    lines.forEach((line) => {
      if (line.startsWith('event:')) {
        event = line.slice(6).trim();
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice(5).trimStart());
      }
    });
    if (!dataLines.length || !onEvent) {
      return;
    }
    const data = JSON.parse(dataLines.join('\n'));
    onEvent(event, data);
  }

  function draftStatusText(status, elapsedMS = 0) {
    const seconds = Math.max(0, Math.round(Number(elapsedMS || 0) / 1000));
    const labels = {
      stream_opened: '接続しました',
      draft_generation_started: '本文を生成しています',
      draft_validation_started: 'Markdownと文体を検証しています',
      style_revision_started: '文体スコアを上げるために一度だけ修正しています',
      draft_lightweight_verification_started: '軽量モデルで最終検証しています',
      runtime_connected: '推論エンドポイントに接続しました',
      running: '生成を継続しています',
      completed: '生成が完了しました',
    };
    return `${labels[status] || status} (${seconds}s)`;
  }

  function draftDoneText(data) {
    const seconds = Math.max(0, Math.round(Number(data.elapsed_ms || 0) / 1000));
    const score = Number(data.score || 0).toFixed(1);
    return `生成が完了しました (${seconds}s / ${data.runes || 0}字 / score ${score})`;
  }

  function setAnswerStreaming(active) {
    el.submitAnswer.disabled = active;
    el.skipDeepDive.disabled = active;
    el.cancelAnswer.classList.toggle('hidden', !active);
  }

  function setDraftStreaming(active) {
    el.generateDraft.disabled = active || !state.completedBrief;
    el.cancelDraft.classList.toggle('hidden', !active);
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
