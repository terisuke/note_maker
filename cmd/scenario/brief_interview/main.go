package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	briefapp "github.com/teradakousuke/note_maker/internal/application/brief"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	"github.com/teradakousuke/note_maker/internal/infrastructure/llamacpp"
)

const defaultOutputDir = "tmp/brief_interview"

func main() {
	outputDir := envOrDefault("SCENARIO_OUTPUT_DIR", defaultOutputDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fatalf("create output dir: %v", err)
	}

	started := time.Now()
	styleProfileID := envOrDefault("STYLE_PROFILE_ID", "asp_scenario")
	personaID := envOrDefault("SCENARIO_PERSONA_ID", "terisuke")
	outputFormatID := envOrDefault("SCENARIO_OUTPUT_FORMAT_ID", "note_article")
	variant := envOrDefault("SCENARIO_BRIEF_VARIANT", "note_reflection")
	service := briefapp.NewInterviewService(scenarioFollowUpGenerator{})
	result, err := service.StartSession(briefapp.StartSessionInput{
		SessionID:      "brief_scenario",
		StyleProfileID: styleProfileID,
		PersonaID:      personaID,
		OutputFormatID: outputFormatID,
	})
	if err != nil {
		fatalf("start session: %v", err)
	}

	answers := scenarioAnswers(variant)
	for !result.Completed && result.NextQuestion != nil && result.NextQuestion.FlowType == briefdomain.QuestionFlowMain {
		answer, ok := answers[result.NextQuestion.ID]
		if !ok {
			answer = "なし"
		}
		result, err = service.Answer(context.Background(), result.Session, answer)
		if err != nil {
			fatalf("answer fixed question: %v", err)
		}
	}

	for !result.Completed {
		if result.NextQuestion == nil {
			fatalf("session not completed and no next question")
		}
		deepAnswer := scriptedDeepDiveAnswer(*result.NextQuestion)
		result, err = service.Answer(context.Background(), result.Session, deepAnswer)
		if err != nil {
			fatalf("answer deep dive: %v", err)
		}
	}
	if result.Brief == nil {
		brief := result.Session.AssembleBrief()
		result.Brief = &brief
	}

	writeJSON(filepath.Join(outputDir, "session.json"), result.Session)
	writeJSON(filepath.Join(outputDir, "brief.json"), result.Brief)

	fmt.Printf("brief interview scenario completed\n")
	fmt.Printf("session_id=%s\n", result.Session.ID)
	fmt.Printf("variant=%s\n", variant)
	fmt.Printf("persona_id=%s\n", result.Session.PersonaID)
	fmt.Printf("output_format_id=%s\n", result.Session.OutputFormatID)
	fmt.Printf("answers=%d\n", len(result.Session.Answers))
	fmt.Printf("deep_dives=%d\n", len(result.Brief.DeepDives))
	fmt.Printf("elapsed_seconds=%.2f\n", time.Since(started).Seconds())
	fmt.Printf("brief=%s\n", filepath.Join(outputDir, "brief.json"))
}

func scenarioAnswers(variant string) map[string]string {
	if variant == "cor_blog" {
		return map[string]string{
			briefdomain.QuestionIDTheme:                 "Evo X2を中心にしたローカルLLM推論基盤を、会社ブログとして技術知見と運用判断の両方から共有する",
			briefdomain.QuestionIDOpeningEpisode:        "CIは通っているのに、実際の推論経路がローカルに落ちると開発機を占有してしまう、という運用上の痛みから始めたい",
			briefdomain.QuestionIDReader:                "Cor.incの社員、AI実装に関わる開発者、ローカル推論基盤を業務導入したい技術責任者",
			briefdomain.QuestionIDReaderProblem:         "検証経路が属人化し、CIの緑と実運用の安心感がつながっていない",
			briefdomain.QuestionIDExpectedReaderAction:  "推論基盤は速さだけでなく、誰が使っても同じ経路で検証できることが重要だと理解してほしい",
			briefdomain.QuestionIDKeyTakeaway:           "速度、品質、再現性を同じシナリオ指標で見ることで、モデル選定をチームの知見に変えられる",
			briefdomain.QuestionIDMustInclude:           "Tailscale VPN越しのOpenAI互換APIを主経路にすること、Ollamaとllama.cppの役割分担、シナリオごとの秒数・スコア・文字数を記録すること、失敗時にローカルfallbackを最終手段として扱うこと",
			briefdomain.QuestionIDConcreteExample:       "Evo X2 primary、llama.cpp fallback、local fallbackを同じbrief/draftシナリオで比較し、score・keyword_overlap・first_chunk_msを残す",
			briefdomain.QuestionIDEvidence:              "scenario_passed、style score、keyword_overlap、runes、verification、elapsed_seconds、first_chunk_msを検証ログとして使う",
			briefdomain.QuestionIDPersonalContext:       "自分たちはAIを道具として使うだけではなく、社員が安心して試せる検証基盤を会社として育てたい。速度が出ても再現性がなければチームの知見にならない、というビジョンを共有したい",
			briefdomain.QuestionIDExclusions:            "単なるベンチマーク自慢、特定モデル礼賛、SSH前提の属人的な運用、根拠のないコスト削減表現",
			briefdomain.QuestionIDTargetLengthStructure: "2400字前後。導入、問題設定、設計判断、実装内容、検証結果、今後の課題、社員へのメッセージで構成する",
			briefdomain.QuestionIDToneStance:            "会社ブログとして明確で実務的に書く。断定はするが、検証結果と意思決定の背景を必ずセットで説明する",
			briefdomain.QuestionIDTitleKeywords:         "Evo X2、ローカルLLM、検証基盤、Tailscale、fallback",
		}
	}
	return map[string]string{
		briefdomain.QuestionIDTheme:                 "ローカルLLMで自分の過去記事を読み直し、文体ではなく思想まで再現できるのかを検証する話",
		briefdomain.QuestionIDOpeningEpisode:        "以前の生成記事を読んだとき、形は似ているのに自分の切実さが抜け落ちていると感じた場面から始めたい",
		briefdomain.QuestionIDReader:                "AIで発信を効率化したいが、自分らしさが薄まることに不安がある個人開発者や発信者",
		briefdomain.QuestionIDReaderProblem:         "AIに代筆させるほど発信は楽になるが、思想や違和感まで薄まるのではないかと迷っている",
		briefdomain.QuestionIDExpectedReaderAction:  "AIに代筆させるのではなく、自分の思想を深掘りする取材相手として使う視点を持ってほしい",
		briefdomain.QuestionIDKeyTakeaway:           "便利さに飛びつく前に、自分の違和感を言語化する工程が発信の核を守る",
		briefdomain.QuestionIDMustInclude:           "Note APIで記事を集めること、文体ガイドと一問一答を分けること、深掘り質問で記事の核を作ること。音楽家からエンジニア、起業、LT登壇、AI駆動開発という自分の文脈も自然に接続したい",
		briefdomain.QuestionIDConcreteExample:       "以前の生成記事を読み返し、形は似ているが切実さがないと気づいた場面。そこから文体分析、ブリーフ質問、深掘り質問へ設計を変えた流れ",
		briefdomain.QuestionIDEvidence:              "style score、keyword_overlap、verification、生成文字数を残し、感覚だけでなく検証可能な形にする",
		briefdomain.QuestionIDPersonalContext:       "音楽家として練習を積み重ねてきた経験、エンジニアとして実装と検証を繰り返してきた経験、起業やLT登壇で自分の言葉を人前に出してきた経験を入れたい。便利さに飛びつく一方で、自分の思想が薄まることへの怖さも正直に書きたい",
		briefdomain.QuestionIDExclusions:            "ローカルLLMを万能だと断言すること、根拠のない性能比較、Gemini依存",
		briefdomain.QuestionIDTargetLengthStructure: "3000字前後、最低2800字。導入、違和感、設計変更、Evo X2でのモデル使い分け、実装と検証、読者への提案、結論で構成する",
		briefdomain.QuestionIDToneStance:            "内省的だが技術検証の具体性もある。僕という一人称で、音楽やLTの経験も比喩として使いながら、読者に問いかける調子にする",
		briefdomain.QuestionIDTitleKeywords:         "ローカルLLM、過去記事、文体分析、思想、違和感、AI駆動開発",
	}
}

func scriptedDeepDiveAnswer(question briefdomain.ArticleQuestion) string {
	switch question.TargetQuestionID {
	case briefdomain.QuestionIDOpeningEpisode:
		if question.FollowUpIndex == 1 {
			return "画面には整った文章が出ているのに、僕が本当に悩んだ時間や焦りが消えていて、これは便利だけれど危ういと感じた"
		}
		return "そのときの感情は、効率化への期待よりも、自分の言葉を失う怖さの方が強かった"
	case briefdomain.QuestionIDMustInclude:
		if question.FollowUpIndex == 1 {
			return "文体分析と記事条件の聞き取りを分けることで、LLMに丸投げせず、音楽家時代の練習やLT登壇のように、取材メモを積み上げる構造にしたい"
		}
		return "読者には、AI生成の前に自分の違和感を言語化する工程こそが大事だと伝えたい"
	case briefdomain.QuestionIDPersonalContext:
		if question.FollowUpIndex == 1 {
			return "音楽の練習で身体に覚え込ませた感覚と、AI駆動開発で小さく試して直す感覚を重ねたい"
		}
		return "便利さに流されるほど、自分で考えたふりをしてしまう怖さがあることを隠さず書きたい"
	case briefdomain.QuestionIDExpectedReaderAction:
		if question.FollowUpIndex == 1 {
			return "自分の過去記事を単なる素材ではなく、今の自分に問い返す鏡として扱う理由を持ってほしい"
		}
		return "最初の一歩は、自分の記事を数本集めて、何度も出てくるテーマを書き出すこと"
	case briefdomain.QuestionIDToneStance:
		if question.FollowUpIndex == 1 {
			return "技術の話をしながらも、なぜそれを作るのかという個人的な切実さを中心に置きたい"
		}
		return "以前、LT登壇を続けた経験のように、怖さを抱えながらも試し続ける姿勢を支えにする"
	default:
		return "その回答の背景には、自分の発信を他人任せにしたくないという感覚がある"
	}
}

type scenarioFollowUpGenerator struct{}

func (scenarioFollowUpGenerator) GenerateFollowUp(ctx context.Context, session briefdomain.ArticleBriefSession, target briefdomain.ArticleQuestion, answer briefdomain.BriefAnswer, followUpIndex int) (string, error) {
	client, err := llamacpp.NewClientFromEnvForPurpose("BRIEF")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	excerpt := followUpPromptExcerpt(answer.Content, 90)
	prompt := fmt.Sprintf(`日本語の記事の取材質問を1つだけ作ってください。
条件:
- はい/いいえで答えられない
- 選択式にしない
- 具体的な経験、感情、判断、失敗、価値観を引き出す
- 質問文だけを出力する
- 質問は必ず「%s」というご回答を踏まえて、から始める

対象質問: %s
回答: %s
深掘り回数: %d
`, excerpt, target.Text, answer.Content, followUpIndex)
	generated, err := client.Generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	question := extractQuestion(generated)
	if !briefdomain.IsAllowedFollowUpQuestion(question) {
		return "", fmt.Errorf("generated follow-up was not allowed: %s", question)
	}
	return question, nil
}

func extractQuestion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`\"'")
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "-*0123456789. "))
		line = strings.Trim(line, "\"'")
		if strings.HasSuffix(line, "？") || strings.HasSuffix(line, "?") {
			return line
		}
	}
	return value
}

func followUpPromptExcerpt(content string, maxRunes int) string {
	content = strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if content == "" {
		return "その点"
	}
	content = strings.Trim(content, "「」\"'")
	runes := []rune(content)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes-1]) + "..."
	}
	return content
}

func writeJSON(path string, value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatalf("encode %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		fatalf("write %s: %v", path, err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
