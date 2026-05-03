package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

const defaultOutputDir = "tmp/format_persona_seed"

type scenarioSummary struct {
	Personas []personaSummary `json:"personas"`
	Formats  []formatSummary  `json:"formats"`
	Samples  []sampleSummary  `json:"samples"`
}

type personaSummary struct {
	ID             string   `json:"id"`
	DisplayName    string   `json:"display_name"`
	DefaultFormat  string   `json:"default_format"`
	SourceKinds    []string `json:"source_kinds"`
	PromptHintPath string   `json:"prompt_hint_path"`
}

type formatSummary struct {
	ID               string `json:"id"`
	DisplayName      string `json:"display_name"`
	RequiresMeta     bool   `json:"requires_meta"`
	AllowsCode       bool   `json:"allows_code"`
	PromptHasGuide   bool   `json:"prompt_has_guide"`
	SampleOutputPath string `json:"sample_output_path"`
}

type sampleSummary struct {
	Name           string `json:"name"`
	PersonaID      string `json:"persona_id"`
	OutputFormatID string `json:"output_format_id"`
	OutputPath     string `json:"output_path"`
}

func main() {
	outputDir := envOrDefault("SCENARIO_OUTPUT_DIR", defaultOutputDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fatalf("create output dir: %v", err)
	}

	personas := personadomain.DefaultRegistry().List()
	formats := outputformat.DefaultRegistry().List()
	if len(personas) < 2 {
		fatalf("expected at least two seeded personas, got %d", len(personas))
	}
	if len(formats) != len(sampleDrafts()) {
		fatalf("sample coverage mismatch: formats=%d samples=%d", len(formats), len(sampleDrafts()))
	}

	guide := scenarioGuide()
	summary := scenarioSummary{}
	seenDefaultFormats := map[string]bool{}
	for _, persona := range personas {
		if strings.TrimSpace(persona.DefaultFormat) == "" {
			fatalf("persona %s has no default format", persona.ID)
		}
		if _, ok := outputformat.DefaultRegistry().Get(persona.DefaultFormat); !ok {
			fatalf("persona %s references unknown default format %s", persona.ID, persona.DefaultFormat)
		}
		if len(persona.Sources) == 0 {
			fatalf("persona %s has no sources", persona.ID)
		}
		hint := persona.PromptHint()
		if hint == "" {
			fatalf("persona %s has empty prompt hint", persona.ID)
		}
		hintPath := filepath.Join(outputDir, persona.ID+"_prompt_hint.md")
		writeFile(hintPath, hint+"\n")
		seenDefaultFormats[persona.DefaultFormat] = true
		summary.Personas = append(summary.Personas, personaSummary{
			ID:             persona.ID,
			DisplayName:    persona.DisplayName,
			DefaultFormat:  persona.DefaultFormat,
			SourceKinds:    sourceKinds(persona),
			PromptHintPath: hintPath,
		})
	}
	if len(seenDefaultFormats) < 2 {
		fatalf("seed personas should exercise at least two default formats")
	}

	defaultPersona := mustPersona(personadomain.IDTerisuke)
	for _, format := range formats {
		sample, ok := sampleDrafts()[format.ID]
		if !ok {
			fatalf("missing sample draft for format %s", format.ID)
		}
		if _, err := articledomain.NewDraftForFormat(sample, format.ID); err != nil {
			fatalf("sample draft for %s failed validation: %v", format.ID, err)
		}
		prompt := draftapp.BuildPromptForMode(guide, scenarioBrief(defaultPersona, format), defaultPersona, format)
		promptHasGuide := strings.Contains(prompt, "## 媒体別Markdownガイド")
		if !promptHasGuide {
			fatalf("prompt for %s did not include embedded format guide", format.ID)
		}
		outputPath := filepath.Join(outputDir, format.ID+".md")
		writeFile(outputPath, sample+"\n")
		summary.Formats = append(summary.Formats, formatSummary{
			ID:               format.ID,
			DisplayName:      format.DisplayName,
			RequiresMeta:     format.RequiresMeta,
			AllowsCode:       format.AllowsCode,
			PromptHasGuide:   promptHasGuide,
			SampleOutputPath: outputPath,
		})
	}

	for _, sample := range personaFormatSamples() {
		persona := mustPersona(sample.personaID)
		format := outputformat.DefaultRegistry().MustGet(sample.formatID)
		brief := scenarioBrief(persona, format)
		prompt := draftapp.BuildPromptForMode(guide, brief, persona, format)
		for _, want := range []string{persona.DisplayName, format.DisplayName, "## 媒体別Markdownガイド"} {
			if !strings.Contains(prompt, want) {
				fatalf("%s prompt missing %q", sample.name, want)
			}
		}
		draft := sampleDrafts()[sample.formatID]
		if _, err := articledomain.NewDraftForFormat(draft, sample.formatID); err != nil {
			fatalf("%s sample failed validation: %v", sample.name, err)
		}
		path := filepath.Join(outputDir, sample.name+".md")
		writeFile(path, draft+"\n")
		summary.Samples = append(summary.Samples, sampleSummary{
			Name:           sample.name,
			PersonaID:      sample.personaID,
			OutputFormatID: sample.formatID,
			OutputPath:     path,
		})
	}

	writeJSON(filepath.Join(outputDir, "summary.json"), summary)
	fmt.Printf("format/persona seed scenario completed\n")
	fmt.Printf("personas=%d\n", len(summary.Personas))
	fmt.Printf("formats=%d\n", len(summary.Formats))
	fmt.Printf("samples=%d\n", len(summary.Samples))
	fmt.Printf("summary=%s\n", filepath.Join(outputDir, "summary.json"))
}

func scenarioGuide() authordomain.WritingStyleGuide {
	return authordomain.WritingStyleGuide{
		ID:                   "guide_format_persona_seed",
		ProfileID:            "profile_format_persona_seed",
		Markdown:             "実体験、検証、読者への次の行動を自然につなぐ。",
		PreferredFirstPerson: "僕",
		RecurringThemes:      []string{"AI", "検証", "発信"},
		ParagraphRhythm:      "短すぎない段落で、経験と判断を接続する。",
		SentenceRhythm:       "読みやすい中程度の文で断定と余白を混ぜる。",
		HeadingGuidance:      "読者が流れを追える具体的な見出しにする。",
		QuoteGuidance:        "印象的な言葉は必要な箇所にだけ置く。",
		OpeningPatterns:      []string{"違和感から始める", "検証結果から始める"},
		ConclusionPatterns:   []string{"次の行動で締める"},
	}
}

func scenarioBrief(persona personadomain.Persona, format outputformat.OutputFormat) briefdomain.ArticleBrief {
	return briefdomain.ArticleBrief{
		StyleProfileID:        "profile_format_persona_seed",
		PersonaID:             persona.ID,
		OutputFormatID:        format.ID,
		Theme:                 "形式別に同じ検証メモを書き分ける",
		Reader:                "AIを使って発信と実装を改善したい開発者",
		MustInclude:           "出力先ごとの記法、検証結果、次の行動",
		PersonalContext:       "音楽家、エンジニア、起業家として検証を積み重ねてきた背景",
		TargetLengthStructure: "短い導入、具体例、結論",
		ToneStance:            persona.PromptHint(),
	}
}

func sampleDrafts() map[string]string {
	return map[string]string{
		outputformat.IDNoteArticle:     "# 形式で文章の届き方が変わる\n\n## 違和感\n\n同じ検証メモでも、出す場所が変わると読者の期待は変わる。\n\n## 試したこと\n\n- noteでは体験と解釈を中心にした\n- 技術媒体では手順を明確にした\n\n## 次の一歩\n\nまずは出力先を決めてから下書きを始める。",
		outputformat.IDMarkdownBlog:    "---\ntitle: \"形式別プロンプトの検証\"\ndescription: \"出力先ごとの記法と検証観点を分けて下書き品質を安定させる\"\npubDate: 2026-05-02\nauthor: \"Terisuke\"\ncategory: \"engineering\"\ntags: [\"AI\", \"開発\", \"検証\"]\nlang: \"ja\"\nfeatured: false\nisDraft: true\n---\n\n# 形式別プロンプトの検証\n\n出力先ごとのルールを分けると、レビューで見るべき点が明確になる。\n\n```go\nfmt.Println(\"validated\")\n```",
		outputformat.IDZennArticle:     "---\ntitle: \"形式別プロンプトをGoで検証する\"\nemoji: \"🧪\"\ntype: \"tech\"\ntopics: [\"go\", \"ai\", \"prompt\"]\npublished: false\n---\n\n## 実装\n\nZennでは手順とコードを先に読める形にする。\n\n:::message\n媒体ごとの記法を混ぜないことが重要です。\n:::\n\n```diff go\n+fmt.Println(\"zenn\")\n```",
		outputformat.IDQiitaArticle:    "---\ntitle: \"Qiita向けプロンプト検証\"\ntags: [{name: Go, versions: [\"1.24\"]}, {name: AI}]\n---\n\n## 環境\n\nQiitaでは再現手順を先に置く。\n\n:::note warn\nZennの記法を混ぜないようにします。\n:::\n\n```diff_go\n+fmt.Println(\"qiita\")\n```",
		outputformat.IDHomepageSection: "<section><h2>形式別に伝わる下書きへ</h2><p>同じ材料でも、出力先に合わせて構成と記法を切り替えることで、読み手が次の行動を取りやすくなります。</p><a href=\"/contact\">相談する</a></section>",
	}
}

type personaFormatSample struct {
	name      string
	personaID string
	formatID  string
}

func personaFormatSamples() []personaFormatSample {
	return []personaFormatSample{
		{name: "terisuke_note", personaID: personadomain.IDTerisuke, formatID: outputformat.IDNoteArticle},
		{name: "terisuke_blog", personaID: personadomain.IDTerisuke, formatID: outputformat.IDMarkdownBlog},
		{name: "cloudia_zenn", personaID: personadomain.IDCloudia, formatID: outputformat.IDZennArticle},
		{name: "cloudia_qiita", personaID: personadomain.IDCloudia, formatID: outputformat.IDQiitaArticle},
		{name: "terisuke_homepage", personaID: personadomain.IDTerisuke, formatID: outputformat.IDHomepageSection},
	}
}

func sourceKinds(persona personadomain.Persona) []string {
	kinds := make([]string, 0, len(persona.Sources))
	for _, source := range persona.Sources {
		kinds = append(kinds, source.Kind)
	}
	return kinds
}

func mustPersona(id string) personadomain.Persona {
	persona, ok := personadomain.DefaultRegistry().Get(id)
	if !ok {
		fatalf("missing persona %s", id)
	}
	return persona
}

func writeJSON(path string, value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatalf("encode %s: %v", err)
	}
	writeFile(path, string(encoded)+"\n")
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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
