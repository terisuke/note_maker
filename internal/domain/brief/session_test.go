package brief

import "testing"

func TestFixedQuestionsAreDeterministic(t *testing.T) {
	questions := FixedQuestions()
	wantIDs := []string{
		QuestionIDTheme,
		QuestionIDOpeningEpisode,
		QuestionIDReader,
		QuestionIDExpectedReaderAction,
		QuestionIDMustInclude,
		QuestionIDExclusions,
		QuestionIDTargetLengthStructure,
		QuestionIDToneStance,
	}
	wantText := []string{
		"What is the article's core theme?",
		"What concrete experience or episode should open the article?",
		"Who is the reader?",
		"What should the reader feel or do after reading?",
		"What must be included?",
		"What must be excluded?",
		"What target length and structure should be used?",
		"Should the article be more introspective, technical, narrative, or practical?",
	}
	if len(questions) != len(wantIDs) {
		t.Fatalf("question count = %d, want %d", len(questions), len(wantIDs))
	}
	for i, question := range questions {
		if question.ID != wantIDs[i] {
			t.Fatalf("question %d id = %q, want %q", i, question.ID, wantIDs[i])
		}
		if question.Text != wantText[i] {
			t.Fatalf("question %d text = %q, want %q", i, question.Text, wantText[i])
		}
		if question.FlowType != QuestionFlowMain {
			t.Fatalf("question %d flow = %q, want %q", i, question.FlowType, QuestionFlowMain)
		}
	}
}

func TestSelectDeepDiveTargetsUsesHighValueDeterministicOrder(t *testing.T) {
	session := answeredFixedSession(t, map[string]string{
		QuestionIDOpeningEpisode:        "The article opens on a late night debugging session.",
		QuestionIDMustInclude:           "Include local LLM latency numbers.",
		QuestionIDExpectedReaderAction:  "Try the workflow on one small article.",
		QuestionIDToneStance:            "Practical but reflective.",
		QuestionIDTargetLengthStructure: "1200 words",
	})

	targets := session.SelectDeepDiveTargets()
	want := []string{
		QuestionIDOpeningEpisode,
		QuestionIDMustInclude,
		QuestionIDExpectedReaderAction,
		QuestionIDToneStance,
	}
	if len(targets) != len(want) {
		t.Fatalf("target count = %d, want %d", len(targets), len(want))
	}
	for i, target := range targets {
		if target.ID != want[i] {
			t.Fatalf("target %d = %q, want %q", i, target.ID, want[i])
		}
	}
}

func TestFollowUpCountLimits(t *testing.T) {
	session := answeredFixedSession(t, nil)

	var asked []ArticleQuestion
	for i := 0; i < MaxTotalFollowUps; i++ {
		question, ok := session.CurrentQuestion()
		if !ok {
			t.Fatalf("expected follow-up %d", i+1)
		}
		if question.FlowType != QuestionFlowDeepDiveFollowUp {
			t.Fatalf("flow = %q, want %q", question.FlowType, QuestionFlowDeepDiveFollowUp)
		}
		asked = append(asked, question)
		if _, err := session.RecordAnswer("A concrete detail for the follow-up."); err != nil {
			t.Fatalf("record follow-up %d: %v", i+1, err)
		}
		if session.DeepDiveAnswerCountForTarget(question.TargetQuestionID) > MaxFollowUpsPerTarget {
			t.Fatalf("target %q exceeded per-target cap", question.TargetQuestionID)
		}
	}
	if session.DeepDiveAnswerCount() != MaxTotalFollowUps {
		t.Fatalf("deep-dive count = %d, want %d", session.DeepDiveAnswerCount(), MaxTotalFollowUps)
	}
	if _, ok := session.CurrentQuestion(); ok {
		t.Fatal("expected no question after total follow-up cap")
	}
	if asked[0].TargetQuestionID != QuestionIDOpeningEpisode || asked[1].TargetQuestionID != QuestionIDOpeningEpisode {
		t.Fatalf("first target was not capped after two follow-ups: %#v", asked[:2])
	}
	if asked[2].TargetQuestionID != QuestionIDMustInclude || asked[3].TargetQuestionID != QuestionIDMustInclude {
		t.Fatalf("second target did not receive remaining follow-ups: %#v", asked[2:])
	}
}

func TestCompletionRules(t *testing.T) {
	session, err := NewArticleBriefSession("session-1", "style-1")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if session.CanComplete() {
		t.Fatal("new session should not be complete")
	}

	session = answeredFixedSession(t, nil)
	if session.CanComplete() {
		t.Fatal("session should require at least one deep dive when high-value targets exist")
	}

	question, ok := session.CurrentQuestion()
	if !ok {
		t.Fatal("expected deep-dive question")
	}
	if _, err := session.RecordAnswer("The opening emotion is a mix of doubt and relief."); err != nil {
		t.Fatalf("record deep dive: %v", err)
	}
	if question.TargetQuestionID != QuestionIDOpeningEpisode {
		t.Fatalf("deep-dive target = %q, want %q", question.TargetQuestionID, QuestionIDOpeningEpisode)
	}
	if !session.CanComplete() {
		t.Fatal("session should complete after required fields and one deep dive")
	}

	skipped := answeredFixedSession(t, nil)
	skipped.MarkDeepDiveSkipped()
	if !skipped.CanComplete() {
		t.Fatal("explicit deep-dive skip should allow completion")
	}
}

func TestAssembleBriefIncludesFixedAndDeepDiveAnswers(t *testing.T) {
	session := answeredFixedSession(t, map[string]string{
		QuestionIDTargetLengthStructure: "",
		QuestionIDExclusions:            "Do not mention hosted SaaS.",
	})
	if _, err := session.RecordAnswer("Show the screen going from timeout to a usable draft."); err != nil {
		t.Fatalf("record deep dive: %v", err)
	}

	articleBrief, err := session.Complete()
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if articleBrief.StyleProfileID != "style-1" {
		t.Fatalf("style profile id = %q", articleBrief.StyleProfileID)
	}
	if articleBrief.Theme != "Local article generation with a small deterministic workflow." {
		t.Fatalf("theme = %q", articleBrief.Theme)
	}
	if articleBrief.TargetLengthStructure != DefaultTargetLengthStructure {
		t.Fatalf("target length default = %q, want %q", articleBrief.TargetLengthStructure, DefaultTargetLengthStructure)
	}
	if articleBrief.Exclusions != "Do not mention hosted SaaS." {
		t.Fatalf("exclusions = %q", articleBrief.Exclusions)
	}
	if len(articleBrief.DeepDives) != 1 {
		t.Fatalf("deep dives = %d, want 1", len(articleBrief.DeepDives))
	}
	if articleBrief.DeepDives[0].TargetQuestionID != QuestionIDOpeningEpisode {
		t.Fatalf("deep-dive target = %q", articleBrief.DeepDives[0].TargetQuestionID)
	}
}

func TestGeneratedFollowUpQuestionRulesRejectBinaryQuestions(t *testing.T) {
	disallowed := []string{
		"Do you want the article to be practical?",
		"Should it be technical or narrative?",
		"Is the reader a beginner?",
	}
	for _, text := range disallowed {
		if IsAllowedFollowUpQuestion(text) {
			t.Fatalf("expected %q to be rejected", text)
		}
	}
	if !IsAllowedFollowUpQuestion("What concrete scene should make that point memorable?") {
		t.Fatal("expected open-ended follow-up to be allowed")
	}
}

func answeredFixedSession(t *testing.T, overrides map[string]string) ArticleBriefSession {
	t.Helper()
	session, err := NewArticleBriefSession("session-1", "style-1")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	answers := map[string]string{
		QuestionIDTheme:                 "Local article generation with a small deterministic workflow.",
		QuestionIDOpeningEpisode:        "Open with a failed local LLM run that timed out while drafting.",
		QuestionIDReader:                "Solo developers who write note.com articles with local tools.",
		QuestionIDExpectedReaderAction:  "They should try a three-phase workflow before drafting.",
		QuestionIDMustInclude:           "Mention style analysis, brief interviews, and final draft checks.",
		QuestionIDExclusions:            "Avoid cloud-only assumptions.",
		QuestionIDTargetLengthStructure: "1800 words with three sections.",
		QuestionIDToneStance:            "Practical and introspective.",
	}
	for key, value := range overrides {
		answers[key] = value
	}
	for _, question := range FixedQuestions() {
		if _, err := session.RecordAnswer(answers[question.ID]); err != nil {
			t.Fatalf("answer %s: %v", question.ID, err)
		}
	}
	return session
}
