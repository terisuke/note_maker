document.addEventListener('DOMContentLoaded', () => {
  const configStorageKey = 'note-maker-config-v1';
  const historyEndpoint = '/api/workflow/artifacts';
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
    'reader_problem',
    'key_takeaway',
    'concrete_example',
    'evidence',
    'title_keywords',
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
    historyStyles: [],
    historySessions: [],
    historyProjects: [],
    historyArticles: [],
    historyDrafts: [],
    historyLoading: false,
    historyError: '',
    historyRequestId: 0,
    selectedHistoryStyle: null,
    selectedHistorySession: null,
    selectedHistoryProject: null,
    selectedHistoryArticle: null,
    selectedHistoryDraft: null,
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
    historyPersonaSelect: document.getElementById('history-persona-select'),
    historyProjectSelect: document.getElementById('history-project-select'),
    historyArticleSelect: document.getElementById('history-article-select'),
    historyDraftSelect: document.getElementById('history-draft-select'),
    historyStyleSelect: document.getElementById('history-style-select'),
    historySessionSelect: document.getElementById('history-session-select'),
    refreshHistory: document.getElementById('refresh-history-btn'),
    openHistory: document.getElementById('open-history-btn'),
    clearHistorySelection: document.getElementById('clear-history-selection-btn'),
    historyStatus: document.getElementById('history-status'),
    historyArticleDetail: document.getElementById('history-article-detail'),
    historyProjectCard: document.getElementById('history-project-card'),
    historyArticleCard: document.getElementById('history-article-card'),
    historyArticleBriefCard: document.getElementById('history-article-brief-card'),
    historyCurrentDraftCard: document.getElementById('history-current-draft-card'),
    historyDraftVersionsCard: document.getElementById('history-draft-versions-card'),
    historySourceSnapshotCard: document.getElementById('history-source-snapshot-card'),
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
    styleGuideCard: document.getElementById('style-guide-card'),
    guidePreview: document.getElementById('guide-preview'),
    startInterview: document.getElementById('start-interview-btn'),
    interviewArea: document.getElementById('interview-area'),
    questionLog: document.getElementById('question-log'),
    answerInput: document.getElementById('answer-input'),
    submitAnswer: document.getElementById('submit-answer-btn'),
    cancelAnswer: document.getElementById('cancel-answer-btn'),
    skipDeepDive: document.getElementById('skip-deep-dive-btn'),
    briefResult: document.getElementById('brief-result'),
    briefCard: document.getElementById('brief-card'),
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
  renderHistoryArticleDetail();
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
  el.historyPersonaSelect.addEventListener('change', loadWorkflowHistory);
  el.historyProjectSelect.addEventListener('change', selectHistoryProject);
  el.historyArticleSelect.addEventListener('change', selectHistoryArticle);
  el.historyDraftSelect.addEventListener('change', selectHistoryDraft);
  el.historyStyleSelect.addEventListener('change', selectHistoryStyle);
  el.historySessionSelect.addEventListener('change', selectHistorySession);
  el.refreshHistory.addEventListener('click', loadWorkflowHistory);
  el.openHistory.addEventListener('click', openSelectedHistory);
  el.clearHistorySelection.addEventListener('click', clearHistorySelection);
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
      populateHistoryPersonaSelect();
      applyPersonaDefaults(false);
      renderModeSummary();
      loadQuestionTemplate();
      loadWorkflowHistory();
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
    const sourceSelector = el.username.value.trim() || defaultStyleSourceSelector();
    if (!sourceSelector) {
      showError('文体ソースを入力してください');
      return;
    }
    setLoading(true, '選択中の媒体ソースから記事を取得し、文体を分析しています...');
    try {
      const data = await requestJSON('/api/author-style/analyze', {
        method: 'POST',
        body: {
          username: sourceSelector,
          source_selector: sourceSelector,
          limit: Number(el.limit.value),
          style_model: el.styleModel.value,
          persona_id: currentPersonaId(),
          output_format_id: currentFormatId(),
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
          output_format_id: currentFormatId(),
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
    renderStyleGuideCard(data);
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
      renderBriefCard(null);
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
    let content = el.answerInput.value.trim();
    if (!content) {
      if (isQuestionRequired(state.nextQuestion)) {
        showError('回答を入力してください');
        return;
      }
      content = '未定';
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
      renderBriefCard(data.brief);
      el.briefResult.classList.remove('hidden');
      el.generateDraft.disabled = false;
      el.skipDeepDive.classList.add('hidden');
      return;
    }
    state.completedBrief = null;
    state.nextQuestion = data.next_question;
    el.briefResult.classList.add('hidden');
    renderBriefCard(null);
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
    el.draftStatus.textContent = 'OpenAI互換APIで下書きを生成しています。';
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
            el.draftStatus.textContent = draftStatusText(data);
            return;
          }
          if (event === 'heartbeat') {
            el.draftStatus.textContent = draftStatusText({ ...data, status: 'running' });
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
            if (data.quality_gate) {
              renderDraft(data);
              el.draftStatus.textContent = '品質ゲートで停止しました。生成済みMarkdownと評価詳細を残しています。';
            }
            const error = new Error(data.message || data.detail || 'stream error');
            error.payload = data;
            throw error;
          }
          if (event === 'done') {
            el.draftStatus.textContent = draftDoneText(data);
          }
        },
      });
    } catch (error) {
      if (error.name === 'AbortError') {
        el.draftStatus.textContent = '停止しました。途中まで生成されたMarkdownは残しています。';
      } else if (error.payload?.quality_gate) {
        showError(`下書き生成は品質ゲートで停止しました: ${error.message}`);
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
    label.textContent = pendingQuestionLabel(question);
    const text = document.createElement('p');
    text.textContent = question.text || '質問を準備しています...';
    if (question.flow_type !== 'deep_dive_follow_up') {
      el.answerInput.placeholder = isQuestionRequired(question)
        ? '短くても大丈夫です。箇条書きでも入力できます。'
        : '任意です。空のまま送ると「未定」で進みます。';
    }
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

  function pendingQuestionLabel(question) {
    if (question.flow_type === 'deep_dive_follow_up') {
      return '次の深掘り質問';
    }
    return isQuestionRequired(question) ? '次の質問' : '次の質問（任意）';
  }

  function isQuestionRequired(question) {
    if (!question) {
      return true;
    }
    return question.required !== false && question.Required !== false;
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

  function populateHistoryPersonaSelect() {
    el.historyPersonaSelect.innerHTML = '';
    state.personas.forEach((persona) => {
      const option = document.createElement('option');
      option.value = persona.id;
      option.textContent = persona.display_name;
      option.selected = persona.id === currentPersonaId();
      el.historyPersonaSelect.appendChild(option);
    });
    if (!el.historyPersonaSelect.value && state.personas[0]) {
      el.historyPersonaSelect.value = state.personas[0].id;
    }
  }

  function onPersonaChange() {
    config.mode.persona = currentPersonaId();
    applyPersonaDefaults(true);
    applyStyleSourceDefault(true);
    saveConfig();
    renderModeSummary();
    loadQuestionTemplate();
    syncHistoryPersonaToCurrentMode();
    loadWorkflowHistory();
  }

  function onFormatChange() {
    config.mode.format = currentFormatId();
    applyStyleSourceDefault(true);
    saveConfig();
    renderModeSummary();
    loadQuestionTemplate();
    loadWorkflowHistory();
  }

  function syncHistoryPersonaToCurrentMode() {
    if (el.historyPersonaSelect.value !== currentPersonaId()) {
      el.historyPersonaSelect.value = currentPersonaId();
    }
  }

  async function loadWorkflowHistory() {
    const personaId = el.historyPersonaSelect.value || currentPersonaId();
    const formatId = currentFormatId();
    const requestId = state.historyRequestId + 1;
    state.historyRequestId = requestId;
    state.historyLoading = true;
    state.historyError = '';
    state.selectedHistoryStyle = null;
    state.selectedHistorySession = null;
    state.selectedHistoryProject = null;
    state.selectedHistoryArticle = null;
    state.selectedHistoryDraft = null;
    renderHistoryPicker();
    renderHistoryArticleDetail();

    try {
      const data = await fetchWorkflowHistoryIndex({ personaId, formatId });
      if (requestId !== state.historyRequestId) {
        return;
      }
      const normalized = normalizeWorkflowHistory(data, personaId, formatId);
      state.historyStyles = normalized.styles;
      state.historySessions = normalized.sessions;
      state.historyProjects = normalized.projects;
      state.historyArticles = normalized.articles;
      state.historyDrafts = normalized.drafts;
    } catch (error) {
      if (requestId !== state.historyRequestId) {
        return;
      }
      state.historyStyles = [];
      state.historySessions = [];
      state.historyProjects = [];
      state.historyArticles = [];
      state.historyDrafts = [];
      state.historyError = historyErrorMessage(error);
    } finally {
      if (requestId === state.historyRequestId) {
        state.historyLoading = false;
        renderHistoryPicker();
        renderHistoryArticleDetail();
      }
    }
  }

  async function fetchWorkflowHistoryIndex({ personaId, formatId }) {
    const params = new URLSearchParams();
    if (personaId) {
      params.set('persona_id', personaId);
    }
    if (formatId) {
      params.set('format_id', formatId);
    }
    return requestJSON(`${historyEndpoint}?${params.toString()}`);
  }

  function renderHistoryPicker() {
    const articles = filteredHistoryArticles();
    const drafts = filteredHistoryDrafts();
    renderHistoryOptions(el.historyProjectSelect, state.historyProjects, 'プロジェクトを選択');
    renderHistoryOptions(el.historyArticleSelect, articles, '記事を選択');
    renderHistoryOptions(el.historyDraftSelect, drafts, '下書きを選択');
    renderHistoryOptions(el.historyStyleSelect, state.historyStyles, '文体ガイドを選択');
    renderHistoryOptions(el.historySessionSelect, state.historySessions, '取材セッションを選択');

    el.historyProjectSelect.disabled = state.historyLoading || !state.historyProjects.length;
    el.historyArticleSelect.disabled = state.historyLoading || !articles.length;
    el.historyDraftSelect.disabled = state.historyLoading || !drafts.length;
    el.historyStyleSelect.disabled = state.historyLoading || !state.historyStyles.length;
    el.historySessionSelect.disabled = state.historyLoading || !state.historySessions.length;
    el.openHistory.disabled = state.historyLoading || !historySelectionReady();

    if (state.historyLoading) {
      el.historyStatus.className = 'history-status loading';
      el.historyStatus.textContent = '保存済みのプロジェクト、記事、下書き、文体ガイド、取材セッションを読み込んでいます...';
      return;
    }
    if (state.historyError) {
      el.historyStatus.className = 'history-status warning';
      el.historyStatus.textContent = state.historyError;
      return;
    }
    if (!state.historyProjects.length && !state.historyArticles.length && !state.historyDrafts.length && !state.historyStyles.length && !state.historySessions.length) {
      el.historyStatus.className = 'history-status empty';
      el.historyStatus.textContent = 'この書き手と出力先の保存済み履歴はまだありません。';
      return;
    }
    const projectCount = `${state.historyProjects.length}件のプロジェクト`;
    const articleCount = `${state.historyArticles.length}件の記事`;
    const draftCount = `${state.historyDrafts.length}件の下書き`;
    const styleCount = `${state.historyStyles.length}件の文体ガイド`;
    const sessionCount = `${state.historySessions.length}件の取材セッション`;
    el.historyStatus.className = 'history-status';
    el.historyStatus.textContent = `${projectCount} / ${articleCount} / ${draftCount} / ${styleCount} / ${sessionCount} を選択できます。`;
  }

  function renderHistoryOptions(select, items, placeholder) {
    const selected = select.value;
    select.innerHTML = '';
    const empty = document.createElement('option');
    empty.value = '';
    empty.textContent = placeholder;
    select.appendChild(empty);
    items.forEach((item) => {
      const option = document.createElement('option');
      option.value = item.id;
      option.textContent = historyOptionLabel(item);
      select.appendChild(option);
    });
    if (items.some((item) => item.id === selected)) {
      select.value = selected;
    }
  }

  function historyOptionLabel(item) {
    const title = item.title || item.theme || item.label || item.id;
    const status = item.completed === true ? '完了' : item.phase || '';
    const updatedAt = formatDateTime(item.updatedAt || item.createdAt);
    return [title, status, updatedAt].filter(Boolean).join(' / ');
  }

  function historySelectionReady() {
    return Boolean(
      el.historyProjectSelect.value
      || el.historyArticleSelect.value
      || el.historyDraftSelect.value
      || el.historyStyleSelect.value
      || el.historySessionSelect.value,
    );
  }

  function historyErrorMessage(error) {
    const message = error.message || '';
    if (message.includes('HTTP 404')) {
      return '履歴APIはまだ接続されていません。バックエンド実装後にここへ保存済み履歴が表示されます。';
    }
    return `履歴の取得に失敗しました: ${message}`;
  }

  async function selectHistoryStyle() {
    state.selectedHistoryStyle = findHistoryStyle(el.historyStyleSelect.value);
    el.openHistory.disabled = !historySelectionReady();
    if (!state.selectedHistoryStyle) {
      return;
    }
    el.historyStatus.className = 'history-status loading';
    el.historyStatus.textContent = '保存済み文体ガイドを確認しています...';
    try {
      const detail = await loadHistoryStyleDetail(state.selectedHistoryStyle);
      state.selectedHistoryStyle = detail;
      renderStyleGuideCard(detail);
      el.guidePreview.textContent = styleGuideMarkdown(detail);
      el.styleResult.classList.remove('hidden');
      renderHistoryPicker();
    } catch (error) {
      el.historyStatus.className = 'history-status warning';
      el.historyStatus.textContent = `文体ガイドを開けませんでした: ${error.message}`;
    }
  }

  async function selectHistorySession() {
    state.selectedHistorySession = findHistorySession(el.historySessionSelect.value);
    el.openHistory.disabled = !historySelectionReady();
    if (!state.selectedHistorySession) {
      return;
    }
    el.historyStatus.className = 'history-status loading';
    el.historyStatus.textContent = '保存済み取材セッションを確認しています...';
    try {
      const detail = await loadHistorySessionDetail(state.selectedHistorySession);
      state.selectedHistorySession = detail;
      if (detail.brief) {
        renderBriefCard(detail.brief);
        el.briefPreview.textContent = JSON.stringify(detail.brief, null, 2);
        el.briefResult.classList.remove('hidden');
      }
      renderHistoryPicker();
    } catch (error) {
      el.historyStatus.className = 'history-status warning';
      el.historyStatus.textContent = `取材セッションを開けませんでした: ${error.message}`;
    }
  }

  async function selectHistoryProject() {
    state.selectedHistoryProject = findHistoryProject(el.historyProjectSelect.value);
    state.selectedHistoryArticle = null;
    state.selectedHistoryDraft = null;
    el.historyArticleSelect.value = '';
    el.historyDraftSelect.value = '';
    renderHistoryPicker();
    renderHistoryArticleDetail();
    if (!state.selectedHistoryProject) {
      return;
    }
    el.historyStatus.className = 'history-status loading';
    el.historyStatus.textContent = 'プロジェクト履歴を確認しています...';
    try {
      state.selectedHistoryProject = await loadHistoryProjectDetail(state.selectedHistoryProject);
      mergeProjectDetailIntoHistory(state.selectedHistoryProject);
      renderHistoryPicker();
      renderHistoryArticleDetail();
    } catch (error) {
      renderHistoryArticleDetail({ warning: `プロジェクト詳細APIは未接続です: ${error.message}` });
      renderHistoryPicker();
    }
  }

  async function selectHistoryArticle() {
    state.selectedHistoryArticle = findHistoryArticle(el.historyArticleSelect.value);
    state.selectedHistoryDraft = null;
    el.historyDraftSelect.value = '';
    renderHistoryPicker();
    renderHistoryArticleDetail();
    if (!state.selectedHistoryArticle) {
      return;
    }
    if (state.selectedHistoryArticle.projectId && !state.selectedHistoryProject) {
      state.selectedHistoryProject = findHistoryProject(state.selectedHistoryArticle.projectId);
      if (state.selectedHistoryProject) {
        el.historyProjectSelect.value = state.selectedHistoryProject.id;
      }
    }
    el.historyStatus.className = 'history-status loading';
    el.historyStatus.textContent = '記事履歴を確認しています...';
    try {
      state.selectedHistoryArticle = await loadHistoryArticleDetail(state.selectedHistoryArticle);
      mergeArticleDetailIntoHistory(state.selectedHistoryArticle);
      renderHistoryPicker();
      renderHistoryArticleDetail();
    } catch (error) {
      renderHistoryArticleDetail({ warning: `記事詳細APIは未接続です: ${error.message}` });
      renderHistoryPicker();
    }
  }

  async function selectHistoryDraft() {
    state.selectedHistoryDraft = findHistoryDraft(el.historyDraftSelect.value);
    if (!state.selectedHistoryDraft) {
      renderHistoryArticleDetail();
      el.openHistory.disabled = !historySelectionReady();
      return;
    }
    if (state.selectedHistoryDraft.articleId && !state.selectedHistoryArticle) {
      state.selectedHistoryArticle = findHistoryArticle(state.selectedHistoryDraft.articleId);
      if (state.selectedHistoryArticle) {
        el.historyArticleSelect.value = state.selectedHistoryArticle.id;
      }
    }
    el.historyStatus.className = 'history-status loading';
    el.historyStatus.textContent = '下書き履歴を確認しています...';
    try {
      state.selectedHistoryDraft = await loadHistoryDraftDetail(state.selectedHistoryDraft);
      mergeDraftDetailIntoHistory(state.selectedHistoryDraft);
      renderHistoryPicker();
      renderHistoryArticleDetail();
    } catch (error) {
      renderHistoryArticleDetail({ warning: `下書き詳細APIは未接続です: ${error.message}` });
      renderHistoryPicker();
    }
  }

  async function openSelectedHistory() {
    clearError();
    el.historyStatus.className = 'history-status loading';
    el.historyStatus.textContent = '選択した履歴を開いています...';
    try {
      const style = el.historyStyleSelect.value
        ? await loadHistoryStyleDetail(state.selectedHistoryStyle || findHistoryStyle(el.historyStyleSelect.value))
        : null;
      const session = el.historySessionSelect.value
        ? await loadHistorySessionDetail(state.selectedHistorySession || findHistorySession(el.historySessionSelect.value))
        : null;
      const article = el.historyArticleSelect.value
        ? await loadHistoryArticleDetail(state.selectedHistoryArticle || findHistoryArticle(el.historyArticleSelect.value))
        : null;
      const draft = el.historyDraftSelect.value
        ? await loadHistoryDraftDetail(state.selectedHistoryDraft || findHistoryDraft(el.historyDraftSelect.value))
        : null;
      const styleForSession = !style && session?.styleProfileId
        ? await loadHistoryStyleDetail({ id: session.styleProfileId })
        : style;

      if (styleForSession) {
        applyHistoryStyle(styleForSession);
      }
      if (session) {
        await applyHistorySession(session);
      }
      if (article) {
        await applyHistoryArticle(article);
      }
      if (draft) {
        applyHistoryDraft(draft);
      }
      if (!styleForSession && !session && !article && !draft && !el.historyProjectSelect.value) {
        showError('開く履歴を選択してください');
        return;
      }
      el.historyStatus.className = 'history-status';
      el.historyStatus.textContent = '選択した履歴を現在の作業状態に反映しました。';
    } catch (error) {
      el.historyStatus.className = 'history-status warning';
      el.historyStatus.textContent = `履歴を開けませんでした: ${error.message}`;
    }
  }

  function clearHistorySelection() {
    el.historyProjectSelect.value = '';
    el.historyArticleSelect.value = '';
    el.historyDraftSelect.value = '';
    el.historyStyleSelect.value = '';
    el.historySessionSelect.value = '';
    state.selectedHistoryProject = null;
    state.selectedHistoryArticle = null;
    state.selectedHistoryDraft = null;
    state.selectedHistoryStyle = null;
    state.selectedHistorySession = null;
    renderHistoryPicker();
    renderHistoryArticleDetail();
  }

  async function loadHistoryStyleDetail(item) {
    if (!item) {
      return null;
    }
    if (styleGuideMarkdown(item)) {
      return item;
    }
    const data = await requestJSON(`/api/author-style/${encodeURIComponent(item.id)}`);
    return normalizeHistoryStyle({ ...item, ...data });
  }

  async function loadHistorySessionDetail(item) {
    if (!item) {
      return null;
    }
    if (item.answers?.length || item.brief || item.nextQuestion) {
      return item;
    }
    const data = await requestJSON(`/api/brief-sessions/${encodeURIComponent(item.id)}`);
    return normalizeHistorySession({ ...item, ...data });
  }

  async function loadHistoryProjectDetail(item) {
    if (!item) {
      return null;
    }
    const normalized = normalizeHistoryProject(item);
    if (normalized.articles.length) {
      return normalized;
    }
    const data = await requestFirstJSON([
      `/api/projects/${encodeURIComponent(normalized.id)}`,
      `/api/history/projects/${encodeURIComponent(normalized.id)}`,
    ]);
    return normalizeHistoryProject({ ...item, ...data });
  }

  async function loadHistoryArticleDetail(item) {
    if (!item) {
      return null;
    }
    const normalized = normalizeHistoryArticle(item);
    if (hasArticleDetail(normalized)) {
      return normalized;
    }
    const urls = [`/api/articles/${encodeURIComponent(normalized.id)}`];
    if (normalized.projectId) {
      urls.push(`/api/projects/${encodeURIComponent(normalized.projectId)}/articles/${encodeURIComponent(normalized.id)}`);
    }
    if (normalized.briefId) {
      urls.push(`/api/briefs/${encodeURIComponent(normalized.briefId)}`);
    }
    urls.push(`/api/history/articles/${encodeURIComponent(normalized.id)}`);
    const data = await requestFirstJSON(urls);
    const detail = data && !data.article && !data.Article && (data.brief || data.Brief || data.theme || data.Theme)
      ? { brief: data }
      : data;
    return normalizeHistoryArticle({ ...item, ...detail });
  }

  async function loadHistoryDraftDetail(item) {
    if (!item) {
      return null;
    }
    const normalized = normalizeHistoryDraft(item);
    if (normalized.markdown || normalized.summary) {
      return normalized;
    }
    const data = await requestFirstJSON([
      `/api/drafts/${encodeURIComponent(normalized.id)}`,
      `/api/history/drafts/${encodeURIComponent(normalized.id)}`,
    ]);
    return normalizeHistoryDraft({ ...item, ...data });
  }

  async function requestFirstJSON(urls) {
    let lastError = null;
    for (const url of urls.filter(Boolean)) {
      try {
        return await requestJSON(url);
      } catch (error) {
        lastError = error;
      }
    }
    throw lastError || new Error('履歴詳細APIが見つかりません');
  }

  function hasArticleDetail(article) {
    return Boolean(
      article.brief
      || article.currentDraft?.markdown
      || article.currentDraft?.summary
      || article.draftVersions.length
      || article.sourceSnapshot,
    );
  }

  function applyHistoryStyle(item) {
    const data = normalizeHistoryStyle(item);
    state.profileId = data.profileId || data.id;
    el.profileId.textContent = state.profileId;
    el.guideId.textContent = data.guideId || '';
    el.articleCount.textContent = data.articleCount === undefined ? '' : String(data.articleCount);
    el.guidePreview.textContent = styleGuideMarkdown(data);
    renderStyleGuideCard(data);
    el.styleResult.classList.remove('hidden');
    el.startInterview.disabled = !state.profileId;
  }

  async function applyHistorySession(item) {
    const data = normalizeHistorySession(item);
    if (data.personaId && state.personas.some((persona) => persona.id === data.personaId)) {
      el.personaSelect.value = data.personaId;
      config.mode.persona = data.personaId;
    }
    if (data.outputFormatId && state.formats.some((format) => format.id === data.outputFormatId)) {
      el.formatSelect.value = data.outputFormatId;
      config.mode.format = data.outputFormatId;
    }
    saveConfig();
    renderModeSummary();
    applyStyleSourceDefault(true);
    await loadQuestionTemplate();
    state.sessionId = data.id;
    state.parentSessionId = data.parentSessionId || '';
    state.profileId = data.styleProfileId || state.profileId;
    state.answers = data.answers || [];
    state.nextQuestion = data.nextQuestion || null;
    state.completedBrief = data.completed ? data.brief : null;
    rememberQuestions(data.questions || state.templateQuestions);
    rememberQuestion(data.nextQuestion);
    el.interviewArea.classList.remove('hidden');
    renderTranscript({
      answers: state.answers,
      next_question: state.nextQuestion,
      completed: data.completed,
    });
    if (data.completed && data.brief) {
      el.briefPreview.textContent = JSON.stringify(data.brief, null, 2);
      renderBriefCard(data.brief);
      el.briefResult.classList.remove('hidden');
      el.generateDraft.disabled = !state.profileId;
      el.skipDeepDive.classList.add('hidden');
    } else {
      renderBriefCard(null);
      el.briefResult.classList.add('hidden');
      el.generateDraft.disabled = true;
      el.skipDeepDive.classList.toggle('hidden', data.nextQuestion?.flow_type !== 'deep_dive_follow_up');
    }
    updateSectionControls();
  }

  async function applyHistoryArticle(item) {
    const data = normalizeHistoryArticle(item);
    state.selectedHistoryArticle = data;
    state.profileId = data.styleProfileId || state.profileId;
    state.sessionId = data.sessionId || state.sessionId;
    if (data.personaId && state.personas.some((persona) => persona.id === data.personaId)) {
      el.personaSelect.value = data.personaId;
      config.mode.persona = data.personaId;
    }
    if (data.outputFormatId && state.formats.some((format) => format.id === data.outputFormatId)) {
      el.formatSelect.value = data.outputFormatId;
      config.mode.format = data.outputFormatId;
    }
    saveConfig();
    renderModeSummary();
    await loadQuestionTemplate();
    if (data.brief) {
      state.completedBrief = data.brief;
      renderBriefCard(data.brief);
      el.briefPreview.textContent = JSON.stringify(data.brief, null, 2);
      el.briefResult.classList.remove('hidden');
      el.generateDraft.disabled = !state.profileId;
    }
    const draft = state.selectedHistoryDraft || data.currentDraft;
    if (draft) {
      applyHistoryDraft(draft);
    }
    renderHistoryArticleDetail();
  }

  function applyHistoryDraft(item) {
    const data = normalizeHistoryDraft(item);
    state.selectedHistoryDraft = data;
    state.sessionId = data.sessionId || state.sessionId;
    state.profileId = data.styleProfileId || state.profileId;
    const draftText = data.markdown || data.summary || '';
    if (draftText) {
      el.markdownOutput.value = draftText;
      syncDraftEditor();
      el.draftResult.classList.remove('hidden');
      setActiveTab('preview');
    }
    el.draftStatus.textContent = historyDraftResumeText(data);
    el.generateDraft.disabled = !state.profileId || !state.sessionId;
    renderHistoryArticleDetail();
  }

  function normalizeWorkflowHistory(data, personaId, formatId) {
    const source = data || {};
    const styleValues = arrayFrom(source.style_guides || source.styleGuides || source.styles || source.author_styles || source.authorStyles || source.profiles);
    const briefValues = arrayFrom(source.briefs || source.Briefs);
    const sessionValues = [
      ...arrayFrom(source.sessions || source.brief_sessions || source.briefSessions || source.items || (Array.isArray(source) ? source : [])),
      ...briefValues,
    ];
    const projectValues = arrayFrom(source.projects || source.Projects);
    const projectArticleValues = projectValues.flatMap((project) => arrayFrom(project.articles || project.Articles)
      .map((article) => ({ project_id: project.id || project.ID || project.project_id || project.projectId, ...article })));
    const sourceSnapshotValues = arrayFrom(source.source_snapshots || source.sourceSnapshots || source.SourceSnapshots);
    const briefArticleValues = briefValues.map((brief) => ({
      article_id: brief.article_id || brief.articleId || brief.session_id || brief.sessionId || brief.id || brief.ID,
      persona_id: brief.persona_id || brief.personaId,
      output_format_id: brief.output_format_id || brief.outputFormatId,
      brief,
    }));
    const snapshotArticleValues = sourceSnapshotValues.map((snapshot) => ({
      article_id: snapshot.article_id || snapshot.articleId || snapshot.id || snapshot.ID,
      persona_id: snapshot.persona_id || snapshot.personaId,
      output_format_id: snapshot.output_format_id || snapshot.outputFormatId,
      source_snapshot: snapshot,
    }));
    const articleValues = [
      ...arrayFrom(source.articles || source.Articles),
      ...arrayFrom(source.article_history || source.articleHistory),
      ...projectArticleValues,
      ...briefArticleValues,
      ...snapshotArticleValues,
    ];
    const articleDraftValues = articleValues.flatMap((article) => [
      ...arrayFrom(article.drafts || article.Drafts || article.draft_versions || article.draftVersions),
      ...[article.current_draft || article.currentDraft || article.draft || article.Draft].filter(Boolean),
    ].map((draft) => ({
      article_id: article.id || article.ID || article.article_id || article.articleId,
      persona_id: article.persona_id || article.personaId,
      output_format_id: article.output_format_id || article.outputFormatId,
      ...draft,
    })));
    const draftValues = [
      ...arrayFrom(source.drafts || source.Drafts),
      ...arrayFrom(source.draft_versions || source.draftVersions),
      ...articleDraftValues,
    ];
    return {
      styles: styleValues.map(normalizeHistoryStyle)
        .filter((item) => item.id)
        .filter((item) => historyItemMatches(item, personaId, formatId)),
      sessions: uniqueHistoryItems(sessionValues.map(normalizeHistorySession)
        .filter((item) => item.id)
        .filter((item) => historyItemMatches(item, personaId, formatId))),
      projects: uniqueHistoryItems(projectValues.map(normalizeHistoryProject)
        .filter((item) => item.id)
        .filter((item) => historyItemMatches(item, personaId, formatId))),
      articles: uniqueHistoryItems(articleValues.map(normalizeHistoryArticle)
        .filter((item) => item.id)
        .filter((item) => historyItemMatches(item, personaId, formatId))),
      drafts: uniqueHistoryItems(draftValues.map(normalizeHistoryDraft)
        .filter((item) => item.id)
        .filter((item) => historyItemMatches(item, personaId, formatId))),
    };
  }

  function normalizeHistoryStyle(item = {}) {
    const profile = item.profile || item.Profile || {};
    const guide = item.guide || item.Guide || {};
    return {
      ...item,
      id: String(item.profile_id || item.profileId || item.style_profile_id || item.styleProfileId || profile.id || profile.ID || item.id || item.ID || '').trim(),
      resultId: item.id || item.ID || '',
      profileId: item.profile_id || item.profileId || item.style_profile_id || item.styleProfileId || profile.id || profile.ID || '',
      guideId: item.guide_id || item.guideId || guide.id || guide.ID || '',
      title: item.title || item.name || item.label || item.display_name || item.displayName || profile.name || profile.Name || '',
      personaId: item.persona_id || item.personaId || profile.persona_id || profile.PersonaID || '',
      outputFormatId: item.output_format_id || item.outputFormatId || profile.output_format_id || profile.OutputFormatID || '',
      articleCount: item.article_count ?? item.articleCount ?? item.source?.article_count ?? item.Source?.ArticleCount,
      guideMarkdown: item.guide_markdown || item.guideMarkdown || item.markdown || item.Markdown || guide.markdown || guide.Markdown || '',
      updatedAt: item.updated_at || item.updatedAt || item.created_at || item.createdAt || '',
      createdAt: item.created_at || item.createdAt || '',
      source: item.source || item.Source || {},
      profile,
      guide,
    };
  }

  function normalizeHistorySession(item = {}) {
    const brief = item.brief || item.Brief || null;
    const title = item.title || item.name || briefField(brief, 'theme', 'Theme') || '';
    return {
      ...item,
      id: String(item.session_id || item.sessionId || item.id || item.ID || '').trim(),
      title,
      theme: briefField(brief, 'theme', 'Theme'),
      styleProfileId: item.style_profile_id || item.styleProfileId || item.profile_id || item.profileId || briefField(brief, 'styleProfileId', 'StyleProfileID') || '',
      personaId: item.persona_id || item.personaId || briefField(brief, 'personaId', 'PersonaID') || '',
      outputFormatId: item.output_format_id || item.outputFormatId || briefField(brief, 'outputFormatId', 'OutputFormatID') || '',
      parentSessionId: item.parent_session_id || item.parentSessionId || '',
      phase: item.phase || item.Phase || '',
      completed: item.completed ?? item.Completed ?? Boolean(brief),
      brief,
      answers: item.answers || item.Answers || [],
      questions: normalizeQuestionList(item.questions || item.Questions || []),
      nextQuestion: normalizeHistoryQuestion(item.next_question || item.nextQuestion || item.NextQuestion || null),
      updatedAt: item.updated_at || item.updatedAt || item.created_at || item.createdAt || '',
      createdAt: item.created_at || item.createdAt || '',
    };
  }

  function normalizeHistoryProject(item = {}) {
    const project = item.project || item.Project || {};
    const id = String(item.project_id || item.projectId || project.id || project.ID || item.id || item.ID || '').trim();
    return {
      ...item,
      id,
      title: item.title || item.name || item.label || item.display_name || item.displayName || project.title || project.Title || project.name || project.Name || id,
      personaId: item.persona_id || item.personaId || project.persona_id || project.PersonaID || '',
      outputFormatId: item.output_format_id || item.outputFormatId || project.output_format_id || project.OutputFormatID || '',
      status: item.status || item.Status || project.status || project.Status || '',
      articleCount: item.article_count ?? item.articleCount ?? project.article_count ?? project.ArticleCount,
      articles: arrayFrom(item.articles || item.Articles || project.articles || project.Articles).map((article) => normalizeHistoryArticle({ project_id: id, ...article })),
      updatedAt: item.updated_at || item.updatedAt || item.created_at || item.createdAt || project.updated_at || project.UpdatedAt || '',
      createdAt: item.created_at || item.createdAt || project.created_at || project.CreatedAt || '',
    };
  }

  function normalizeHistoryArticle(item = {}) {
    const article = item.article || item.Article || {};
    const brief = item.brief || item.Brief || item.article_brief || item.articleBrief || article.brief || article.Brief || null;
    const currentDraft = normalizeMaybeDraft(item.current_draft || item.currentDraft || item.draft || item.Draft || article.current_draft || article.CurrentDraft || null);
    const draftVersions = arrayFrom(item.draft_versions || item.draftVersions || item.drafts || item.Drafts || article.draft_versions || article.DraftVersions || article.drafts || article.Drafts)
      .map((draft) => normalizeHistoryDraft({ article_id: item.article_id || item.articleId || article.id || article.ID || item.id || item.ID, ...draft }))
      .filter((draft) => draft.id || draft.markdown || draft.summary);
    const id = String(item.article_id || item.articleId || article.id || article.ID || item.id || item.ID || '').trim();
    const title = item.title || item.name || article.title || article.Title || briefField(brief, 'theme', 'Theme') || currentDraft?.title || id;
    return {
      ...item,
      id,
      projectId: item.project_id || item.projectId || article.project_id || article.ProjectID || '',
      title,
      theme: briefField(brief, 'theme', 'Theme'),
      personaId: item.persona_id || item.personaId || article.persona_id || article.PersonaID || briefField(brief, 'persona_id', 'PersonaID') || '',
      outputFormatId: item.output_format_id || item.outputFormatId || article.output_format_id || article.OutputFormatID || briefField(brief, 'output_format_id', 'OutputFormatID') || '',
      styleProfileId: item.style_profile_id || item.styleProfileId || article.style_profile_id || article.StyleProfileID || briefField(brief, 'style_profile_id', 'StyleProfileID') || '',
      sessionId: item.session_id || item.sessionId || item.brief_session_id || item.briefSessionId || article.session_id || article.SessionID || '',
      briefId: item.brief_id || item.briefId || article.brief_id || article.BriefID || '',
      status: item.status || item.Status || item.phase || item.Phase || article.status || article.Status || '',
      brief,
      currentDraft,
      draftVersions,
      sourceSnapshot: item.source_snapshot || item.sourceSnapshot || article.source_snapshot || article.SourceSnapshot || null,
      updatedAt: item.updated_at || item.updatedAt || item.created_at || item.createdAt || article.updated_at || article.UpdatedAt || '',
      createdAt: item.created_at || item.createdAt || article.created_at || article.CreatedAt || '',
    };
  }

  function normalizeMaybeDraft(draft) {
    if (!draft) {
      return null;
    }
    const normalized = normalizeHistoryDraft(draft);
    return normalized.id || normalized.markdown || normalized.summary ? normalized : null;
  }

  function normalizeHistoryDraft(item = {}) {
    const draft = item.draft || item.Draft || {};
    const markdown = item.markdown || item.Markdown || item.draft_markdown || item.draftMarkdown || item.text || item.Text || item.body || item.Body || item.content || item.Content || draft.markdown || draft.Markdown || draft.text || draft.Text || '';
    const id = String(item.draft_id || item.draftId || draft.id || draft.ID || item.id || item.ID || '').trim();
    return {
      ...item,
      id,
      articleId: item.article_id || item.articleId || draft.article_id || draft.ArticleID || '',
      personaId: item.persona_id || item.personaId || draft.persona_id || draft.PersonaID || '',
      outputFormatId: item.output_format_id || item.outputFormatId || draft.output_format_id || draft.OutputFormatID || '',
      sessionId: item.session_id || item.sessionId || item.brief_session_id || item.briefSessionId || draft.session_id || draft.SessionID || '',
      styleProfileId: item.style_profile_id || item.styleProfileId || draft.style_profile_id || draft.StyleProfileID || '',
      title: item.title || item.name || draft.title || draft.Title || markdownTitle(markdown) || id,
      version: item.version ?? item.Version ?? item.attempt ?? item.Attempt ?? item.revision ?? item.Revision ?? '',
      kind: item.kind || item.Kind || '',
      status: item.status || item.Status || '',
      markdown,
      summary: item.summary || item.Summary || draft.summary || draft.Summary || '',
      score: item.score ?? item.Score ?? item.quality_gate?.score ?? item.qualityGate?.score ?? draft.score ?? draft.Score,
      passed: item.passed ?? item.Passed ?? item.evaluation?.passed ?? item.Evaluation?.Passed,
      runes: item.runes ?? item.Runes ?? item.quality_gate?.runes ?? item.qualityGate?.runes,
      validationError: item.validation_error || item.validationError || item.ValidationError || '',
      verification: item.verification || item.Verification || null,
      updatedAt: item.updated_at || item.updatedAt || item.created_at || item.createdAt || draft.updated_at || draft.UpdatedAt || '',
      createdAt: item.created_at || item.createdAt || draft.created_at || draft.CreatedAt || '',
    };
  }

  function normalizeHistoryQuestion(question) {
    if (!question) {
      return null;
    }
    return {
      id: question.id || question.ID || '',
      text: question.text || question.Text || '',
      flow_type: question.flow_type || question.flowType || question.FlowType || 'main',
      target_field: question.target_field || question.targetField || question.TargetField || '',
      target_question_id: question.target_question_id || question.targetQuestionId || question.TargetQuestionID || '',
      follow_up_index: question.follow_up_index || question.followUpIndex || question.FollowUpIndex || 0,
      required: question.required ?? question.Required,
    };
  }

  function historyItemMatches(item, personaId, formatId) {
    return (!item.personaId || !personaId || item.personaId === personaId)
      && (!item.outputFormatId || !formatId || item.outputFormatId === formatId);
  }

  function findHistoryStyle(id) {
    return state.historyStyles.find((item) => item.id === id) || null;
  }

  function findHistorySession(id) {
    return state.historySessions.find((item) => item.id === id) || null;
  }

  function findHistoryProject(id) {
    return state.historyProjects.find((item) => item.id === id) || null;
  }

  function findHistoryArticle(id) {
    return state.historyArticles.find((item) => item.id === id) || null;
  }

  function findHistoryDraft(id) {
    return state.historyDrafts.find((item) => item.id === id) || null;
  }

  function filteredHistoryArticles() {
    const projectId = el.historyProjectSelect.value;
    if (!projectId) {
      return state.historyArticles;
    }
    return state.historyArticles.filter((article) => !article.projectId || article.projectId === projectId);
  }

  function filteredHistoryDrafts() {
    const articleId = el.historyArticleSelect.value;
    if (!articleId) {
      return state.historyDrafts;
    }
    const article = state.selectedHistoryArticle || findHistoryArticle(articleId);
    const articleDrafts = article?.draftVersions || [];
    const globalDrafts = state.historyDrafts.filter((draft) => !draft.articleId || draft.articleId === articleId);
    return uniqueHistoryItems([...articleDrafts, ...globalDrafts]);
  }

  function uniqueHistoryItems(items) {
    const byId = new Map();
    items.forEach((item) => {
      const existing = byId.get(item.id);
      if (!existing || (!existing.brief && item.brief)) {
        byId.set(item.id, item);
      }
    });
    return [...byId.values()];
  }

  function mergeProjectDetailIntoHistory(project) {
    if (!project) {
      return;
    }
    state.historyProjects = uniqueHistoryItems([project, ...state.historyProjects]);
    if (project.articles.length) {
      state.historyArticles = uniqueHistoryItems([...project.articles, ...state.historyArticles]);
    }
  }

  function mergeArticleDetailIntoHistory(article) {
    if (!article) {
      return;
    }
    state.historyArticles = uniqueHistoryItems([article, ...state.historyArticles]);
    if (article.draftVersions.length) {
      state.historyDrafts = uniqueHistoryItems([...article.draftVersions, ...state.historyDrafts]);
    }
    if (article.currentDraft) {
      state.historyDrafts = uniqueHistoryItems([article.currentDraft, ...state.historyDrafts]);
    }
  }

  function mergeDraftDetailIntoHistory(draft) {
    if (!draft) {
      return;
    }
    state.historyDrafts = uniqueHistoryItems([draft, ...state.historyDrafts]);
  }

  function renderHistoryArticleDetail(options = {}) {
    renderHistoryProjectCard(state.selectedHistoryProject, options.warning);
    renderHistoryArticleCard(state.selectedHistoryArticle, options.warning);
    const article = state.selectedHistoryArticle;
    const selectedDraft = state.selectedHistoryDraft;
    renderHistoryBriefCard(article?.brief || null);
    renderHistoryCurrentDraftCard(selectedDraft || article?.currentDraft || null);
    renderHistoryDraftVersionsCard(article?.draftVersions || filteredHistoryDrafts());
    renderHistorySourceSnapshotCard(article?.sourceSnapshot || null);
  }

  function renderHistoryProjectCard(project, warning) {
    if (!project) {
      fillArtifactCard(el.historyProjectCard, null, [], [], 'プロジェクトを選択すると、記事一覧と更新状況をここに表示します。');
      return;
    }
    fillArtifactCard(
      el.historyProjectCard,
      project.title || 'プロジェクト',
      [
        ['Project', project.id],
        ['Persona', project.personaId],
        ['Format', project.outputFormatId],
        ['Status', project.status],
        ['Updated', formatDateTime(project.updatedAt)],
      ],
      [
        ['記事', [`${project.articleCount ?? project.articles.length ?? 0}件`]],
        warning ? ['注意', [warning]] : null,
      ].filter(Boolean),
    );
  }

  function renderHistoryArticleCard(article, warning) {
    if (!article) {
      fillArtifactCard(el.historyArticleCard, null, [], warning ? [['注意', [warning]]] : [], '記事を選択すると、ブリーフ、下書き、参照ソースの要約をここに表示します。');
      return;
    }
    fillArtifactCard(
      el.historyArticleCard,
      article.title || '記事',
      [
        ['Article', article.id],
        ['Project', article.projectId],
        ['Persona', article.personaId],
        ['Format', article.outputFormatId],
        ['Status', article.status],
        ['Updated', formatDateTime(article.updatedAt)],
      ],
      [
        ['ブリーフ', [article.brief ? briefField(article.brief, 'theme', 'Theme') || '保存済み' : '未接続または未作成']],
        ['下書き', [`${article.draftVersions.length}件のバージョン${article.currentDraft ? ' / 現在版あり' : ''}`]],
        warning ? ['注意', [warning]] : null,
      ].filter(Boolean),
    );
  }

  function renderHistoryBriefCard(brief) {
    if (!brief) {
      fillArtifactCard(el.historyArticleBriefCard, null, [], [], '記事ブリーフはまだありません。詳細API接続後、テーマ、読者、含める内容を要約表示します。');
      return;
    }
    fillArtifactCard(
      el.historyArticleBriefCard,
      briefField(brief, 'theme', 'Theme') || '記事ブリーフ',
      [
        ['Persona', briefField(brief, 'persona_id', 'PersonaID')],
        ['Format', briefField(brief, 'output_format_id', 'OutputFormatID')],
        ['Style', briefField(brief, 'style_profile_id', 'StyleProfileID')],
      ],
      [
        ['読者', [briefField(brief, 'reader', 'Reader')]],
        ['冒頭の具体例', [briefField(brief, 'opening_episode', 'OpeningEpisode')]],
        ['必ず含めること', [briefField(brief, 'must_include', 'MustInclude')]],
        ['読後アクション', [briefField(brief, 'expected_reader_action', 'ExpectedReaderAction')]],
      ],
    );
  }

  function renderHistoryCurrentDraftCard(draft) {
    if (!draft) {
      fillArtifactCard(el.historyCurrentDraftCard, null, [], [], '現在の下書きはまだありません。下書き詳細API接続後、本文の冒頭と評価を表示します。');
      return;
    }
    fillArtifactCard(
      el.historyCurrentDraftCard,
      draft.title || '現在の下書き',
      [
        ['Draft', draft.id],
        ['Version', draft.version],
        ['Score', draft.score === undefined ? '' : Number(draft.score).toFixed(1)],
        ['Status', draft.status || draft.kind],
        ['Updated', formatDateTime(draft.updatedAt)],
      ],
      [
        ['本文要約', summarizeDraftLines(draft, 5)],
        draft.validationError ? ['検証メモ', [draft.validationError]] : null,
      ].filter(Boolean),
    );
  }

  function renderHistoryDraftVersionsCard(drafts) {
    const versions = arrayFrom(drafts).filter((draft) => draft.id || draft.markdown || draft.summary);
    if (!versions.length) {
      fillArtifactCard(el.historyDraftVersionsCard, null, [], [], '下書きバージョンはまだありません。保存済みバージョンがあると番号、評価、更新日時を一覧表示します。');
      return;
    }
    fillArtifactCard(
      el.historyDraftVersionsCard,
      '下書きバージョン',
      [['Versions', versions.length]],
      [['一覧', versions.slice(0, 8).map(historyDraftVersionLine)]],
    );
  }

  function renderHistorySourceSnapshotCard(snapshot) {
    const normalized = normalizeSourceSnapshot(snapshot);
    if (!normalized) {
      fillArtifactCard(el.historySourceSnapshotCard, null, [], [], 'ソーススナップショットはまだありません。接続後、参照記事や取得日時を表示します。');
      return;
    }
    fillArtifactCard(
      el.historySourceSnapshotCard,
      normalized.title || 'ソーススナップショット',
      [
        ['Fetched', formatDateTime(normalized.fetchedAt)],
        ['Articles', normalized.articles.length],
      ],
      [
        ['参照ソース', normalized.articles.length ? normalized.articles.slice(0, 6).map(sourceArticleLine) : [normalized.summary]],
      ],
    );
  }

  function fillArtifactCard(card, title, metaItems, sections, emptyText) {
    card.innerHTML = '';
    if (!title && !arrayFrom(sections).length) {
      card.className = 'artifact-card empty';
      card.textContent = emptyText;
      return;
    }
    card.className = 'artifact-card';
    if (title) {
      card.appendChild(createArtifactHeader(title, metaItems));
    }
    arrayFrom(sections).forEach(([sectionTitle, values]) => {
      const visibleValues = arrayFrom(values).filter((value) => String(value || '').trim());
      if (visibleValues.length) {
        card.appendChild(createArtifactSection(sectionTitle, visibleValues));
      }
    });
  }

  function normalizeSourceSnapshot(snapshot) {
    if (!snapshot) {
      return null;
    }
    if (Array.isArray(snapshot)) {
      return {
        title: 'ソーススナップショット',
        fetchedAt: '',
        summary: '',
        articles: snapshot,
      };
    }
    if (typeof snapshot === 'string') {
      return {
        title: 'ソーススナップショット',
        fetchedAt: '',
        summary: snapshot,
        articles: [],
      };
    }
    const articles = arrayFrom(snapshot.articles || snapshot.Articles || snapshot.sources || snapshot.Sources);
    return {
      title: snapshot.title || snapshot.Title || snapshot.source_selector || snapshot.sourceSelector || '',
      fetchedAt: snapshot.fetched_at || snapshot.fetchedAt || snapshot.created_at || snapshot.createdAt || '',
      summary: snapshot.summary || snapshot.Summary || snapshot.url || snapshot.URL || '',
      articles,
    };
  }

  function sourceArticleLine(article) {
    const title = article.title || article.Title || article.id || article.ID || '参照記事';
    const url = article.url || article.URL || '';
    const fetchedAt = formatDateTime(article.fetched_at || article.fetchedAt || article.at || article.At);
    return [title, url, fetchedAt].filter(Boolean).join(' / ');
  }

  function summarizeDraftLines(draft, limit) {
    return compactTextLines(draft.markdown || draft.summary || '', limit).map((line) => line.replace(/^#{1,6}\s+/, ''));
  }

  function historyDraftVersionLine(draft) {
    const title = draft.title || draft.id || '下書き';
    const version = draft.version ? `v${draft.version}` : draft.kind || '';
    const score = draft.score === undefined ? '' : `score ${Number(draft.score).toFixed(1)}`;
    const updatedAt = formatDateTime(draft.updatedAt || draft.createdAt);
    return [version, title, score, updatedAt].filter(Boolean).join(' / ');
  }

  function historyDraftResumeText(draft) {
    const score = draft.score === undefined ? '' : ` / style score ${Number(draft.score).toFixed(1)}`;
    const version = draft.version ? ` v${draft.version}` : '';
    return `保存済み下書き${version}をMarkdownエディタに反映しました${score}。`;
  }

  function markdownTitle(markdown) {
    const heading = String(markdown || '').split('\n').find((line) => line.trim().startsWith('# '));
    return heading ? heading.replace(/^#\s+/, '').trim() : '';
  }


  function applyPersonaDefaults(forceFormat) {
    const persona = currentPersona();
    if (!persona) {
      return;
    }
    if ((forceFormat || !el.formatSelect.value) && persona.default_format) {
      el.formatSelect.value = persona.default_format;
      config.mode.format = persona.default_format;
    }
    applyStyleSourceDefault(false);
  }

  function applyStyleSourceDefault(force) {
    const source = defaultStyleSourceSelector();
    if (!source) {
      return;
    }
    const current = el.username.value.trim();
    if (force || !current || isKnownPersonaSource(current)) {
      el.username.value = source;
    }
    el.username.placeholder = source;
  }

  function defaultStyleSourceSelector() {
    const persona = currentPersona();
    const format = currentFormat();
    if (!persona || !format) {
      return '';
    }
    const sources = persona.sources || [];
    const find = (kind) => sources.find((source) => source.kind === kind);
    let source = null;
    if (format.id === 'markdown_blog' || format.id === 'homepage_section') {
      source = find('github') || find('rss');
    } else if (format.id === 'zenn_article') {
      source = find('zenn');
    } else if (format.id === 'qiita_article') {
      source = find('qiita');
    } else if (format.id === 'note_article') {
      source = find('note');
    }
    source = source || sources[0];
    if (!source) {
      return '';
    }
    return source.ref ? `${source.kind}:${source.ref}` : `${source.kind}:${source.url || ''}`;
  }

  function isKnownPersonaSource(value) {
    const sources = (state.personas || []).flatMap((persona) => persona.sources || []);
    return sources.some((source) => {
      const refSelector = source.ref ? `${source.kind}:${source.ref}` : '';
      const urlSelector = source.url ? `${source.kind}:${source.url}` : '';
      return value === source.ref || value === source.url || value === refSelector || value === urlSelector;
    });
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

    const text = document.createElement('div');
    text.className = 'question-template-text';
    text.textContent = question.text;
    text.setAttribute('aria-label', 'テンプレート質問');

    const label = document.createElement('span');
    label.className = 'question-config-tag';
    label.textContent = isQuestionRequired(question) ? '必須' : '任意';

    row.append(text, label);
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

  function renderStyleGuideCard(data) {
    el.styleGuideCard.innerHTML = '';
    const markdown = styleGuideMarkdown(data);
    if (!data || !markdown) {
      el.styleGuideCard.className = 'artifact-card empty';
      el.styleGuideCard.textContent = '文体ガイドはまだありません。文体分析または保存済み履歴から選択してください。';
      return;
    }
    const normalized = normalizeHistoryStyle(data);
    el.styleGuideCard.className = 'artifact-card';
    el.styleGuideCard.appendChild(createArtifactHeader(
      normalized.title || '文体ガイド',
      [
        ['Profile', normalized.profileId || normalized.id],
        ['Guide', normalized.guideId],
        ['Articles', normalized.articleCount === undefined ? '' : String(normalized.articleCount)],
      ],
    ));
    const sections = markdownSectionsForCard(markdown);
    if (sections.length) {
      sections.slice(0, 5).forEach((section) => {
        el.styleGuideCard.appendChild(createArtifactSection(section.title, section.items));
      });
    } else {
      el.styleGuideCard.appendChild(createArtifactSection('要点', compactTextLines(markdown, 6)));
    }
  }

  function renderBriefCard(brief) {
    el.briefCard.innerHTML = '';
    if (!brief) {
      el.briefCard.className = 'artifact-card empty';
      el.briefCard.textContent = '記事ブリーフはまだありません。取材完了後、または保存済みセッション選択後に表示します。';
      return;
    }
    el.briefCard.className = 'artifact-card';
    const theme = briefField(brief, 'theme', 'Theme') || '記事ブリーフ';
    el.briefCard.appendChild(createArtifactHeader(
      theme,
      [
        ['Persona', briefField(brief, 'persona_id', 'PersonaID')],
        ['Format', briefField(brief, 'output_format_id', 'OutputFormatID')],
        ['Style', briefField(brief, 'style_profile_id', 'StyleProfileID')],
      ],
    ));
    [
      ['読者', briefField(brief, 'reader', 'Reader')],
      ['冒頭の具体例', briefField(brief, 'opening_episode', 'OpeningEpisode')],
      ['読後アクション', briefField(brief, 'expected_reader_action', 'ExpectedReaderAction')],
      ['必ず含めること', briefField(brief, 'must_include', 'MustInclude')],
      ['本人文脈', briefField(brief, 'personal_context', 'PersonalContext')],
      ['含めないこと', briefField(brief, 'exclusions', 'Exclusions')],
      ['構成と長さ', briefField(brief, 'target_length_structure', 'TargetLengthStructure')],
      ['トーンと立場', briefField(brief, 'tone_stance', 'ToneStance')],
    ].filter(([, value]) => String(value || '').trim()).forEach(([label, value]) => {
      el.briefCard.appendChild(createArtifactSection(label, [value]));
    });
    const customAnswers = brief.CustomAnswers || brief.custom_answers || brief.customAnswers || [];
    const deepDives = brief.DeepDives || brief.deep_dives || brief.deepDives || [];
    if (customAnswers.length) {
      el.briefCard.appendChild(createAnswerSection('追加回答', customAnswers));
    }
    if (deepDives.length) {
      el.briefCard.appendChild(createAnswerSection('深掘りメモ', deepDives));
    }
  }

  function createArtifactHeader(title, metaItems) {
    const header = document.createElement('div');
    header.className = 'artifact-card-header';
    const titleElement = document.createElement('strong');
    titleElement.textContent = title;
    const meta = document.createElement('div');
    meta.className = 'artifact-meta';
    metaItems.filter(([, value]) => value !== undefined && value !== null && String(value).trim()).forEach(([label, value]) => {
      const item = document.createElement('span');
      item.textContent = `${label}: ${value}`;
      meta.appendChild(item);
    });
    header.append(titleElement, meta);
    return header;
  }

  function createArtifactSection(title, values) {
    const section = document.createElement('section');
    section.className = 'artifact-section';
    const heading = document.createElement('h4');
    heading.textContent = title;
    section.appendChild(heading);
    const list = document.createElement('ul');
    values.filter((value) => String(value || '').trim()).forEach((value) => {
      const item = document.createElement('li');
      item.textContent = String(value).trim();
      list.appendChild(item);
    });
    section.appendChild(list);
    return section;
  }

  function createAnswerSection(title, answers) {
    return createArtifactSection(title, answers.map((answer) => {
      const questionId = answerValue(answer, 'question_id', 'QuestionID');
      const content = answerValue(answer, 'content', 'Content');
      const question = state.questionTextById[questionId] || questionId || '回答';
      return `${question}: ${content}`;
    }));
  }

  function markdownSectionsForCard(markdown) {
    const sections = [];
    let current = { title: '要点', items: [] };
    String(markdown || '').split('\n').forEach((line) => {
      const trimmed = line.trim();
      if (!trimmed) {
        return;
      }
      const heading = trimmed.match(/^#{1,4}\s+(.+)$/);
      if (heading) {
        if (current.items.length) {
          sections.push(current);
        }
        current = { title: heading[1].trim(), items: [] };
        return;
      }
      current.items.push(trimmed.replace(/^[-*]\s+/, ''));
    });
    if (current.items.length) {
      sections.push(current);
    }
    return sections.map((section) => ({
      title: section.title,
      items: section.items.slice(0, 6),
    }));
  }

  function compactTextLines(value, limit) {
    return String(value || '').split('\n').map((line) => line.trim()).filter(Boolean).slice(0, limit);
  }

  function styleGuideMarkdown(data = {}) {
    return data.guideMarkdown || data.guide_markdown || data.markdown || data.Markdown || data.guide?.markdown || data.guide?.Markdown || data.Guide?.Markdown || '';
  }

  function briefField(brief, snake, pascal) {
    if (!brief) {
      return '';
    }
    return brief[snake] ?? brief[toCamelCase(snake)] ?? brief[pascal] ?? '';
  }

  function toCamelCase(value) {
    return String(value || '').replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
  }

  function arrayFrom(value) {
    return Array.isArray(value) ? value : [];
  }

  function formatDateTime(value) {
    if (!value) {
      return '';
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return '';
    }
    return date.toLocaleString('ja-JP', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  function renderDraft(data) {
    const qualityGate = data.quality_gate || data.qualityGate || {};
    const evaluation = data.evaluation;
    const passed = evaluation?.Passed ?? evaluation?.passed;
    const comparison = evaluation?.Comparison ?? evaluation?.comparison;
    const failures = evaluation?.Failures ?? evaluation?.failures ?? [];
    const score = qualityGate.score ?? comparison?.score ?? comparison?.Score ?? 0;
    const runes = qualityGate.runes ?? qualityGate.draft?.runes ?? 0;
    const failedMetrics = qualityGate.failed_metrics || qualityGate.failedMetrics || [];
    const draftText = data.draft ?? qualityGate.draft?.text ?? qualityGate.draft_text ?? '';

    el.evaluationSummary.className = `evaluation ${passed ? 'passed' : 'failed'}`;
    el.evaluationSummary.innerHTML = `
      <strong>${passed ? 'PASS' : 'NEEDS REVIEW'}</strong>
      <span>style score: ${Number(score).toFixed(1)}</span>
      ${runes ? `<span>${Number(runes).toLocaleString()}字</span>` : ''}
      ${failedMetrics.length ? `<p>${failedMetrics.map(failedMetricSummary).join('<br>')}</p>` : ''}
      ${failures.length ? `<p>${failures.join('<br>')}</p>` : ''}
    `;
    renderVerification(data.verification || qualityGate.verification);
    el.markdownOutput.value = draftText;
    syncDraftEditor();
    el.draftResult.classList.remove('hidden');
    setActiveTab('preview');
  }

  function failedMetricSummary(metric) {
    const name = escapeHTML(metric.name || metric.Name || 'metric');
    const score = metric.score ?? metric.Score;
    const threshold = metric.threshold ?? metric.Threshold;
    const message = metric.message || metric.Message || '';
    if (message) {
      return escapeHTML(message);
    }
    if (metric.missing || metric.Missing) {
      return `${name} missing`;
    }
    if (score !== undefined && threshold !== undefined) {
      return `${name}: ${Number(score).toFixed(1)} / ${Number(threshold).toFixed(1)}`;
    }
    return name;
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

  function draftStatusText(statusOrData, elapsedMS = 0) {
    const data = typeof statusOrData === 'object' && statusOrData !== null
      ? statusOrData
      : { status: statusOrData, elapsed_ms: elapsedMS };
    const status = data.status;
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
    const elapsed = Math.max(0, Math.round(Number(data.elapsed_ms || elapsedMS || 0) / 1000));
    if (status === 'runtime_connected' && data.endpoint) {
      const model = data.model ? ` / ${data.model}` : '';
      return `${labels[status]}: ${data.endpoint}${model} (${elapsed}s)`;
    }
    return `${labels[status] || status} (${elapsed}s)`;
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
