package static_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

type staticContract struct {
	document *goquery.Document
	script   string
}

func TestHistoryUIContract(t *testing.T) {
	contract := loadStaticContract(t)

	for _, selector := range []string{
		"#history-persona-select",
		"#history-project-select",
		"#history-article-select",
		"#history-draft-select",
		"#history-style-select",
		"#history-session-select",
		"#refresh-history-btn",
		"#open-history-btn",
		"#clear-history-selection-btn",
		"#history-status",
		"#history-article-detail",
		"#history-project-card",
		"#history-article-card",
		"#history-article-brief-card",
		"#history-current-draft-card",
		"#history-draft-versions-card",
		"#history-source-snapshot-card",
		"#style-guide-card",
		"#brief-card",
	} {
		assertSelectorCount(t, contract.document, selector, 1)
	}

	openHistoryButton := contract.document.Find("#open-history-btn")
	if _, ok := openHistoryButton.Attr("disabled"); !ok {
		t.Fatalf("#open-history-btn should start disabled until a history item is selected")
	}

	assertScriptContains(t, contract.script, []string{
		"const historyEndpoint = '/api/workflow/artifacts'",
		"loadWorkflowHistory",
		"normalizeWorkflowHistory",
		"normalizeHistoryProject",
		"normalizeHistoryArticle",
		"normalizeHistoryDraft",
		"renderHistoryArticleDetail",
		"loadHistoryArticleDetail",
		"loadHistoryDraftDetail",
		"renderStyleGuideCard",
		"renderBriefCard",
		"requestJSON(`${historyEndpoint}?",
		"/api/projects/",
		"/api/articles/",
		"/api/drafts/",
		"/api/author-style/",
		"/api/brief-sessions/",
	})
}

func TestPersonaAuthoringContract(t *testing.T) {
	contract := loadStaticContract(t)

	for _, selector := range []string{
		"#add-persona-btn",
		"#edit-persona-btn",
		"#delete-persona-btn",
		"#add-persona-form",
		"#persona-form-title",
		"#persona-id-input",
		"#persona-display-name-input",
		"#persona-default-format-select",
		"#persona-description-input",
		"#persona-first-person-input",
		"#persona-source-kind-input",
		"#persona-source-ref-input",
		"#persona-source-url-input",
		"#save-persona-btn",
		"#cancel-persona-btn",
		"#persona-status",
	} {
		assertSelectorCount(t, contract.document, selector, 1)
	}
	if got := strings.TrimSpace(contract.document.Find("#add-persona-btn").Text()); got != "+ Add persona" {
		t.Fatalf("#add-persona-btn text = %q, want + Add persona", got)
	}
	if _, ok := contract.document.Find("#add-persona-form").Attr("class"); !ok {
		t.Fatalf("#add-persona-form should declare a class so it can start compact/hidden")
	}

	assertScriptContains(t, contract.script, []string{
		"el.addPersonaToggle.addEventListener('click', togglePersonaForm)",
		"el.editPersona.addEventListener('click', startPersonaEdit)",
		"el.deletePersona.addEventListener('click', deleteSelectedPersona)",
		"el.addPersonaForm.addEventListener('submit', createPersona)",
		"el.cancelPersona.addEventListener('click', hidePersonaForm)",
		"editing ? `/api/personas/${encodeURIComponent(personaId)}` : '/api/personas'",
		"method: 'POST'",
		"`/api/personas/${encodeURIComponent(personaId)}`",
		"method: editing ? 'PATCH' : 'POST'",
		"`/api/personas/${encodeURIComponent(persona.id)}`",
		"method: 'DELETE'",
		"populatePersonaSelect()",
		"populateHistoryPersonaSelect()",
		"書き手追加APIはまだ接続されていません。バックエンド実装後に保存できます。",
		"書き手更新APIはまだ接続されていません。バックエンド実装後に保存できます。",
		"書き手削除APIはまだ接続されていません。バックエンド実装後に削除できます。",
	})
	assertFunctionContains(t, contract.script, "startPersonaEdit", []string{
		"state.personaFormMode = 'edit'",
		"state.editingPersonaId = persona.id",
		"el.personaFormTitle.textContent = '書き手を編集'",
		"fillPersonaForm(persona)",
	})
	assertFunctionContains(t, contract.script, "personaPayloadFromForm", []string{
		"id: slugifyPersonaId(el.personaIdInput.value || el.personaNameInput.value)",
		"display_name: el.personaNameInput.value.trim()",
		"description: el.personaDescriptionInput.value.trim()",
		"default_format: el.personaDefaultFormatSelect.value || currentFormatId()",
		"payload.voice_notes = { first_person: voice }",
		"payload.sources = [{ kind: sourceKind || 'manual', ref: sourceRef, url: sourceURL }]",
	})
	assertFunctionContains(t, contract.script, "upsertPersona", []string{
		"state.personas = [",
		"...state.personas.filter((item) => item.id !== persona.id)",
	})
	assertFunctionContains(t, contract.script, "deleteSelectedPersona", []string{
		"window.confirm",
		"state.personas = state.personas.filter((item) => item.id !== persona.id)",
		"config.mode.persona = nextPersona?.id || ''",
		"await loadWorkflowHistory()",
	})
}

func TestModelSelectorConfigContract(t *testing.T) {
	contract := loadStaticContract(t)

	for selector, label := range map[string]string{
		"#style-model":  "文体分析モデル",
		"#brief-model":  "深掘り質問モデル",
		"#draft-model":  "下書き生成モデル",
		"#verify-model": "最終検証モデル",
	} {
		assertSelectorCount(t, contract.document, selector, 1)
		if got := strings.TrimSpace(contract.document.Find(`label[for="` + strings.TrimPrefix(selector, "#") + `"]`).Text()); got != label {
			t.Fatalf("label for %s = %q, want %q", selector, got, label)
		}
	}

	assertScriptContains(t, contract.script, []string{
		"el.styleModel.addEventListener('change', saveModelConfig)",
		"el.briefModel.addEventListener('change', saveModelConfig)",
		"el.draftModel.addEventListener('change', saveModelConfig)",
		"el.verifyModel.addEventListener('change', saveModelConfig)",
	})
	assertFunctionContains(t, contract.script, "populateModelSelects", []string{
		"const available = models.length ? models : ['gemma4:31b']",
		"style: config.models.style || 'gemma4:latest'",
		"brief: config.models.brief || 'gemma4:e2b'",
		"draft: config.models.draft || 'gemma4:31b'",
		"verify: config.models.verify || 'gemma4:latest'",
		"setOptions(el.styleModel, available, defaults.style)",
		"setOptions(el.briefModel, available, defaults.brief)",
		"setOptions(el.draftModel, available, defaults.draft)",
		"setOptions(el.verifyModel, available, defaults.verify)",
		"saveModelConfig()",
	})
	assertFunctionContains(t, contract.script, "saveModelConfig", []string{
		"config.models = {",
		"style: el.styleModel.value",
		"brief: el.briefModel.value",
		"draft: el.draftModel.value",
		"verify: el.verifyModel.value",
		"saveConfig()",
	})
	assertFunctionContains(t, contract.script, "loadConfig", []string{
		"models: { style: 'gemma4:e2b', brief: 'qwen3.6:27b', draft: 'gemma4:31b', verify: 'gemma4:latest' }",
		"localStorage.getItem(configStorageKey)",
		"models: { ...fallback.models, ...(saved.models || {}) }",
	})
}

func TestQuestionCRUDPersistenceContract(t *testing.T) {
	contract := loadStaticContract(t)

	for _, selector := range []string{
		"#question-config-list",
		"#add-question-btn",
		"#reset-questions-btn",
	} {
		assertSelectorCount(t, contract.document, selector, 1)
	}
	if got := strings.TrimSpace(contract.document.Find("#add-question-btn").Text()); got != "質問を追加" {
		t.Fatalf("#add-question-btn text = %q, want 質問を追加", got)
	}
	if got := strings.TrimSpace(contract.document.Find("#reset-questions-btn").Text()); got != "初期値に戻す" {
		t.Fatalf("#reset-questions-btn text = %q, want 初期値に戻す", got)
	}

	assertScriptContains(t, contract.script, []string{
		"el.addQuestion.addEventListener('click', addQuestion)",
		"el.resetQuestions.addEventListener('click', resetQuestions)",
		"const configStorageKey = 'note-maker-config-v1'",
	})
	assertFunctionContains(t, contract.script, "createCustomQuestionRow", []string{
		"input.setAttribute('aria-label', '追加質問')",
		"input.addEventListener('input'",
		"config.customQuestions[index].text = input.value",
		"saveConfig()",
		"remove.textContent = '削除'",
		"config.customQuestions.splice(index, 1)",
		"renderQuestionConfig()",
	})
	assertFunctionContains(t, contract.script, "addQuestion", []string{
		"config.customQuestions.push",
		"id: `custom_${Date.now()}`",
		"text: '追加で聞きたい質問を入力してください'",
		"target_field: 'custom'",
		"saveConfig()",
		"renderQuestionConfig()",
	})
	assertFunctionContains(t, contract.script, "resetQuestions", []string{
		"config.customQuestions = []",
		"saveConfig()",
		"loadQuestionTemplate()",
	})
	assertFunctionContains(t, contract.script, "loadConfig", []string{
		"saved.customQuestions || saved.custom_questions || migrateLegacyQuestions(saved.questions)",
		"customQuestions: normalizeQuestionList(savedCustomQuestions)",
	})
	assertFunctionContains(t, contract.script, "currentQuestions", []string{
		"const templateIds = new Set",
		"const templateTexts = new Set",
		"normalizeQuestionList(config.customQuestions)",
		"!templateIds.has(question.id)",
		"!templateTexts.has(question.text)",
	})
}

func TestHistoryOpenControlsApplyReadableCardsContract(t *testing.T) {
	contract := loadStaticContract(t)

	assertScriptContains(t, contract.script, []string{
		"el.historyPersonaSelect.addEventListener('change', loadWorkflowHistory)",
		"el.historyStyleSelect.addEventListener('change', selectHistoryStyle)",
		"el.historySessionSelect.addEventListener('change', selectHistorySession)",
		"el.refreshHistory.addEventListener('click', loadWorkflowHistory)",
		"el.openHistory.addEventListener('click', openSelectedHistory)",
		"el.clearHistorySelection.addEventListener('click', clearHistorySelection)",
	})
	assertFunctionContains(t, contract.script, "loadWorkflowHistory", []string{
		"state.historyLoading = true",
		"state.selectedHistoryStyle = null",
		"state.selectedHistorySession = null",
		"fetchWorkflowHistoryIndex({ personaId, formatId })",
		"normalizeWorkflowHistory(data, personaId, formatId)",
		"renderHistoryPicker()",
	})
	assertFunctionContains(t, contract.script, "renderHistoryPicker", []string{
		"renderHistoryOptions(el.historyStyleSelect, state.historyStyles, '文体ガイドを選択')",
		"renderHistoryOptions(el.historySessionSelect, state.historySessions, '取材セッションを選択')",
		"el.openHistory.disabled = state.historyLoading || !historySelectionReady()",
		"この書き手と出力先の保存済み履歴はまだありません。",
		"選択できます。",
	})
	assertFunctionContains(t, contract.script, "openSelectedHistory", []string{
		"loadHistoryStyleDetail",
		"loadHistorySessionDetail",
		"applyHistoryStyle(styleForSession)",
		"await applyHistorySession(session)",
		"選択した履歴を現在の作業状態に反映しました。",
	})
	assertFunctionContains(t, contract.script, "applyHistorySession", []string{
		"config.mode.persona = data.personaId",
		"config.mode.format = data.outputFormatId",
		"await loadQuestionTemplate()",
		"state.answers = data.answers || []",
		"rememberQuestions(data.questions || state.templateQuestions)",
		"renderTranscript",
		"renderBriefCard(data.brief)",
		"el.generateDraft.disabled = !state.profileId",
	})
}

func TestHistoryProjectArticleDraftAdapterContract(t *testing.T) {
	contract := loadStaticContract(t)

	assertFunctionContains(t, contract.script, "normalizeWorkflowHistory", []string{
		"const projectValues = arrayFrom(source.projects || source.Projects)",
		"...arrayFrom(source.articles || source.Articles)",
		"...arrayFrom(source.article_history || source.articleHistory)",
		"...arrayFrom(source.drafts || source.Drafts)",
		"...arrayFrom(source.draft_versions || source.draftVersions)",
		"projectValues.flatMap",
		"project.articles || project.Articles",
		"article.drafts || article.Drafts || article.draft_versions || article.draftVersions",
		"article.current_draft || article.currentDraft || article.draft || article.Draft",
		"projects: uniqueHistoryItems(projectValues.map(normalizeHistoryProject)",
		"articles: uniqueHistoryItems(articleValues.map(normalizeHistoryArticle)",
		"drafts: uniqueHistoryItems(draftValues.map(normalizeHistoryDraft)",
	})
	assertFunctionContains(t, contract.script, "normalizeHistoryProject", []string{
		"const project = item.project || item.Project || {}",
		"project.id || project.ID",
		"project.title || project.Title || project.name || project.Name",
		"project.persona_id || project.PersonaID",
		"project.output_format_id || project.OutputFormatID",
		"item.articles || item.Articles || project.articles || project.Articles",
		"normalizeHistoryArticle({ project_id: id, ...article })",
	})
	assertFunctionContains(t, contract.script, "normalizeHistoryArticle", []string{
		"const article = item.article || item.Article || {}",
		"article.id || article.ID",
		"article.title || article.Title",
		"item.brief || item.Brief || item.article_brief || item.articleBrief || article.brief || article.Brief",
		"item.current_draft || item.currentDraft || item.draft || item.Draft || article.current_draft || article.CurrentDraft",
		"item.draft_versions || item.draftVersions || item.drafts || item.Drafts || article.draft_versions || article.DraftVersions || article.drafts || article.Drafts",
		"article.source_snapshot || article.SourceSnapshot",
	})
	assertFunctionContains(t, contract.script, "normalizeHistoryDraft", []string{
		"const draft = item.draft || item.Draft || {}",
		"draft.id || draft.ID",
		"draft.article_id || draft.ArticleID",
		"draft.session_id || draft.SessionID",
		"draft.style_profile_id || draft.StyleProfileID",
		"draft.markdown || draft.Markdown || draft.text || draft.Text",
		"draft.score ?? draft.Score",
	})
}

func TestHistoryWrappedDetailResponseContract(t *testing.T) {
	contract := loadStaticContract(t)

	assertFunctionContains(t, contract.script, "loadHistoryProjectDetail", []string{
		"`/api/projects/${encodeURIComponent(normalized.id)}`",
		"`/api/history/projects/${encodeURIComponent(normalized.id)}`",
		"return normalizeHistoryProject({ ...item, ...data })",
	})
	assertFunctionContains(t, contract.script, "loadHistoryArticleDetail", []string{
		"`/api/articles/${encodeURIComponent(normalized.id)}`",
		"`/api/projects/${encodeURIComponent(normalized.projectId)}/articles/${encodeURIComponent(normalized.id)}`",
		"`/api/history/articles/${encodeURIComponent(normalized.id)}`",
		"const detail = data && !data.article && !data.Article && (data.brief || data.Brief || data.theme || data.Theme)",
		"return normalizeHistoryArticle({ ...item, ...detail })",
	})
	assertFunctionContains(t, contract.script, "loadHistoryDraftDetail", []string{
		"`/api/drafts/${encodeURIComponent(normalized.id)}`",
		"`/api/history/drafts/${encodeURIComponent(normalized.id)}`",
		"return normalizeHistoryDraft({ ...item, ...data })",
	})
	assertFunctionContains(t, contract.script, "requestFirstJSON", []string{
		"for (const url of urls.filter(Boolean))",
		"return await requestJSON(url)",
		"throw lastError || new Error('履歴詳細APIが見つかりません')",
	})
}

func TestArtifactCardsReadableContract(t *testing.T) {
	contract := loadStaticContract(t)

	for _, selector := range []string{
		"#style-result #style-guide-card",
		"#style-result .artifact-raw #guide-preview",
		"#brief-result #brief-card",
		"#brief-result .artifact-raw #brief-preview",
	} {
		assertSelectorCount(t, contract.document, selector, 1)
	}

	assertFunctionContains(t, contract.script, "renderStyleGuideCard", []string{
		"el.styleGuideCard.className = 'artifact-card empty'",
		"文体ガイドはまだありません。",
		"createArtifactHeader",
		"['Profile', normalized.profileId || normalized.id]",
		"['Guide', normalized.guideId]",
		"['Articles', normalized.articleCount === undefined ? '' : String(normalized.articleCount)]",
		"markdownSectionsForCard(markdown)",
		"createArtifactSection",
	})
	assertFunctionContains(t, contract.script, "renderBriefCard", []string{
		"el.briefCard.className = 'artifact-card empty'",
		"記事ブリーフはまだありません。",
		"createArtifactHeader",
		"['Persona', briefField(brief, 'persona_id', 'PersonaID')]",
		"['Format', briefField(brief, 'output_format_id', 'OutputFormatID')]",
		"['Style', briefField(brief, 'style_profile_id', 'StyleProfileID')]",
		"['読者', briefField(brief, 'reader', 'Reader')]",
		"['必ず含めること', briefField(brief, 'must_include', 'MustInclude')]",
		"createAnswerSection('追加回答', customAnswers)",
		"createAnswerSection('深掘りメモ', deepDives)",
	})
	assertFunctionContains(t, contract.script, "createArtifactHeader", []string{
		"header.className = 'artifact-card-header'",
		"titleElement.textContent = title",
		"meta.className = 'artifact-meta'",
	})
	assertFunctionContains(t, contract.script, "createAnswerSection", []string{
		"state.questionTextById[questionId]",
		"return `${question}: ${content}`",
	})
}

func TestArtifactCardEditContract(t *testing.T) {
	contract := loadStaticContract(t)

	assertScriptContains(t, contract.script, []string{
		"styleEditMode: false",
		"briefEditMode: false",
		"briefVersionsVisible: false",
		"'edit-brief-btn'",
		"'show-brief-versions-btn'",
		"'edit-style-guide-btn'",
		"PATCH",
		"`/api/author-style/${encodeURIComponent(styleId)}`",
		"`/api/briefs/${encodeURIComponent(sessionId)}`",
		"`/api/briefs/${encodeURIComponent(sessionId)}/versions`",
		"additiveEndpointStatus(error, '文体ガイド編集APIはまだ接続されていません。内容は保存されませんでした。')",
		"additiveEndpointStatus(error, '記事ブリーフ編集APIはまだ接続されていません。内容は保存されませんでした。')",
	})
	assertFunctionContains(t, contract.script, "renderStyleGuideCard", []string{
		"state.currentStyleArtifact = normalized",
		"if (state.styleEditMode)",
		"renderStyleGuideEditForm(normalized)",
		"createCardEditButton('文体ガイドを編集'",
		"appendArtifactStatus(el.styleGuideCard, state.styleEditStatus, state.styleEditStatusType)",
	})
	assertFunctionContains(t, contract.script, "renderBriefCard", []string{
		"if (state.briefEditMode)",
		"renderBriefEditForm(brief)",
		"createBriefCardActions(brief)",
		"appendArtifactStatus(el.briefCard, state.briefEditStatus, state.briefEditStatusType)",
		"renderBriefVersionHistory(brief)",
	})
	assertFunctionContains(t, contract.script, "createBriefCardActions", []string{
		"createCardEditButton('記事ブリーフを編集'",
		"createCardActionButton('履歴', '記事ブリーフのバージョン履歴を表示'",
	})
	assertFunctionContains(t, contract.script, "toggleBriefVersions", []string{
		"`/api/briefs/${encodeURIComponent(sessionId)}/versions`",
		"normalizeBriefVersions(data)",
		"briefVersionErrorMessage(error)",
	})
	assertFunctionContains(t, contract.script, "renderBriefVersionHistory", []string{
		"section.id = 'brief-version-history'",
		"state.briefVersionsLoading",
		"state.briefVersionsError",
		"briefVersionLine(version)",
	})
	assertFunctionContains(t, contract.script, "renderBriefEditForm", []string{
		"form.id = 'brief-edit-form'",
		"['theme', 'テーマ', 'input']",
		"['reader', '読者', 'textarea']",
		"['must_include', '必ず含めること', 'textarea']",
		"save.id = 'save-brief-edit-btn'",
		"cancel.id = 'cancel-brief-edit-btn'",
		"form.addEventListener('submit', (event) => saveBriefEdit(event, brief))",
	})
	assertFunctionContains(t, contract.script, "saveBriefEdit", []string{
		"method: 'PATCH'",
		"body: { fields }",
		"state.completedBrief = { ...originalBrief, ...updatedBrief }",
		"el.briefPreview.textContent = JSON.stringify(state.completedBrief, null, 2)",
		"setBriefEditStatus('記事ブリーフを保存しました。', 'success')",
	})
	assertFunctionContains(t, contract.script, "saveStyleGuideEdit", []string{
		"method: 'PATCH'",
		"guide_markdown: markdown",
		"setStyleEditStatus('文体ガイドを保存しました。', 'success')",
		"el.guidePreview.textContent = styleGuideMarkdown(updated)",
	})
	assertFunctionContains(t, contract.script, "renderStyleGuideEditForm", []string{
		"form.id = 'style-guide-edit-form'",
		"'style-guide-markdown-input'",
		"save.id = 'save-style-guide-edit-btn'",
		"cancel.id = 'cancel-style-guide-edit-btn'",
	})
	assertFunctionContains(t, contract.script, "artifactStatusIdForContainer", []string{
		"return 'brief-edit-status'",
		"return 'style-guide-edit-status'",
	})
}

func loadStaticContract(t *testing.T) staticContract {
	t.Helper()

	index, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	script, err := os.ReadFile("js/script.js")
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(index))
	if err != nil {
		t.Fatalf("parse index: %v", err)
	}
	return staticContract{document: document, script: string(script)}
}

func assertSelectorCount(t *testing.T, document *goquery.Document, selector string, want int) {
	t.Helper()
	if got := document.Find(selector).Length(); got != want {
		t.Fatalf("selector %s count = %d, want %d", selector, got, want)
	}
}

func assertScriptContains(t *testing.T, script string, wants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q", want)
		}
	}
}

func assertFunctionContains(t *testing.T, script, name string, wants []string) {
	t.Helper()
	body, ok := functionBody(script, name)
	if !ok {
		t.Fatalf("script missing function %s", name)
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Fatalf("function %s missing %q", name, want)
		}
	}
}

func functionBody(script, name string) (string, bool) {
	signature := "function " + name + "("
	start := strings.Index(script, signature)
	if start < 0 {
		return "", false
	}
	open := functionBodyOpen(script, start+len("function "+name))
	if open < 0 {
		return "", false
	}
	depth := 0
	for index := open; index < len(script); index++ {
		switch script[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return script[open : index+1], true
			}
		}
	}
	return "", false
}

func functionBodyOpen(script string, parenStart int) int {
	if parenStart >= len(script) || script[parenStart] != '(' {
		return -1
	}
	depth := 0
	for index := parenStart; index < len(script); index++ {
		switch script[index] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				for next := index + 1; next < len(script); next++ {
					if script[next] == '{' {
						return next
					}
					if script[next] != ' ' && script[next] != '\n' && script[next] != '\t' && script[next] != '\r' {
						return -1
					}
				}
				return -1
			}
		}
	}
	return -1
}
