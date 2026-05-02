document.addEventListener('DOMContentLoaded', () => {
  const configStorageKey = 'note-maker-config-v1';
  const legacyTemplateQuestionIds = new Set([
    'theme',
    'opening_episode',
    'reader',
    'expected_reader_action',
    'must_include',
    'personal_context',
    'exclusions',
    'target_length_structure',
    'tone_stance',
    'cor_blog_purpose',
    'cor_blog_category',
    'cor_blog_metadata',
    'cor_blog_evidence',
    'cor_blog_next_action',
    'target_stack',
    'prerequisite_knowledge',
    'code_examples',
    'references',
    'target_conversion',
    'primary_cta',
    'brand_voice',
    'story_arc',
    'technical_proof',
    'homepage_cta',
    'homepage_trust',
    'cloudia_viewpoint',
  ]);

  const config = loadConfig();
  const state = {
    profileId: '',
    sessionId: '',
    parentSessionId: '',
    nextQuestion: null,
    answers: [],
    completedBrief: null,
    personas: [],
    formats: [],
    templateQuestions: [],
    templateLoading: false,
    templateError: '',
    templateRequestId: 0,
    storageConfig: null,
    questionTextById: {},
    lastSubmittedAnswer: '',
    answerAbortController: null,
    draftAbortController: null,
    pendingSectionReplacement: null,
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
    storageDriver: document.getElementById('storage-driver'),
    storagePath: document.getElementById('storage-path'),
    saveStorage: document.getElementById('save-storage-btn'),
    storageSummary: document.getElementById('storage-summary'),
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
    copyPreview: document.getElementById('copy-preview-btn'),
    regenerateSection: document.getElementById('regenerate-section-btn'),
    sectionStatus: document.getElementById('section-status'),
    sectionCandidate: document.getElementById('section-candidate'),
    sectionCandidateOutput: document.getElementById('section-candidate-output'),
    acceptSection: document.getElementById('accept-section-btn'),
    rejectSection: document.getElementById('reject-section-btn'),
    loading: document.getElementById('loading'),
    loadingText: document.getElementById('loading-text'),
    errorArea: document.getElementById('error-message-area'),
  };

  renderQuestionConfig();
  initializeModeControls();
  checkModels();
  loadStorageConfig();

  el.personaSelect.addEventListener('change', onPersonaChange);
  el.formatSelect.addEventListener('change', onFormatChange);
  el.styleModel.addEventListener('change', saveModelConfig);
  el.briefModel.addEventListener('change', saveModelConfig);
  el.draftModel.addEventListener('change', saveModelConfig);
  el.verifyModel.addEventListener('change', saveModelConfig);
  el.storageDriver.addEventListener('change', onStorageDriverChange);
  el.saveStorage.addEventListener('click', saveStorageConfig);
  el.addQuestion.addEventListener('click', addQuestion);
  el.resetQuestions.addEventListener('click', resetQuestions);
  el.analyzeStyle.addEventListener('click', analyzeStyle);
  el.usePresetStyle.addEventListener('click', usePresetStyle);
  el.startInterview.addEventListener('click', startInterview);
  el.submitAnswer.addEventListener('click', submitAnswer);
  el.answerInput.addEventListener('keydown', onAnswerInputKeydown);
  el.cancelAnswer.addEventListener('click', () => state.answerAbortController?.abort());
  el.skipDeepDive.addEventListener('click', skipDeepDive);
  el.generateDraft.addEventListener('click', generateDraft);
  el.cancelDraft.addEventListener('click', () => state.draftAbortController?.abort());
  el.copy.addEventListener('click', copyMarkdown);
  el.copyPreview.addEventListener('click', copyPreviewText);
  el.markdownOutput.addEventListener('input', syncDraftEditor);
  el.markdownOutput.addEventListener('keyup', updateSectionControls);
  el.markdownOutput.addEventListener('click', updateSectionControls);
  el.markdownOutput.addEventListener('select', updateSectionControls);
  el.regenerateSection.addEventListener('click', regenerateCurrentSection);
  el.acceptSection.addEventListener('click', acceptSectionCandidate);
  el.rejectSection.addEventListener('click', rejectSectionCandidate);

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
      loadQuestionTemplate();
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
      state.parentSessionId = data.parent_session_id || '';
      state.nextQuestion = data.next_question;
      state.answers = data.answers || [];
      state.completedBrief = null;
      rememberQuestions([...state.templateQuestions, ...currentQuestions()]);
      rememberQuestion(data.next_question);
      el.interviewArea.classList.remove('hidden');
      el.briefResult.classList.add('hidden');
      el.generateDraft.disabled = true;
      renderTranscript(data);
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
    state.lastSubmittedAnswer = content;
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
    try {
      await requestSSE(`/api/brief-sessions/${state.sessionId}/answers`, {
        method: 'POST',
        body: { ...payload, brief_model: el.briefModel.value },
        signal: state.answerAbortController.signal,
        onEvent(event, data) {
          if (event === 'status') {
            return;
          }
          if (event === 'chunk') {
            streamedQuestion += data.text || '';
            streamedQuestionItem = renderPendingQuestion(streamedQuestionItem, streamedQuestion);
            return;
          }
          if (event === 'result') {
            applyInterviewResult(data);
            return;
          }
          if (event === 'error') {
            throw new Error(data.message || data.detail || 'stream error');
          }
        },
      });
    } catch (error) {
      if (error.name === 'AbortError') {
        renderPendingQuestion(streamedQuestionItem, streamedQuestion || '処理を停止しました。途中までの内容は画面に残しています。');
      } else {
        showError(`回答の保存に失敗しました: ${error.message}`);
      }
    } finally {
      setAnswerStreaming(false);
      state.answerAbortController = null;
    }
  }

  function applyInterviewResult(data) {
    state.sessionId = data.session_id || state.sessionId;
    state.parentSessionId = data.parent_session_id || '';
    state.answers = data.answers || [];
    rememberQuestion(data.next_question);
    renderTranscript(data);
    if (data.completed) {
      state.completedBrief = data.brief;
      state.nextQuestion = null;
      el.briefPreview.textContent = JSON.stringify(data.brief, null, 2);
      el.briefResult.classList.remove('hidden');
      el.generateDraft.disabled = false;
      el.skipDeepDive.classList.add('hidden');
      return;
    }
    state.completedBrief = null;
    state.nextQuestion = data.next_question;
    el.briefResult.classList.add('hidden');
    el.generateDraft.disabled = true;
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
    el.sectionStatus.textContent = '';
    rejectSectionCandidate();
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
            syncDraftEditor();
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

  function renderTranscript(data = {}) {
    const answers = data.answers || state.answers || [];
    const nextQuestion = data.next_question ?? state.nextQuestion;
    el.questionLog.innerHTML = '';
    answers.forEach((answer) => {
      el.questionLog.appendChild(createTranscriptItem(answer));
    });
    if (nextQuestion && !data.completed) {
      el.questionLog.appendChild(createPendingQuestionItem(nextQuestion));
    }
    el.questionLog.scrollTop = el.questionLog.scrollHeight;
  }

  function createTranscriptItem(answer) {
    const questionId = answerValue(answer, 'question_id', 'QuestionID');
    const flowType = answerValue(answer, 'flow_type', 'FlowType') || 'main';
    const targetQuestionId = answerValue(answer, 'target_question_id', 'TargetQuestionID');
    const item = document.createElement('div');
    item.className = `transcript-item answered ${flowType === 'deep_dive_follow_up' ? 'deep-dive' : ''}`;

    const questionBubble = document.createElement('div');
    questionBubble.className = 'question-bubble';
    const questionLabel = document.createElement('span');
    questionLabel.className = 'bubble-label';
    questionLabel.textContent = flowType === 'deep_dive_follow_up' ? '深掘り質問' : '質問';
    const questionText = document.createElement('p');
    questionText.textContent = questionTextForAnswer(answer);
    questionBubble.append(questionLabel, questionText);
    const parentContext = parentContextForAnswer(answer);
    if (parentContext) {
      questionBubble.appendChild(parentContext);
    }

    const answerBubble = document.createElement('div');
    answerBubble.className = 'answer-bubble';
    answerBubble.dataset.answerId = questionId;
    if (targetQuestionId) {
      answerBubble.dataset.targetQuestionId = targetQuestionId;
    }
    const answerLabel = document.createElement('span');
    answerLabel.className = 'bubble-label';
    answerLabel.textContent = '回答';
    const answerText = document.createElement('p');
    answerText.className = 'answer-text';
    answerText.textContent = answerValue(answer, 'content', 'Content') || '';
    const editButton = document.createElement('button');
    editButton.type = 'button';
    editButton.className = 'answer-edit-btn';
    editButton.textContent = '編集';
    editButton.addEventListener('click', () => startAnswerEdit(answerBubble, answer));
    answerBubble.append(answerLabel, answerText, editButton);

    item.append(questionBubble, answerBubble);
    return item;
  }

  function createPendingQuestionItem(question) {
    rememberQuestion(question);
    const item = document.createElement('div');
    item.className = `transcript-item pending ${question.flow_type === 'deep_dive_follow_up' ? 'deep-dive' : ''}`;
    const questionBubble = document.createElement('div');
    questionBubble.className = 'question-bubble current';
    const label = document.createElement('span');
    label.className = 'bubble-label';
    label.textContent = question.flow_type === 'deep_dive_follow_up' ? '次の深掘り質問' : '次の質問';
    const text = document.createElement('p');
    text.textContent = question.text || '質問を準備しています...';
    questionBubble.append(label, text);
    if (question.flow_type === 'deep_dive_follow_up') {
      const context = parentContextForQuestion(question);
      if (context) {
        questionBubble.appendChild(context);
      }
    }
    item.appendChild(questionBubble);
    return item;
  }

  function renderPendingQuestion(existingItem, text) {
    if (existingItem) {
      const paragraph = existingItem.querySelector('p');
      if (paragraph) {
        paragraph.textContent = text;
      }
      el.questionLog.scrollTop = el.questionLog.scrollHeight;
      return existingItem;
    }
    const item = createPendingQuestionItem({
      id: 'streaming_follow_up',
      text,
      flow_type: 'deep_dive_follow_up',
    });
    el.questionLog.appendChild(item);
    el.questionLog.scrollTop = el.questionLog.scrollHeight;
    return item;
  }

  function startAnswerEdit(container, answer) {
    const original = answerValue(answer, 'content', 'Content') || '';
    container.innerHTML = '';
    const textarea = document.createElement('textarea');
    textarea.className = 'answer-edit-input';
    textarea.rows = 5;
    textarea.value = original;
    const actions = document.createElement('div');
    actions.className = 'edit-actions';
    const save = document.createElement('button');
    save.type = 'button';
    save.className = 'primary-btn';
    save.textContent = '保存して分岐';
    const cancel = document.createElement('button');
    cancel.type = 'button';
    cancel.className = 'secondary-btn';
    cancel.textContent = 'キャンセル';
    actions.append(save, cancel);
    container.append(textarea, actions);
    textarea.focus();
    textarea.setSelectionRange(textarea.value.length, textarea.value.length);
    save.addEventListener('click', () => editAnswer(answer, textarea.value));
    cancel.addEventListener('click', () => renderTranscript());
  }

  async function editAnswer(answer, content) {
    clearError();
    const trimmed = content.trim();
    if (!trimmed) {
      showError('回答を入力してください');
      return;
    }
    const answerId = answerValue(answer, 'question_id', 'QuestionID');
    setLoading(true, '回答を編集し、新しいセッションへ分岐しています...');
    try {
      const data = await requestJSON(`/api/brief-sessions/${state.sessionId}/answers/${encodeURIComponent(answerId)}/edit`, {
        method: 'POST',
        body: {
          content: trimmed,
          brief_model: el.briefModel.value,
        },
      });
      state.lastSubmittedAnswer = trimmed;
      applyInterviewResult(data);
      el.answerInput.focus();
    } catch (error) {
      showError(`回答編集に失敗しました: ${error.message}`);
      renderTranscript();
    } finally {
      setLoading(false);
    }
  }

  function rememberQuestions(questions) {
    (questions || []).forEach(rememberQuestion);
  }

  function rememberQuestion(question) {
    if (!question?.id || !question?.text) {
      return;
    }
    state.questionTextById[question.id] = question.text;
  }

  function questionTextForAnswer(answer) {
    const questionId = answerValue(answer, 'question_id', 'QuestionID');
    const flowType = answerValue(answer, 'flow_type', 'FlowType') || 'main';
    if (state.questionTextById[questionId]) {
      return state.questionTextById[questionId];
    }
    if (flowType === 'deep_dive_follow_up') {
      const targetQuestionId = answerValue(answer, 'target_question_id', 'TargetQuestionID');
      const index = answerValue(answer, 'follow_up_index', 'FollowUpIndex');
      const target = state.questionTextById[targetQuestionId] || targetQuestionId || '回答';
      return `${target} への深掘り ${index || ''}`.trim();
    }
    return questionId || '質問';
  }

  function parentContextForAnswer(answer) {
    const flowType = answerValue(answer, 'flow_type', 'FlowType') || 'main';
    if (flowType !== 'deep_dive_follow_up') {
      return null;
    }
    return parentContextForQuestion({
      target_question_id: answerValue(answer, 'target_question_id', 'TargetQuestionID'),
    });
  }

  function parentContextForQuestion(question) {
    const targetQuestionId = question.target_question_id || question.TargetQuestionID;
    if (!targetQuestionId) {
      return null;
    }
    const parentAnswer = (state.answers || []).find((answer) => answerValue(answer, 'question_id', 'QuestionID') === targetQuestionId);
    if (!parentAnswer) {
      return null;
    }
    const context = document.createElement('blockquote');
    context.className = 'parent-context';
    const parentQuestion = state.questionTextById[targetQuestionId] || targetQuestionId;
    const parentContent = answerValue(parentAnswer, 'content', 'Content') || '';
    context.textContent = `深掘りの根拠 - ${parentQuestion}: ${truncate(parentContent, 140)}`;
    return context;
  }

  function answerValue(answer, snake, pascal) {
    return answer?.[snake] ?? answer?.[pascal] ?? '';
  }

  function truncate(value, maxLength) {
    const text = String(value || '');
    return text.length > maxLength ? `${text.slice(0, maxLength - 1)}...` : text;
  }

  function onAnswerInputKeydown(event) {
    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
      event.preventDefault();
      if (!el.submitAnswer.disabled) {
        submitAnswer();
      }
      return;
    }
    if (event.key !== 'ArrowUp' || el.answerInput.value.trim() || !state.lastSubmittedAnswer) {
      return;
    }
    if (el.answerInput.selectionStart !== 0 || el.answerInput.selectionEnd !== 0) {
      return;
    }
    event.preventDefault();
    el.answerInput.value = state.lastSubmittedAnswer;
    el.answerInput.setSelectionRange(el.answerInput.value.length, el.answerInput.value.length);
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
    loadQuestionTemplate();
  }

  function onFormatChange() {
    config.mode.format = currentFormatId();
    saveConfig();
    renderModeSummary();
    loadQuestionTemplate();
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

  async function loadStorageConfig() {
    try {
      const data = await requestJSON('/api/config/storage');
      state.storageConfig = data;
      applyStorageConfig(data);
    } catch (error) {
      el.storageSummary.className = 'storage-summary warning';
      el.storageSummary.textContent = `保存方式を取得できませんでした: ${error.message}`;
    }
  }

  function applyStorageConfig(data) {
    el.storageDriver.value = data.configured_driver || data.active_driver || 'json';
    el.storagePath.value = data.configured_path || data.active_path || defaultStoragePath(el.storageDriver.value);
    el.storageDriver.disabled = Boolean(data.env_locked);
    el.storagePath.disabled = Boolean(data.env_locked);
    el.saveStorage.disabled = Boolean(data.env_locked);
    renderStorageSummary(data);
  }

  function renderStorageSummary(data) {
    const active = `${storageDriverLabel(data.active_driver)} / ${data.active_path}`;
    const configured = `${storageDriverLabel(data.configured_driver)} / ${data.configured_path}`;
    const status = data.env_locked
      ? '環境変数で固定されています。UIからは変更できません。'
      : data.restart_required
        ? data.restart_message
        : '現在の保存方式で動作中です。';
    el.storageSummary.className = `storage-summary${data.restart_required ? ' warning' : ''}`;
    el.storageSummary.innerHTML = `
      <span>現在: <strong>${escapeHTML(active)}</strong></span>
      <span>次回起動: <strong>${escapeHTML(configured)}</strong></span>
      <span>${escapeHTML(status)}</span>
    `;
  }

  function onStorageDriverChange() {
    const currentPath = el.storagePath.value.trim();
    if (!currentPath || currentPath === defaultStoragePath('json') || currentPath === defaultStoragePath('sqlite')) {
      el.storagePath.value = defaultStoragePath(el.storageDriver.value);
    }
  }

  async function saveStorageConfig() {
    clearError();
    try {
      const data = await requestJSON('/api/config/storage', {
        method: 'PATCH',
        body: {
          workflow_store_driver: el.storageDriver.value,
          workflow_store_path: el.storagePath.value,
        },
      });
      state.storageConfig = data;
      applyStorageConfig(data);
    } catch (error) {
      showError(`保存方式の保存に失敗しました: ${error.message}`);
    }
  }

  function storageDriverLabel(driver) {
    return driver === 'sqlite' ? 'SQLite' : 'JSONファイル';
  }

  function defaultStoragePath(driver) {
    return driver === 'sqlite' ? 'data/workflow_store.db' : 'data/workflow_store.json';
  }

  async function loadQuestionTemplate() {
    const personaId = currentPersonaId();
    const formatId = currentFormatId();
    if (!personaId || !formatId) {
      state.templateQuestions = [];
      state.templateError = '';
      state.templateLoading = false;
      renderQuestionConfig();
      return;
    }

    const requestId = state.templateRequestId + 1;
    state.templateRequestId = requestId;
    state.templateLoading = true;
    state.templateError = '';
    state.templateQuestions = [];
    renderQuestionConfig();

    try {
      const params = new URLSearchParams({ persona_id: personaId, format_id: formatId });
      const data = await requestJSON(`/api/brief-sessions/templates?${params.toString()}`);
      if (requestId !== state.templateRequestId) {
        return;
      }
      state.templateQuestions = normalizeTemplateQuestions(data);
      rememberQuestions(state.templateQuestions);
    } catch (error) {
      if (requestId !== state.templateRequestId) {
        return;
      }
      state.templateQuestions = [];
      state.templateError = `テンプレート質問を取得できませんでした: ${error.message}`;
    } finally {
      if (requestId === state.templateRequestId) {
        state.templateLoading = false;
        renderQuestionConfig();
      }
    }
  }

  function renderQuestionConfig() {
    el.questionConfigList.innerHTML = '';

    if (state.templateLoading) {
      el.questionConfigList.appendChild(createQuestionConfigStatus('テンプレート質問を読み込んでいます...'));
    }

    state.templateQuestions.forEach((question) => {
      el.questionConfigList.appendChild(createTemplateQuestionRow(question));
    });

    if (state.templateError) {
      el.questionConfigList.appendChild(createQuestionConfigStatus(state.templateError, true));
    }

    config.customQuestions.forEach((question, index) => {
      el.questionConfigList.appendChild(createCustomQuestionRow(question, index));
    });

    if (!state.templateLoading && !state.templateQuestions.length && !state.templateError && !config.customQuestions.length) {
      el.questionConfigList.appendChild(createQuestionConfigStatus('テンプレート質問はありません。追加質問を入力できます。'));
    }
  }

  function createTemplateQuestionRow(question) {
    const row = document.createElement('div');
    row.className = 'question-config-row template';

    const input = document.createElement('input');
    input.type = 'text';
    input.value = question.text;
    input.readOnly = true;
    input.setAttribute('aria-label', 'テンプレート質問');

    const label = document.createElement('span');
    label.className = 'question-config-tag';
    label.textContent = 'テンプレート';

    row.append(input, label);
    return row;
  }

  function createCustomQuestionRow(question, index) {
    const row = document.createElement('div');
    row.className = 'question-config-row custom';

    const input = document.createElement('input');
    input.type = 'text';
    input.value = question.text;
    input.setAttribute('aria-label', '追加質問');
    input.addEventListener('input', () => {
      config.customQuestions[index].text = input.value;
      saveConfig();
    });

    const remove = document.createElement('button');
    remove.type = 'button';
    remove.className = 'secondary-btn';
    remove.textContent = '削除';
    remove.addEventListener('click', () => {
      config.customQuestions.splice(index, 1);
      saveConfig();
      renderQuestionConfig();
    });

    row.append(input, remove);
    return row;
  }

  function createQuestionConfigStatus(message, isError = false) {
    const status = document.createElement('div');
    status.className = `question-config-status${isError ? ' error' : ''}`;
    status.textContent = message;
    return status;
  }

  function addQuestion() {
    config.customQuestions.push({
      id: `custom_${Date.now()}`,
      text: '追加で聞きたい質問を入力してください',
      flow_type: 'main',
      target_field: 'custom',
    });
    saveConfig();
    renderQuestionConfig();
  }

  function resetQuestions() {
    config.customQuestions = [];
    saveConfig();
    loadQuestionTemplate();
  }

  function currentQuestions() {
    const templateIds = new Set(state.templateQuestions.map((question) => question.id));
    const templateTexts = new Set(state.templateQuestions.map((question) => question.text.trim()).filter(Boolean));
    return normalizeQuestionList(config.customQuestions)
      .filter((question) => question.id && question.text && !templateIds.has(question.id) && !templateTexts.has(question.text));
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

  function normalizeTemplateQuestions(data) {
    const values = Array.isArray(data)
      ? data
      : data?.questions || data?.template_questions || data?.template?.questions || data?.Template?.Questions || [];
    if (!Array.isArray(values)) {
      return [];
    }
    return normalizeQuestionList(values)
      .map((question) => ({ ...question, template: true }))
      .filter((question) => question.id && question.text);
  }

  function loadConfig() {
    const fallback = {
      mode: { persona: 'terisuke', format: 'note_article' },
      models: { style: 'gemma4:e2b', brief: 'qwen3.6:27b', draft: 'gemma4:31b', verify: 'gemma4:latest' },
      customQuestions: [],
    };
    try {
      const saved = JSON.parse(localStorage.getItem(configStorageKey) || '{}');
      const savedCustomQuestions = saved.customQuestions || saved.custom_questions || migrateLegacyQuestions(saved.questions);
      return {
        models: { ...fallback.models, ...(saved.models || {}) },
        mode: { ...fallback.mode, ...(saved.mode || {}) },
        customQuestions: normalizeQuestionList(savedCustomQuestions),
      };
    } catch (_) {
      return fallback;
    }
  }

  function saveConfig() {
    localStorage.setItem(configStorageKey, JSON.stringify(config));
  }

  function migrateLegacyQuestions(questions) {
    if (!Array.isArray(questions)) {
      return [];
    }
    return questions.filter((question) => question?.id && !legacyTemplateQuestionIds.has(question.id));
  }

  function normalizeQuestionList(questions) {
    if (!Array.isArray(questions)) {
      return [];
    }
    const seen = new Set();
    return questions
      .map((question) => ({
        id: String(question.id || question.ID || '').trim(),
        text: String(question.text || question.Text || '').trim(),
        flow_type: question.flow_type || question.FlowType || 'main',
        target_field: question.target_field || question.TargetField || 'custom',
      }))
      .filter((question) => {
        if (!question.id || !question.text || seen.has(question.id)) {
          return false;
        }
        seen.add(question.id);
        return true;
      });
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
    syncDraftEditor();
    el.draftResult.classList.remove('hidden');
    setActiveTab('preview');
  }

  function syncDraftEditor() {
    el.previewContent.innerHTML = marked.parse(el.markdownOutput.value || '');
    updateSectionControls();
  }

  function updateSectionControls() {
    const section = currentMarkdownSection();
    el.regenerateSection.disabled = !section || !state.profileId || !state.sessionId;
    if (!el.markdownOutput.value.trim()) {
      el.sectionStatus.textContent = '';
      return;
    }
    el.sectionStatus.textContent = section
      ? `選択中のセクション: ## ${section.heading}`
      : '再生成するには Markdown タブで ## 見出し配下にカーソルを置いてください。';
  }

  async function regenerateCurrentSection() {
    clearError();
    const section = currentMarkdownSection();
    if (!section) {
      showError('再生成する ## セクションにカーソルを置いてください');
      return;
    }
    el.regenerateSection.disabled = true;
    el.sectionStatus.textContent = `## ${section.heading} を再生成しています。`;
    rejectSectionCandidate();
    try {
      const data = await requestJSON(`/api/drafts/${encodeURIComponent(state.sessionId)}/regenerate-section`, {
        method: 'POST',
        body: {
          style_profile_id: state.profileId,
          session_id: state.sessionId,
          persona_id: currentPersonaId(),
          output_format_id: currentFormatId(),
          draft_model: el.draftModel.value,
          draft_markdown: el.markdownOutput.value,
          section_anchor: section.anchor,
        },
      });
      state.pendingSectionReplacement = data;
      el.sectionCandidateOutput.value = data.replacement_markdown || '';
      el.sectionCandidate.classList.remove('hidden');
      el.sectionStatus.textContent = `## ${section.heading} の再生成候補を確認してください。`;
      setActiveTab('markdown');
      el.sectionCandidateOutput.focus();
    } catch (error) {
      showError(`セクション再生成に失敗しました: ${error.message}`);
      updateSectionControls();
    } finally {
      el.regenerateSection.disabled = !currentMarkdownSection();
    }
  }

  function acceptSectionCandidate() {
    if (!state.pendingSectionReplacement) {
      return;
    }
    const responseSection = state.pendingSectionReplacement.section || {};
    const section = markdownSections(el.markdownOutput.value).find((candidate) => candidate.anchor === responseSection.anchor)
      || currentMarkdownSection();
    const updated = replaceMarkdownSection(el.markdownOutput.value, section, el.sectionCandidateOutput.value);
    if (!updated) {
      showError('候補の反映に失敗しました。対象セクションを再選択して再生成してください。');
      return;
    }
    el.markdownOutput.value = updated;
    rejectSectionCandidate();
    syncDraftEditor();
    el.markdownOutput.focus();
  }

  function rejectSectionCandidate() {
    state.pendingSectionReplacement = null;
    el.sectionCandidate.classList.add('hidden');
    el.sectionCandidateOutput.value = '';
  }

  function currentMarkdownSection() {
    return sectionAtOffset(el.markdownOutput.value, el.markdownOutput.selectionStart || 0);
  }

  function sectionAtOffset(markdown, offset) {
    const sections = markdownSections(markdown);
    return sections.find((section) => offset >= section.start && offset < section.end)
      || sections.find((section) => offset === section.end)
      || null;
  }

  function markdownSections(markdown) {
    const headings = [];
    let offset = 0;
    const lines = markdown.match(/[^\n]*(?:\n|$)/g) || [];
    lines.forEach((line) => {
      if (!line) {
        return;
      }
      const trimmed = line.replace(/\n$/, '').trim();
      if (trimmed.startsWith('## ') && !trimmed.startsWith('### ')) {
        const heading = trimmed.slice(3).trim();
        headings.push({ offset, heading });
      }
      offset += line.length;
    });
    return headings.map((heading, index) => {
      const end = index + 1 < headings.length ? headings[index + 1].offset : markdown.length;
      return {
        anchor: sectionAnchor(heading.heading),
        heading: heading.heading,
        start: heading.offset,
        end,
        content: markdown.slice(heading.offset, end),
      };
    });
  }

  function replaceMarkdownSection(markdown, section, replacement) {
    if (!section || section.start < 0 || section.end < section.start || section.end > markdown.length) {
      return '';
    }
    const candidate = normalizeReplacementSection(replacement, section.heading);
    if (!candidate) {
      return '';
    }
    return markdown.slice(0, section.start) + candidate + markdown.slice(section.end);
  }

  function normalizeReplacementSection(replacement, heading) {
    let candidate = String(replacement || '').trim();
    if (!candidate) {
      return '';
    }
    if (!candidate.startsWith('## ')) {
      candidate = `## ${heading}\n\n${candidate}`;
    }
    if (!candidate.endsWith('\n')) {
      candidate += '\n';
    }
    const sections = markdownSections(candidate);
    if (sections.length !== 1 || sections[0].anchor !== sectionAnchor(heading)) {
      return '';
    }
    return candidate;
  }

  function sectionAnchor(heading) {
    return String(heading || '')
      .trim()
      .replace(/^##\s+/, '')
      .toLowerCase()
      .replaceAll(' ', '-')
      .replaceAll('　', '-')
      .replaceAll('_', '-')
      .replaceAll('/', '-')
      .replace(/[:"'`?!：？！]/g, '')
      .replace(/-+/g, '-')
      .replace(/^-|-$/g, '');
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

  function copyPreviewText() {
    navigator.clipboard.writeText(el.previewContent.innerText || '').then(() => {
      const original = el.copyPreview.textContent;
      el.copyPreview.textContent = 'コピーしました';
      setTimeout(() => {
        el.copyPreview.textContent = original;
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
