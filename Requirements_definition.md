## Note記事下書きジェネレーター 実装要件定義書

**文書バージョン:** 2.0
**作成日:** 2025年4月25日
**最終更新日:** 2025年4月25日
**作成者:** [担当者名/チーム名]

**改訂履歴**
| バージョン | 更新日        | 更新内容                                                                                   | 更新者     |
|:------|:-----------|:---------------------------------------------------------------------------------------|:--------|
| 1.0   | 2025年4月15日 | 初版作成 (ベース: 要件定義書 v1.0)                                                            | [担当者名] |
| 1.1   | 2025年4月20日 | Note API v3調査反映、AIモデル変更(Gemini 2.5 Pro Preview)、関連セクション修正 (ベース: 要件定義書 v1.1) | [担当者名] |
| 2.0   | 2025年4月25日 | 「実装志向要件定義書」の内容を統合、構成変更、実装コード例の拡充、プロンプト設計の具体化、全体レビュー           | [担当者名] |

**目次**

1.  **はじめに**
    A.  文書目的
    B.  背景
    C.  スコープ
    D.  用語定義
2.  **システム概要**
    A.  目的
    B.  全体構成
3.  **技術スタック**
    A.  フロントエンド
    B.  バックエンド
4.  **機能要件**
    A.  フロントエンド機能
    B.  バックエンドAPI機能
        1.  APIエンドポイント定義
        2.  Note API連携
        3.  文体分析ロジック (任意実装、高度化オプション)
        4.  Gemini APIによる記事生成
        5.  出力形式
5.  **バックエンド実装詳細 (Go)**
    A.  Note APIクライアント実装
    B.  文体分析実装 (任意実装)
    C.  Gemini APIクライアント実装
    D.  Web APIハンドラ実装
6.  **フロントエンド実装詳細**
    A.  HTML構造 (`static/index.html`)
    B.  CSSスタイリング (`static/css/style.css`)
    C.  JavaScriptロジック (`static/js/script.js`)
7.  **Gemini 2.5 Pro Preview プロンプト設計**
    A.  設計思想
    B.  基本構造
    C.  構成要素詳細
    D.  プロンプト構築関数 (`buildPrompt`) とプロンプト例
    E.  モデルパラメータ調整
    F.  プロンプトエンジニアリング戦略
8.  **非機能要件**
    A.  パフォーマンス
    B.  セキュリティ
    C.  可用性
    D.  保守性
    E.  運用・監視
    F.  APIキー管理
9.  **デプロイメント**
    A.  ディレクトリ構成
    B.  環境変数の設定
    C.  ビルドとデプロイ (ローカル, Docker)
10. **テスト戦略**
11. **付録**
    A.  参考資料
    B.  Note API仕様に関する補足

---

## 1. はじめに

### A. 文書目的
本ドキュメントは、「Note記事下書きジェネレーター」の開発における実装レベルの要件を定義することを目的とします。開発者が具体的な実装作業を進めるための技術仕様、コード例、設計指針を提供し、開発チーム内での共通認識を確立します。

### B. 背景
コンテンツクリエイターやマーケターは、Noteプラットフォーム上で質の高い記事を継続的に公開する必要がありますが、アイデア出しから執筆には多大な時間と労力がかかります。このプロセスを効率化するため、既存のNote記事やユーザーの指示に基づき、AIを活用して新しい記事の下書きを生成するツールを開発します。

### C. スコープ
**対象範囲:**

*   指定されたNote記事URLから記事本文を取得する機能。
*   ユーザーが指定したキーワードやテーマに基づき、取得した記事本文をコンテキストとして、GoogleのAIモデル `gemini-2.5-pro-preview-03-25` を用いて新しい記事の下書きを生成する機能。
*   (オプション) 指定Noteユーザーの記事群から文体を分析する機能。
*   生成された下書きをマークダウン形式で出力する機能。
*   バックエンドAPIの開発 (Go言語)。
*   基本的な操作を行うためのWebフロントエンドの開発 (HTML/CSS/JS)。

**対象範囲外:**

*   高度なUI/UXデザイン。
*   ユーザー認証・認可機能。
*   Noteへの直接投稿機能。
*   生成された下書きの高度な編集・校正機能。
*   画像生成・挿入機能 (プレースホルダー挿入は可)。
*   複数ユーザー対応、チーム機能など。

### D. 用語定義
*   **Note記事:** note株式会社が運営するプラットフォーム「note」上で公開されている記事。
*   **記事本文:** Note記事の主要なテキストコンテンツ。
*   **下書き:** 生成AIによって作成された、公開前の記事の草稿。
*   **Gemini 2.5 Pro Preview:** Googleによって開発された大規模言語モデル（LLM）。本システムで使用する主要AIモデル。
*   **API:** Application Programming Interface。本システムではバックエンド機能を提供するインターフェースを指す。
*   **Note API v2/v3:** Noteが（非公式ながら）提供しているAPI。ユーザー情報、記事一覧、記事詳細取得に利用。**安定性や公式サポートは保証されない点に留意。**
*   **スクレイピング:** ウェブサイトから情報を自動的に抽出する技術。Note APIが利用できない場合の代替手段。
*   **プロンプト:** 生成AIモデルに対して、特定のタスクを実行させるための指示や入力テキスト。
*   **文体分析:** 記事の段落長、文長、頻出表現、トーンなどを分析すること（本ドキュメントでは高度化オプション）。

## 2. システム概要

### A. 目的
本システムは、既存のNote記事（URL指定）とユーザーからの指示（キーワード、テーマ等）を入力とし、AI (`gemini-2.5-pro-preview-03-25`) を活用して新しいNote記事の下書きを自動生成するWebアプリケーションを提供します。記事作成の初期段階における時間コスト削減と効率化を目指します。オプションとして、特定ユーザーの文体を模倣する機能も検討します。

### B. 全体構成
本システムは、ユーザーインターフェースを提供するフロントエンドと、主要な処理を実行するバックエンドAPIから構成されます。

*   **フロントエンド:**
    *   HTML/CSS/JavaScriptで実装。
    *   ユーザーがNote記事URLや生成指示を入力。
    *   バックエンドAPIを呼び出し、結果（生成された下書き）を表示。
*   **バックエンドAPI:**
    *   Go言語で実装。
    *   Note API (v2/v3) またはWebスクレイピングにより指定されたNote記事本文を取得。
    *   ユーザー入力を解釈し、Gemini 2.5 Pro Previewへのプロンプトを構築。
    *   Gemini APIを呼び出し、記事下書きを生成。
    *   生成された下書き（マークダウン形式）をフロントエンドに返却。

```mermaid
graph LR
    A[ユーザー] -- 1. URL, 指示入力 --> B(フロントエンド);
    B -- 2. APIリクエスト --> C{バックエンド API (Go)};
    C -- 3. Note記事取得 --> D{Note API v2/v3 / スクレイピング};
    D -- 4. 記事本文 --> C;
    C -- 5. プロンプト構築 --> E{Gemini 2.5 Pro Preview API};
    E -- 6. 生成テキスト --> C;
    C -- 7. マークダウン下書き --> B;
    B -- 8. 下書き表示 --> A;
```

## 3. 技術スタック

### A. フロントエンド
*   HTML5 / CSS3 / JavaScript (ES6+)
*   マークダウンレンダリング: marked.js (または同等のライブラリ)
*   フレームワーク: なし (Vanilla JS) または軽量フレームワーク (任意)

### B. バックエンド
*   言語: Go (最新安定版, 1.18以降推奨)
*   Webフレームワーク: 標準パッケージ `net/http` + `gorilla/mux` (または Gin, Echo 等)
*   HTTPクライアント: 標準パッケージ `net/http`
*   JSONパーサー: `encoding/json`
*   HTMLパーサー (スクレイピング用): `golang.org/x/net/html` または `github.com/PuerkitoBio/goquery`
*   設定管理: `github.com/spf13/viper` (任意) または 環境変数
*   ロギング: `log/slog` (構造化ログ, Go 1.21+) または 標準 `log`
*   Gemini APIクライアント: `google.golang.org/api/vertexai/v1` (Vertex AI経由推奨) または `google.golang.org/genai` (Gemini API直接)
*   **注意:** Go 1.16以降、`io/ioutil` は非推奨。`io` および `os` パッケージの関数を使用する。

## 4. 機能要件

### A. フロントエンド機能
*   ユーザーが以下を入力できるシンプルなインターフェース:
    *   参照するNote記事のURL (必須)
    *   生成する記事のキーワード (任意)
    *   生成する記事の主なテーマ・視点 (任意)
    *   想定読者層 (任意)
    *   含めないでほしい内容 (任意)
    *   文体 (例: ですます調/である調) (任意, デフォルト: ですます調)
    *   トーン (例: 客観的/情熱的) (任意, デフォルト: 客観的)
    *   (オプション) 文体分析対象のNoteユーザー名
    *   希望する文字数 (目安)
*   「生成」ボタンによりバックエンドAPIを呼び出す。
*   処理中のローディング表示。
*   生成された記事下書き（マークダウン）をテキストエリアに表示。
*   マークダウンのプレビュー表示機能。
*   生成されたマークダウンをクリップボードにコピーする機能。
*   エラー発生時のメッセージ表示。

### B. バックエンドAPI機能

#### 1. APIエンドポイント定義
*   **エンドポイント:** `/api/generate`
*   **メソッド:** `POST`
*   **リクエストボディ (JSON):**
    ```json
    {
      "note_url": "string (required)",        // 対象Note記事のURL
      "keywords": ["string"],                 // 関連キーワード (任意)
      "theme": "string",                      // 主なテーマ・視点 (任意)
      "target_audience": "string",            // 想定読者層 (任意)
      "exclusions": "string",                 // 含めないでほしい内容 (任意)
      "style_choice": "string",               // 文体 (任意, デフォルト: "ですます調")
      "tone_choice": "string",                // トーン (任意, デフォルト: "客観的")
      "word_count": "int",                    // 目標文字数 (任意, デフォルト: 1500)
      "reference_username_for_style": "string" // (オプション) 文体分析用ユーザー名
    }
    ```
*   **レスポンスボディ (JSON):**
    *   **成功時 (200 OK):**
        ```json
        {
          "draft": "string" // 生成された記事下書き (マークダウン形式)
        }
        ```
    *   **エラー時 (4xx/5xx):**
        ```json
        {
          "error": {
            "code": "string",    // エラーコード (例: "INVALID_URL", "FETCH_FAILED", "GENERATION_FAILED")
            "message": "string" // エラーメッセージ
          }
        }
        ```

#### 2. Note API連携 / 記事本文取得
*   **目的:** 指定されたNote記事URLから、記事の主要なテキストコンテンツを取得する。
*   **取得戦略 (優先度順):**
    1.  **Note API v3 (試行):**
        *   エンドポイント: `https://note.com/api/v3/notes/{note_id}` (Note URLから`note_id`を抽出する必要あり)
        *   実装: 指定されたURLから `note_id` をパースし、APIを叩いてみる。成功すれば本文 (`data.body`) を取得。
        *   **注意点:** 公式ドキュメントがなく、非公式な利用となるため、予告なく仕様変更や利用不可になるリスクが高い。レスポンス構造が変わる可能性もある。**本番運用での安定性は期待できない。**
    2.  **Webスクレイピング (代替策):**
        *   実装: API v3での取得が失敗した場合、指定URLのHTMLを直接取得し、本文が含まれる要素（例: `div.o-noteContentText` や `div[data-testid="note-body"]` など、実際のHTML構造を確認して決定）からテキストを抽出する。`goquery` 等のライブラリ利用を推奨。
        *   **注意点:** Note側のHTML構造変更に非常に脆弱。定期的なメンテナンスが必要。利用規約に抵触しない範囲で、適切なUser-Agent設定、アクセス間隔調整を行う。
    3.  **RSSフィード (補助的):**
        *   ユーザーページのRSS (`https://note.com/{username}/rss`) から記事URLを取得できる場合があるが、本文全文が含まれないことが多いため、主たる取得方法としては不向き。
*   **実装方針:**
    *   まずNote API v3での取得を試行する。成功すればそのデータを利用。
    *   API v3が失敗した場合（404, 5xxエラー、レスポンス構造不一致等）、Webスクレイピングを実行する。
    *   両方失敗した場合は、クライアントに `FETCH_FAILED` エラーを返す。
    *   適切なタイムアウト設定、エラーハンドリングを行う。

#### 3. 文体分析ロジック (任意実装、高度化オプション)
*   **目的:** (オプション機能) 指定されたNoteユーザーの過去記事から文体を分析し、記事生成時のプロンプトに反映させることで、よりそのユーザーらしい記事を生成する。
*   **実装:**
    *   Note API v2 (`https://note.com/api/v2/creators/{username}/contents?kind=note&page=1`) を使用して、指定ユーザーの最新記事リストを取得。
    *   リストから複数の記事IDを取得し、Note API v3またはスクレイピングで各記事本文を取得（並行処理、レート制限考慮）。
    *   取得した複数記事の本文を分析し、特徴（平均文長、段落長、頻出語、文末表現、トーン、よく使うハッシュタグ等）を抽出する。
    *   分析結果を構造化し、記事生成時のプロンプトに含める。
*   **注意:** ユーザー毎に十分な記事数がないと分析精度が低い。API負荷も増大する。

#### 4. Gemini APIによる記事生成
*   **使用モデル:** `gemini-2.5-pro-preview-03-25` (Vertex AI経由推奨)
    *   **理由:** 高度な推論能力、長文コンテキスト処理能力により、高品質な記事下書き生成が期待できるため。Flashモデルよりコストは高いが、創造性や指示追従性で優位。
*   **処理フロー:**
    1.  取得したNote記事本文、ユーザー入力（キーワード、テーマ等）、（オプションで）文体分析結果を基に、**7. プロンプト設計** に基づくプロンプトを構築。
    2.  Vertex AI API (または Gemini API) を使用して、`gemini-2.5-pro-preview-03-25` モデルにプロンプトを送信。
    3.  APIレスポンスから生成されたテキスト（記事下書き）を抽出。
    4.  エラーハンドリング（APIエラー、コンテンツフィルターによるブロック等）を行う。
*   **コスト・パフォーマンス:**
    *   Proモデルは高価なため、トークン使用量を監視。`maxOutputTokens` を適切に設定する。
    *   応答時間はFlashより長くなる可能性があるため、タイムアウト設定に注意。

#### 5. 出力形式
*   生成された記事下書きは、マークダウン形式のテキストとして返却する。見出し、リスト、強調、コードブロック等の基本的な記法に対応することを期待する（プロンプトで指示）。

## 5. バックエンド実装詳細 (Go)

（以下に主要部分のコード例を示す。完全な実装は省略。）

### A. Note APIクライアント実装

```go
package noteapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery" // スクレイピング用
)

const (
	noteAPIV3BaseURL = "https://note.com/api/v3/notes/"
	requestTimeout  = 15 * time.Second
	userAgent       = "Mozilla/5.0 (compatible; NoteArticleGeneratorBot/1.0; +http://example.com/bot)" // 適切なUserAgentを設定
)

var noteIDRegex = regexp.MustCompile(`note\.com/(?:[^/]+/)?n/([a-zA-Z0-9]+)`)

// 記事詳細 (API v3 レスポンス想定)
type NoteArticleDetailV3 struct {
	Data struct {
		Name string `json:"name"` // タイトル
		Body string `json:"body"` // 本文 (HTML形式の場合あり)
		// 他に必要なフィールドがあれば追加
	} `json:"data"`
}

// HTTPクライアント初期化
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: requestTimeout,
	}
}

// Note記事本文取得関数
func FetchArticleBody(noteURL string) (string, string, error) {
	client := newHTTPClient()

	// 1. URLからNote IDを抽出
	noteID := extractNoteID(noteURL)
	if noteID == "" {
		// Note IDが抽出できない場合、スクレイピングを試みる
		title, body, err := scrapeArticle(client, noteURL)
		if err != nil {
			return "", "", fmt.Errorf("failed to extract Note ID and scraping failed: %w", err)
		}
		return title, body, nil
	}

	// 2. Note API v3 を試行
	title, body, err := fetchArticleFromAPIV3(client, noteID)
	if err == nil {
		// API v3 成功
		// bodyがHTMLの場合があるので、プレーンテキストに変換する処理が必要な場合がある
		// body = convertHTMLToText(body) // 必要に応じて実装
		return title, body, nil
	}
	fmt.Printf("Note API v3 failed for %s (Error: %v), attempting scraping...\n", noteID, err) // エラーログ

	// 3. API v3 が失敗した場合、スクレイピングを実行
	title, body, err = scrapeArticle(client, noteURL)
	if err != nil {
		return "", "", fmt.Errorf("API v3 failed and scraping also failed: %w", err)
	}

	return title, body, nil
}

// URLからNote IDを抽出
func extractNoteID(noteURL string) string {
	matches := noteIDRegex.FindStringSubmatch(noteURL)
	if len(matches) >= 2 {
		return matches[1]
	}
	// 例: note.com/info/n/naaaaaaaaaaaa のような形式にも対応する場合、追加の正規表現が必要
	// 例: note.com/magazine/{magazine_id}/n/{note_id} のような形式
	parts := strings.Split(noteURL, "/n/")
	if len(parts) == 2 {
		idParts := strings.Split(parts[1], "?") // クエリパラメータを除去
		if len(idParts[0]) > 5 { // 短すぎるものはIDではないと仮定
			return idParts[0]
		}
	}
	return ""
}

// Note API v3から記事取得
func fetchArticleFromAPIV3(client *http.Client, noteID string) (string, string, error) {
	reqURL := noteAPIV3BaseURL + noteID
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("API returned non-OK status: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", err)
	}

	var detail NoteArticleDetailV3
	if err := json.Unmarshal(bodyBytes, &detail); err != nil {
		// JSONパースエラーの場合、レスポンスが期待通りでない可能性
		fmt.Printf("JSON parse error for Note ID %s. Response Body: %s\n", noteID, string(bodyBytes))
		return "", "", fmt.Errorf("JSON parse error: %w", err)
	}

	if detail.Data.Body == "" {
		return "", "", fmt.Errorf("API response body is empty")
	}

	return detail.Data.Name, detail.Data.Body, nil
}

// Webスクレイピングで記事取得
func scrapeArticle(client *http.Client, noteURL string) (string, string, error) {
	req, err := http.NewRequest("GET", noteURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request for scraping: %w", err)
	}
	req.Header.Set("User-Agent", userAgent) // 適切なUser-Agent

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("scraping request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("scraping target returned non-OK status: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// タイトル取得 (実際のセレクタに合わせて調整)
	title := doc.Find("h1").First().Text() // 例: h1タグ
	if title == "" {
		title = doc.Find("meta[property='og:title']").AttrOr("content", "") // OpenGraphから取得
	}
	title = strings.TrimSpace(title)


	// 本文取得 (実際のセレクタに合わせて調整)
	// 複数の可能性のあるセレクタを試す
	body := ""
	selectors := []string{
		"div.o-noteContentText",
		"div.note-common-styles__textnote-body", // 最近の構造？
		"div[data-testid='note-body']",         // testid は変わりやすいので注意
		"article",                               // article タグ全体
	}
	for _, selector := range selectors {
		body = doc.Find(selector).Text() // .Text() は子要素のテキストも結合する
		if strings.TrimSpace(body) != "" {
			// .Html() を使ってHTMLを取得し、後で変換する方が良い場合もある
			// bodyHtml, _ := doc.Find(selector).Html()
			break
		}
	}

	body = strings.TrimSpace(body)

	if title == "" && body == "" {
		return "", "", fmt.Errorf("failed to extract title and body using scraping")
	}
	// スクレイピングの場合、不要な部分（メニュー、フッターなど）が含まれる可能性あり
    // 必要であれば、さらに不要部分を除去する処理を追加

	return title, body, nil
}

// (参考) 複数記事の並行取得（レート制限対応版） - 文体分析用
// func fetchLatestArticles(username string, count int) ([]NoteArticleDetail, error) { ... }
// 上記の実装では Note API v2/v3 を使っているため、v3 が不安定な場合は注意が必要。
// v2で記事リスト取得 -> 各記事URLに対して FetchArticleBody を並行実行（レート制限付き）する形に修正する。
```

### B. 文体分析実装 (任意実装)
```go
package analyzer

import (
	"strings"
	// 他に必要なパッケージ
)

// 文体分析結果
type StyleAnalysis struct {
	ParagraphLength     string   // 例: "短め", "中程度", "長め"
	AvgSentenceLength   int
	CommonExpressions   []string // 例: "です", "ます"
	Tone                string   // 例: "丁寧", "カジュアル"
	UniqueExpressions   []string // 例: "〜と言えるでしょう"
	CommonHashtags      []string
}

// 文体分析を行う関数 (複数記事の本文を受け取る)
func AnalyzeWritingStyle(articleBodies []string) StyleAnalysis {
    analysis := StyleAnalysis{
        // 初期化
    }
    if len(articleBodies) == 0 {
        return analysis // 分析対象がない
    }

    combinedText := strings.Join(articleBodies, "\n\n")

    // --- ここに分析ロジックを実装 ---
    // 1. 段落分析 (改行文字 `\n\n` で分割)
    // 2. 文分析 (句読点 `。` `！` `？` で分割) -> 平均文長
    // 3. 頻出表現分析 (形態素解析ライブラリ kakasi, mecab等の利用も検討)
    // 4. トーン分析 (文末表現 `です/ます` vs `だ/である` の比率など)
    // 5. 特徴的表現の抽出
    // 6. (ハッシュタグは別途記事リストから取得)

    // ダミー実装
    analysis.ParagraphLength = "中程度"
    analysis.AvgSentenceLength = 50
    analysis.CommonExpressions = []string{"です", "ます"}
    analysis.Tone = "丁寧"

    return analysis
}

// 日本語の文を分割する簡易関数 (より高度な分割には形態素解析が必要)
func splitJapaneseSentences(text string) []string {
	// 実装例は 文書2 を参照
    // ...
    return []string{} // ダミー
}
```

### C. Gemini APIクライアント実装
```go
package gemini

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/vertexai/apiv1"
	"cloud.google.com/go/vertexai/apiv1/vertexaipb"
	// "google.golang.org/genai" // Gemini API直接利用の場合
	// "google.golang.org/api/option"
)

// 記事生成の入力
type ArticleGenerationInput struct {
	OriginalArticleTitle string
	OriginalArticleBody  string
	Keywords             []string
	Theme                string
	TargetAudience       string
	Exclusions           string
	StyleChoice          string
	ToneChoice           string
	WordCount            int
	StyleAnalysis        *analyzer.StyleAnalysis // オプション: 文体分析結果
}

// Vertex AI経由で記事を生成する関数
func GenerateArticleVertexAI(ctx context.Context, input ArticleGenerationInput) (string, error) {
	projectID := os.Getenv("GOOGLE_PROJECT_ID")
	location := os.Getenv("GOOGLE_LOCATION") // 例: "us-central1"
	modelName := "gemini-2.5-pro-preview-06-05" // 使用するモデル

	if projectID == "" || location == "" {
		return "", fmt.Errorf("GOOGLE_PROJECT_ID and GOOGLE_LOCATION must be set")
	}

	client, err := vertexai.NewPredictionClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create prediction client: %w", err)
	}
	defer client.Close()

	prompt := buildPrompt(input) // プロンプト構築 (7.D参照)

	// パラメータ設定 (7.E参照)
	params := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"temperature":     {Kind: &structpb.Value_NumberValue{NumberValue: 0.7}},
			"maxOutputTokens": {Kind: &structpb.Value_NumberValue{NumberValue: 4096}}, // 目標文字数に応じて調整
			"topP":            {Kind: &structpb.Value_NumberValue{NumberValue: 0.95}},
			"topK":            {Kind: &structpb.Value_NumberValue{NumberValue: 40}},
		},
	}

	// リクエスト作成
	endpoint := fmt.Sprintf("projects/%s/locations/%s/publishers/google/models/%s", projectID, location, modelName)
	req := &vertexaipb.GenerateContentRequest{
		Endpoint: endpoint,
		Contents: []*vertexaipb.Content{
			{Role: "user", Parts: []*vertexaipb.Part{{Data: &vertexaipb.Part_Text{Text: prompt}}}},
		},
		GenerationConfig: &vertexaipb.GenerationConfig{
			Temperature:     proto.Float32(0.7),
			MaxOutputTokens: proto.Int32(4096),
			TopP:            proto.Float32(0.95),
			TopK:            proto.Float32(40.0), // float32に注意
		},
		// SafetySettings: []*vertexaipb.SafetySetting{...} // 必要に応じて設定
	}


	resp, err := client.GenerateContent(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	// レスポンス検証とテキスト抽出
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
        // コンテンツフィルターなどによるブロックの可能性
        if resp.PromptFeedback != nil && len(resp.PromptFeedback.BlockReason) > 0 {
            return "", fmt.Errorf("generation blocked: %s", resp.PromptFeedback.BlockReason)
        }
		return "", fmt.Errorf("empty response from Gemini API")
	}

	var generatedText strings.Builder
    for _, part := range resp.Candidates[0].Content.Parts {
        if textPart, ok := part.GetData().(*vertexaipb.Part_Text); ok {
            generatedText.WriteString(textPart.Text)
        }
    }


	return generatedText.String(), nil
}

// プロンプト構築関数 (詳細は 7.D で定義)
func buildPrompt(input ArticleGenerationInput) string {
	// ... (7.D の実装を参照)
	return "ここに構築されたプロンプトが入ります"
}
```

### D. Web APIハンドラ実装
```go
package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog" // Go 1.21+
	"net/http"
	// "time" // 必要に応じて

	"your_project_path/analyzer" // analyzer パッケージのパス
	"your_project_path/gemini"   // gemini パッケージのパス
	"your_project_path/noteapi"  // noteapi パッケージのパス

	"github.com/gorilla/mux"
)

// リクエストボディの構造体
type GenerateRequest struct {
	NoteURL          string   `json:"note_url"`
	Keywords         []string `json:"keywords"`
	Theme            string   `json:"theme"`
	TargetAudience   string   `json:"target_audience"`
	Exclusions       string   `json:"exclusions"`
	StyleChoice      string   `json:"style_choice"`
	ToneChoice       string   `json:"tone_choice"`
	WordCount        int      `json:"word_count"`
	RefUsernameStyle string   `json:"reference_username_for_style"` // オプション
}

// エラーレスポンスの構造体
type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// 成功レスポンスの構造体
type SuccessResponse struct {
	Draft string `json:"draft"`
}

// 記事生成ハンドラ
func GenerateArticleHandler(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// --- 入力検証 ---
	if req.NoteURL == "" {
		respondWithError(w, "MISSING_REQUIRED_FIELD", "note_url is required", http.StatusBadRequest)
		return
	}
	// URL形式の簡易チェックなど

	// デフォルト値設定
	if req.StyleChoice == "" { req.StyleChoice = "ですます調" }
	if req.ToneChoice == "" { req.ToneChoice = "客観的" }
	if req.WordCount <= 0 { req.WordCount = 1500 }

	// --- Note記事本文取得 ---
	slog.Info("Fetching article", "url", req.NoteURL)
	title, body, err := noteapi.FetchArticleBody(req.NoteURL)
	if err != nil {
		slog.Error("Failed to fetch article", "url", req.NoteURL, "error", err)
		respondWithError(w, "FETCH_FAILED", fmt.Sprintf("Failed to fetch article content: %v", err), http.StatusInternalServerError)
		return
	}
	slog.Info("Article fetched successfully", "url", req.NoteURL, "title", title)

	// --- (オプション) 文体分析 ---
	var styleAnalysis *analyzer.StyleAnalysis
	if req.RefUsernameStyle != "" {
		// TODO: 文体分析の実装 (複数記事取得 -> 分析)
		slog.Info("Performing style analysis", "username", req.RefUsernameStyle)
		// analysisResult, err := analyzer.AnalyzeUserStyle(req.RefUsernameStyle)
		// if err != nil {
		//     slog.Warn("Style analysis failed", "username", req.RefUsernameStyle, "error", err)
		// } else {
		//     styleAnalysis = &analysisResult
		//     slog.Info("Style analysis completed", "username", req.RefUsernameStyle)
		// }
	}

	// --- Gemini APIによる記事生成 ---
	generationInput := gemini.ArticleGenerationInput{
		OriginalArticleTitle: title,
		OriginalArticleBody:  body,
		Keywords:             req.Keywords,
		Theme:                req.Theme,
		TargetAudience:       req.TargetAudience,
		Exclusions:           req.Exclusions,
		StyleChoice:          req.StyleChoice,
		ToneChoice:           req.ToneChoice,
		WordCount:            req.WordCount,
		StyleAnalysis:        styleAnalysis, // 分析結果を渡す
	}

	slog.Info("Generating article with Gemini", "model", "gemini-2.5-pro-preview")
	generatedDraft, err := gemini.GenerateArticleVertexAI(r.Context(), generationInput)
	if err != nil {
		slog.Error("Failed to generate article", "error", err)
		respondWithError(w, "GENERATION_FAILED", fmt.Sprintf("Failed to generate article draft: %v", err), http.StatusInternalServerError)
		return
	}
	slog.Info("Article generated successfully")

	// --- 成功レスポンス返却 ---
	respondWithJSON(w, http.StatusOK, SuccessResponse{Draft: generatedDraft})
}

// エラーレスポンス送信ヘルパー
func respondWithError(w http.ResponseWriter, code, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	errResp := ErrorResponse{}
	errResp.Error.Code = code
	errResp.Error.Message = message
	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		slog.Error("Failed to encode error response", "error", err)
	}
}

// JSONレスポンス送信ヘルパー
func respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("Failed to encode success response", "error", err)
		// ここでエラーレスポンスを返そうとするとヘッダーが二重に書き込まれる可能性
		http.Error(w, `{"error":{"code":"INTERNAL_SERVER_ERROR", "message":"Failed to encode response"}}`, http.StatusInternalServerError)
	}
}

// ルーター設定 (main.go などで利用)
func SetupRouter() *mux.Router {
    r := mux.NewRouter()
    r.HandleFunc("/api/generate", GenerateArticleHandler).Methods("POST")
    // 他のエンドポイントがあれば追加
    return r
}
```

## 6. フロントエンド実装詳細

（文書2のHTML/CSS/JSコード例をベースとし、APIリクエストボディを 4.B.1 の定義に合わせるように修正）

### A. HTML構造 (`static/index.html`)
```html
<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Note記事下書きジェネレーター</title>
    <link rel="stylesheet" href="css/style.css">
    <script src="https://cdnjs.cloudflare.com/ajax/libs/marked/4.3.0/marked.min.js"></script>
</head>
<body>
    <div class="container">
        <header>
            <h1>Note記事下書きジェネレーター</h1>
            <p>Note記事URLと指示から、AIが新しい記事の下書きを生成します</p>
        </header>

        <main>
            <div class="input-section">
                <div class="form-group">
                    <label for="note-url">参照するNote記事URL</label>
                    <input type="url" id="note-url" placeholder="https://note.com/..." required>
                </div>

                <div class="form-group">
                    <label for="keywords">キーワード (カンマ区切り)</label>
                    <input type="text" id="keywords" placeholder="例: AI, 生産性向上, 未来">
                </div>

                <div class="form-group">
                    <label for="theme">主なテーマ・視点</label>
                    <input type="text" id="theme" placeholder="例: 中小企業におけるAI導入のメリット">
                </div>

                <div class="form-group">
                    <label for="target-audience">想定読者層</label>
                    <input type="text" id="target-audience" placeholder="例: 中小企業の経営者">
                </div>

                 <div class="form-group">
                    <label for="exclusions">含めないでほしい内容</label>
                    <input type="text" id="exclusions" placeholder="例: 専門的すぎる技術詳細">
                </div>

                <div class="form-group-inline">
                    <div class="form-group">
                        <label for="style-choice">文体</label>
                        <select id="style-choice">
                            <option value="ですます調" selected>ですます調</option>
                            <option value="である調">である調</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="tone-choice">トーン</label>
                        <select id="tone-choice">
                            <option value="客観的" selected>客観的</option>
                            <option value="情熱的">情熱的</option>
                            <option value="ユーモラス">ユーモラス</option>
                            <option value="前向き">前向き</option>
                            <option value="丁寧">丁寧</option>
                            <option value="具体的">具体的</option>
                            <option value="親しみやすい">親しみやすい</option>
                        </select>
                    </div>
                     <div class="form-group">
                        <label for="word-count">目標文字数</label>
                        <select id="word-count">
                            <option value="1000">短め (~1000字)</option>
                            <option value="1500" selected>中程度 (~1500字)</option>
                            <option value="2000">やや長め (~2000字)</option>
                            <option value="3000">長め (~3000字)</option>
                        </select>
                    </div>
                </div>
                <!-- オプション: 文体分析用ユーザー名入力 -->
                <!--
                <div class="form-group">
                    <label for="ref-username-style">文体参考ユーザー名 (オプション)</label>
                    <input type="text" id="ref-username-style" placeholder="例: note_user_name">
                </div>
                -->

                <button id="generate-btn" class="primary-btn">記事下書きを生成</button>
            </div>

            <div id="loading" class="hidden">
                <div class="spinner"></div>
                <p>記事を生成中です…</p>
                <p class="small">※AIモデルの処理には1分程度かかる場合があります</p>
            </div>

            <div id="result-section" class="hidden">
                 <h2>生成された下書き</h2>
                <div class="tabs">
                    <button class="tab-btn active" data-tab="preview">プレビュー</button>
                    <button class="tab-btn" data-tab="markdown">マークダウン</button>
                </div>

                <div class="tab-content">
                     <div id="preview-tab" class="tab-pane active">
                        <div id="preview-content"></div>
                    </div>
                    <div id="markdown-tab" class="tab-pane">
                        <textarea id="markdown-output" readonly></textarea>
                        <button id="copy-btn" class="secondary-btn">コピー</button>
                    </div>
                </div>
            </div>

            <div id="error-message-area"></div>

        </main>

        <footer>
            <p>© 2025 Note記事下書きジェネレーター</p>
        </footer>
    </div>

    <script src="js/script.js"></script>
</body>
</html>
```

### B. CSSスタイリング (`static/css/style.css`)
（文書2のCSS例を流用、または必要に応じて調整。`.form-group-inline` などのスタイルを追加）
```css
/* ... (文書2のCSSをベースに追加・修正) ... */
.form-group-inline {
    display: flex;
    gap: 1rem; /* 要素間のスペース */
    flex-wrap: wrap; /* 必要に応じて折り返し */
    margin-bottom: 1rem;
}

.form-group-inline .form-group {
    flex: 1; /* 要素を均等に配置 */
    min-width: 150px; /* 最小幅 */
    margin-bottom: 0; /* 下マージンを削除 */
}

/* エラーメッセージエリア */
#error-message-area {
    margin-top: 1rem;
}

.error-message {
    background-color: #fdecea;
    color: var(--error-color);
    padding: 0.75rem 1rem;
    border: 1px solid #f5c6cb;
    border-left: 4px solid var(--error-color);
    border-radius: 4px;
    margin-bottom: 1rem;
}
/* ... (他は文書2と同様) ... */
```

### C. JavaScriptロジック (`static/js/script.js`)
```javascript
document.addEventListener('DOMContentLoaded', function() {
    // 要素の取得
    const noteUrlInput = document.getElementById('note-url');
    const keywordsInput = document.getElementById('keywords');
    const themeInput = document.getElementById('theme');
    const targetAudienceInput = document.getElementById('target-audience');
    const exclusionsInput = document.getElementById('exclusions');
    const styleChoiceSelect = document.getElementById('style-choice');
    const toneChoiceSelect = document.getElementById('tone-choice');
    const wordCountSelect = document.getElementById('word-count');
    // const refUsernameStyleInput = document.getElementById('ref-username-style'); // オプション
    const generateBtn = document.getElementById('generate-btn');
    const resultSection = document.getElementById('result-section');
    const markdownOutput = document.getElementById('markdown-output');
    const previewContent = document.getElementById('preview-content');
    const loadingDiv = document.getElementById('loading');
    const copyBtn = document.getElementById('copy-btn');
    const tabBtns = document.querySelectorAll('.tab-btn');
    const tabPanes = document.querySelectorAll('.tab-pane');
    const errorMessageArea = document.getElementById('error-message-area');

    // 記事生成ボタンクリック時の処理
    generateBtn.addEventListener('click', function() {
        clearErrorMessage(); // 既存のエラーメッセージをクリア
        const noteUrl = noteUrlInput.value.trim();
        const keywords = keywordsInput.value.trim().split(',').map(k => k.trim()).filter(k => k !== '');
        const theme = themeInput.value.trim();
        const targetAudience = targetAudienceInput.value.trim();
        const exclusions = exclusionsInput.value.trim();
        const styleChoice = styleChoiceSelect.value;
        const toneChoice = toneChoiceSelect.value;
        const wordCount = parseInt(wordCountSelect.value);
        // const refUsernameStyle = refUsernameStyleInput.value.trim(); // オプション

        // 入力検証
        if (!noteUrl) {
            showErrorMessage('Note記事URLを入力してください');
            return;
        }
        try {
            new URL(noteUrl); // URL形式の簡易チェック
            if (!noteUrl.includes('note.com')) {
                showErrorMessage('有効なNote記事URLを入力してください');
                return;
            }
        } catch (_) {
            showErrorMessage('有効なURL形式で入力してください');
            return;
        }

        // APIリクエストデータ構築
        const requestData = {
            note_url: noteUrl,
            keywords: keywords,
            theme: theme,
            target_audience: targetAudience,
            exclusions: exclusions,
            style_choice: styleChoice,
            tone_choice: toneChoice,
            word_count: wordCount,
            // reference_username_for_style: refUsernameStyle // オプション
        };

        // 記事の生成開始
        generateArticle(requestData);
    });

    // 記事の生成
    function generateArticle(requestData) {
        loadingDiv.classList.remove('hidden');
        resultSection.classList.add('hidden');
        generateBtn.disabled = true; // ボタンを無効化

        fetch('/api/generate', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(requestData)
        })
        .then(async response => { // async追加でレスポンスボディを読みやすく
            if (!response.ok) {
                const errorData = await response.json().catch(() => ({ // JSONパース失敗時のフォールバック
                     error: { message: `サーバーエラーが発生しました (Status: ${response.status})` }
                }));
                // errorData.error が存在しない場合も考慮
                const errorMessage = errorData.error?.message || `サーバーエラーが発生しました (Status: ${response.status})`;
                throw new Error(errorMessage);
            }
            return response.json();
        })
        .then(data => {
            if (data.draft) {
                displayGeneratedArticle(data.draft);
            } else {
                // APIは200 OKでもdraftがない場合（ありえないはずだが念のため）
                 showErrorMessage('生成された記事が見つかりません');
            }
        })
        .catch(error => {
            console.error('Generation failed:', error);
            showErrorMessage(`記事の生成に失敗しました: ${error.message}`);
        })
        .finally(() => {
            loadingDiv.classList.add('hidden');
            generateBtn.disabled = false; // ボタンを有効化
        });
    }

    // 生成された記事の表示
    function displayGeneratedArticle(markdown) {
        markdownOutput.value = markdown;
        previewContent.innerHTML = marked.parse(markdown); // marked.jsでプレビュー
        resultSection.classList.remove('hidden');

        // 結果表示時にプレビュータブをアクティブにする
        setActiveTab('preview');

        resultSection.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }

    // エラーメッセージの表示
    function showErrorMessage(message) {
        const errorDiv = document.createElement('div');
        errorDiv.className = 'error-message';
        errorDiv.textContent = message;
        errorMessageArea.appendChild(errorDiv);
    }

    // エラーメッセージのクリア
    function clearErrorMessage() {
        errorMessageArea.innerHTML = '';
    }

    // タブ切り替え
    tabBtns.forEach(btn => {
        btn.addEventListener('click', function() {
            const tabId = this.getAttribute('data-tab');
            setActiveTab(tabId);
        });
    });

    // 特定のタブをアクティブにする関数
    function setActiveTab(tabId) {
         tabBtns.forEach(b => b.classList.remove('active'));
         document.querySelector(`.tab-btn[data-tab="${tabId}"]`).classList.add('active');

         tabPanes.forEach(pane => pane.classList.remove('active'));
         document.getElementById(`${tabId}-tab`).classList.add('active');
    }


    // コピーボタン
    copyBtn.addEventListener('click', function() {
        if (navigator.clipboard && window.isSecureContext) {
             // navigator.clipboard API推奨 (HTTPSまたはlocalhost)
            navigator.clipboard.writeText(markdownOutput.value).then(() => {
                 showCopySuccessMessage(this);
            }).catch(err => {
                console.error('Clipboard copy failed:', err);
                // フォールバック (古い方法)
                fallbackCopyTextToClipboard(markdownOutput.value, this);
            });
        } else {
            // 古いブラウザやHTTP用のフォールバック
            fallbackCopyTextToClipboard(markdownOutput.value, this);
        }
    });

    function fallbackCopyTextToClipboard(text, buttonElement) {
        markdownOutput.select(); // テキストエリアを選択状態にする
        try {
            const successful = document.execCommand('copy');
            if (successful) {
                 showCopySuccessMessage(buttonElement);
            } else {
                 console.error('Fallback copy command failed');
                 alert('コピーに失敗しました。手動でコピーしてください。');
            }
        } catch (err) {
            console.error('Fallback copy error:', err);
            alert('コピーに失敗しました。手動でコピーしてください。');
        }
        window.getSelection().removeAllRanges(); // 選択解除
    }

    function showCopySuccessMessage(buttonElement) {
        const originalText = buttonElement.textContent;
        buttonElement.textContent = 'コピーしました！';
        buttonElement.disabled = true;
        setTimeout(() => {
            buttonElement.textContent = originalText;
            buttonElement.disabled = false;
        }, 2000);
    }
});
```

## 7. Gemini 2.5 Pro Preview プロンプト設計

### A. 設計思想
*   Gemini 2.5 Pro Preview の高度な文脈理解能力と指示追従能力を最大限に活用。
*   入力情報（元記事、ユーザー指示、文体分析結果(オプション)）を明確に構造化して提供。
*   出力形式（マークダウン）、文体、トーン、構成、禁止事項などを具体的に指示。
*   高品質な下書き生成を目指し、プロンプトは継続的に評価・改善。

### B. 基本構造
プロンプトは以下のセクションで構成される。
1.  **役割設定 (Role Setting):** モデルに期待する役割を定義。
2.  **タスク定義 (Task Definition):** 実行すべきタスクを明確に指示。
3.  **入力コンテキスト (Input Context):**
    *   参照元Note記事 (タイトルと本文)
    *   ユーザー指示 (キーワード、テーマ、読者層、除外事項)
    *   (オプション) 文体分析結果
4.  **出力指示 (Output Instructions):**
    *   生成する記事の構成・形式 (マークダウン、見出し、リスト等)
    *   スタイル・トーン指定 (文体、語調)
    *   文字数・長さの目安
    *   制約・禁止事項 (コピペ禁止、特定の表現の使用/不使用)
    *   ハッシュタグの要求

### C. 構成要素詳細
各構成要素を具体的に記述する。

1.  **役割設定:** `あなたは、読者の関心を引きつけ、分かりやすく情報を伝えるプロのコンテンツライターです。` や `あなたは、{TargetAudience} 向けの専門的な内容を、指定されたトーンで解説するエキスパートです。` のように具体的に。
2.  **タスク定義:** `以下の入力コンテキストに基づき、新しいNote記事の下書きを生成してください。`
3.  **入力コンテキスト:**
    *   **参照元Note記事:**
        ```
        # 参照元Note記事
        ## タイトル
        {OriginalArticleTitle}
        ## 本文
        {OriginalArticleBody}
        ---
        ```
    *   **ユーザー指示:**
        ```
        # ユーザー指示
        * キーワード: {Keywords}
        * 主なテーマ/視点: {Theme}
        * 想定読者層: {TargetAudience}
        * 含めないでほしい内容: {Exclusions}
        ---
        ```
    *   **(オプション) 文体分析結果:**
        ```
        # (参考) 文体特徴
        * 平均段落長: {StyleAnalysis.ParagraphLength}
        * 平均文長: 約 {StyleAnalysis.AvgSentenceLength} 文字
        * よく使われる表現: {StyleAnalysis.CommonExpressions}
        * トーン: {StyleAnalysis.Tone}
        * 特徴的な言い回し: {StyleAnalysis.UniqueExpressions}
        * よく使うハッシュタグ: {StyleAnalysis.CommonHashtags}
        ---
        ```
4.  **出力指示:**
    *   **構成・形式:** `以下の構成で、マークダウン形式で記述してください。\n1. 導入 (読者の興味を引くフック)\n2. 本論 (指示されたキーワードやテーマを盛り込み、複数のセクションに分けて説明。必要に応じて ## や ### の見出しを使用)\n3. 結論 (記事全体の要約と行動喚起)\n各セクションは、参照元の記事本文や文体特徴({StyleAnalysis.ParagraphLength}など)を参考に、適切な長さで記述してください。`
    *   **スタイル・トーン:** `文体は「{StyleChoice}」、トーンは「{ToneChoice}」を基本としてください。` (文体分析結果がある場合は `特に、参照元の文体特徴（トーン: {StyleAnalysis.Tone}, よく使われる表現: {StyleAnalysis.CommonExpressions}）を意識してください。` を追加)
    *   **文字数・長さ:** `全体の文字数は約{WordCount}字を目安にしてください。` (厳密な制御は難しいことに注意)
    *   **制約・禁止事項:** `参照元記事の表現をそのままコピー＆ペーストすることは避けてください。指定されたテーマと読者層に合わせて内容を再構成・加筆してください。{Exclusions}で指定された内容は含めないでください。`
    *   **ハッシュタグ:** `記事の最後に、内容と関連性の高いハッシュタグを3〜5個、#タグ の形式でリストしてください。` (文体分析結果がある場合は `(参考: {StyleAnalysis.CommonHashtags})` を追加)

### D. プロンプト構築関数 (`buildPrompt`) とプロンプト例

```go
package gemini

import (
	"fmt"
	"strings"
	"your_project_path/analyzer" // analyzer パッケージのパス
)

// プロンプト構築関数
func buildPrompt(input ArticleGenerationInput) string {
	var prompt strings.Builder

	// 1. 役割設定
	prompt.WriteString("あなたは、読者の関心を引きつけ、分かりやすく情報を伝えるプロのコンテンツライターです。\n")
	if input.TargetAudience != "" {
		prompt.WriteString(fmt.Sprintf("特に、%s向けの解説を得意としています。\n", input.TargetAudience))
	}
	prompt.WriteString("\n")

	// 2. タスク定義
	prompt.WriteString("以下の入力コンテキストに基づき、新しいNote記事の下書きを生成してください。\n\n")

	// 3. 入力コンテキスト
	prompt.WriteString("--- 入力コンテキスト ---\n")
	// 参照元記事
	prompt.WriteString("# 参照元Note記事\n")
	prompt.WriteString(fmt.Sprintf("## タイトル\n%s\n", input.OriginalArticleTitle))
	prompt.WriteString(fmt.Sprintf("## 本文\n%s\n", input.OriginalArticleBody)) // 長すぎる場合は要約や抜粋を検討
	prompt.WriteString("---\n")

	// ユーザー指示
	prompt.WriteString("# ユーザー指示\n")
	if len(input.Keywords) > 0 {
		prompt.WriteString(fmt.Sprintf("* キーワード: %s\n", strings.Join(input.Keywords, ", ")))
	}
	if input.Theme != "" {
		prompt.WriteString(fmt.Sprintf("* 主なテーマ/視点: %s\n", input.Theme))
	}
	if input.TargetAudience != "" {
		prompt.WriteString(fmt.Sprintf("* 想定読者層: %s\n", input.TargetAudience))
	}
	if input.Exclusions != "" {
		prompt.WriteString(fmt.Sprintf("* 含めないでほしい内容: %s\n", input.Exclusions))
	}
	prompt.WriteString("---\n")

	// (オプション) 文体分析結果
	if input.StyleAnalysis != nil {
		prompt.WriteString("# (参考) 文体特徴\n")
		if input.StyleAnalysis.ParagraphLength != "" {
			prompt.WriteString(fmt.Sprintf("* 平均段落長: %s\n", input.StyleAnalysis.ParagraphLength))
		}
		if input.StyleAnalysis.AvgSentenceLength > 0 {
			prompt.WriteString(fmt.Sprintf("* 平均文長: 約 %d 文字\n", input.StyleAnalysis.AvgSentenceLength))
		}
		if len(input.StyleAnalysis.CommonExpressions) > 0 {
			prompt.WriteString(fmt.Sprintf("* よく使われる表現: %s\n", strings.Join(input.StyleAnalysis.CommonExpressions, ", ")))
		}
		if input.StyleAnalysis.Tone != "" {
			prompt.WriteString(fmt.Sprintf("* トーン: %s\n", input.StyleAnalysis.Tone))
		}
		if len(input.StyleAnalysis.UniqueExpressions) > 0 {
			prompt.WriteString(fmt.Sprintf("* 特徴的な言い回し: %s\n", strings.Join(input.StyleAnalysis.UniqueExpressions, ", ")))
		}
		if len(input.StyleAnalysis.CommonHashtags) > 0 {
			prompt.WriteString(fmt.Sprintf("* よく使うハッシュタグ: %s\n", strings.Join(input.StyleAnalysis.CommonHashtags, ", ")))
		}
		prompt.WriteString("---\n")
	}
	prompt.WriteString("\n") // 区切り

	// 4. 出力指示
	prompt.WriteString("--- 出力指示 ---\n")
	prompt.WriteString("以下の指示に従って、記事の下書きを作成してください。\n\n")

	// 構成・形式
	prompt.WriteString("1.  **形式:** 全体を **必ず** 有効なマークダウン形式で出力してください。\n")
	prompt.WriteString("2.  **タイトル:** 記事の最初に `# 新しいタイトル` の形式で、内容に合った魅力的なタイトルを付けてください。\n")
	// prompt.WriteString("3.  **アイキャッチ画像プレースホルダー:** タイトルの直後か最初の段落の後に `[ここにアイキャッチ画像を挿入]` というテキストを入れてください。\n") // 必要に応じて
	prompt.WriteString("3.  **構成:**\n")
	prompt.WriteString("    *   導入: 読者の興味を引きつけ、記事のテーマと目的を明確に示す。\n")
	prompt.WriteString("    *   本論: 指示されたキーワードやテーマを盛り込み、論理的な流れで複数のセクション（`## 見出し` や `### 小見出し` を使用）に分けて説明する。\n")
	prompt.WriteString("    *   結論: 記事全体の要点をまとめ、読者へのメッセージや次のアクションを促す。\n")
	if input.StyleAnalysis != nil && input.StyleAnalysis.ParagraphLength != "" {
		prompt.WriteString(fmt.Sprintf("    *   段落の長さは、参照元の記事の傾向（平均: %s）を参考にしてください。\n", input.StyleAnalysis.ParagraphLength))
	}
	prompt.WriteString("4.  **スタイル・トーン:**\n")
	prompt.WriteString(fmt.Sprintf("    *   文体は「%s」、トーンは「%s」を基本としてください。\n", input.StyleChoice, input.ToneChoice))
	if input.TargetAudience != "" {
		prompt.WriteString(fmt.Sprintf("    *   想定読者層（%s）が理解しやすい言葉遣いを心がけてください。\n", input.TargetAudience))
	}
	if input.StyleAnalysis != nil && input.StyleAnalysis.Tone != "" {
		prompt.WriteString(fmt.Sprintf("    *   可能であれば、参照元の文体特徴（トーン: %s）も意識してください。\n", input.StyleAnalysis.Tone))
	}
	prompt.WriteString("5.  **文字数:** 全体の文字数は約 %d 字を目安にしてください。\n", input.WordCount)
	prompt.WriteString("6.  **制約・禁止事項:**\n")
	prompt.WriteString("    *   **参照元記事の表現をそのままコピー＆ペーストすることは絶対に避けてください。** 参照元はあくまで内容の参考とし、指定されたテーマと読者層に合わせて、あなた自身の言葉で内容を再構成・加筆してください。\n")
	if input.Exclusions != "" {
		prompt.WriteString(fmt.Sprintf("    *   「%s」に関する内容は含めないでください。\n", input.Exclusions))
	}
	prompt.WriteString("7.  **ハッシュタグ:** 記事の最後に、記事内容に最も関連性の高いハッシュタグを3〜5個、`#タグ名` の形式でリストしてください。")
	if input.StyleAnalysis != nil && len(input.StyleAnalysis.CommonHashtags) > 0 {
		prompt.WriteString(fmt.Sprintf(" (参考にすべきハッシュタグ: %s)", strings.Join(input.StyleAnalysis.CommonHashtags, ", ")))
	}
	prompt.WriteString("\n\n")

	prompt.WriteString("それでは、上記の指示に基づいて記事の下書きの生成を開始してください。")

	return prompt.String()
}
```

**プロンプト例:** (上記の`buildPrompt`関数によって生成されるテキストのイメージ)
```text
あなたは、読者の関心を引きつけ、分かりやすく情報を伝えるプロのコンテンツライターです。
特に、中小企業の経営者向けの解説を得意としています。

以下の入力コンテキストに基づき、新しいNote記事の下書きを生成してください。

--- 入力コンテキスト ---
# 参照元Note記事
## タイトル
AI活用で変わる未来の働き方
## 本文
近年、AI技術は目覚ましい発展を遂げており、私たちの働き方に大きな影響を与え始めています...（元記事本文が続く）...
---
# ユーザー指示
* キーワード: AI, 生産性向上, 中小企業
* 主なテーマ/視点: 元記事の内容を踏まえつつ、特に中小企業におけるAI導入の具体的なメリットと導入時の注意点に焦点を当てる。
* 想定読者層: 中小企業の経営者
* 含めないでほしい内容: 専門的すぎる技術詳細、大規模言語モデルの仕組み
---

--- 出力指示 ---
以下の指示に従って、記事の下書きを作成してください。

1.  **形式:** 全体を **必ず** 有効なマークダウン形式で出力してください。
2.  **タイトル:** 記事の最初に `# 新しいタイトル` の形式で、内容に合った魅力的なタイトルを付けてください。
3.  **構成:**
    *   導入: 読者の興味を引きつけ、記事のテーマと目的を明確に示す。
    *   本論: 指示されたキーワードやテーマを盛り込み、論理的な流れで複数のセクション（`## 見出し` や `### 小見出し` を使用）に分けて説明する。
    *   結論: 記事全体の要点をまとめ、読者へのメッセージや次のアクションを促す。
4.  **スタイル・トーン:**
    *   文体は「ですます調」、トーンは「前向き」を基本としてください。
    *   想定読者層（中小企業の経営者）が理解しやすい言葉遣いを心がけてください。
5.  **文字数:** 全体の文字数は約 1500 字を目安にしてください。
6.  **制約・禁止事項:**
    *   **参照元記事の表現をそのままコピー＆ペーストすることは絶対に避けてください。** 参照元はあくまで内容の参考とし、指定されたテーマと読者層に合わせて、あなた自身の言葉で内容を再構成・加筆してください。
    *   「専門的すぎる技術詳細、大規模言語モデルの仕組み」に関する内容は含めないでください。
7.  **ハッシュタグ:** 記事の最後に、記事内容に最も関連性の高いハッシュタグを3〜5個、`#タグ名` の形式でリストしてください。

それでは、上記の指示に基づいて記事の下書きの生成を開始してください。
```

### E. モデルパラメータ調整 (Vertex AI `GenerationConfig`)
*   `temperature`: (推奨: 0.6 - 0.8) 創造性と一貫性のバランスを取る。初期値 0.7。
*   `maxOutputTokens`: (推奨: 1024 - 4096) 生成する記事の長さに合わせて設定。コストに直結するため、`word_count` から適切な値を計算（例: 日本語1文字≒1.5-2トークンとして余裕を持たせる）。初期値 4096。
*   `topP`: (推奨: 0.9 - 0.95) 多様性の制御。初期値 0.95。
*   `topK`: (推奨: 40) 考慮するトークン数。初期値 40。
*   `stopSequences`: (任意) 特定の文字列で生成を停止させたい場合。

### F. プロンプトエンジニアリング戦略
*   **Few-Shot Learning:** (任意) 複雑な構成や特定の言い回しを要求する場合、プロンプト内に簡単な「期待される出力例」を1〜数個含める。
*   **Chain-of-Thought (CoT) Prompting:** (任意、Proモデル向け) 複雑な論理展開が必要な場合、「まず〇〇をリストアップし、次に△△について論じ、最後に□□でまとめてください」のように思考プロセスを段階的に指示する。
*   **継続的改善:** 生成結果を定期的に評価し、プロンプトやパラメータを微調整するPDCAサイクルを回す。

## 8. 非機能要件

### A. パフォーマンス
*   **応答時間:** APIリクエスト受信からレスポンス返却まで、通常ケースで**30秒以内**を目指す (Note記事取得時間 + Gemini API処理時間を含む)。Gemini 2.5 Pro Preview の処理時間は変動するため、フロントエンドでのタイムアウトは長め（例: 60-90秒）に設定。
*   **スループット:** 同時接続ユーザー数 X人、秒間 Yリクエスト（想定利用規模に基づき決定）。

### B. セキュリティ
*   **APIキー管理:** Gemini APIキー、その他外部サービスキーは環境変数またはシークレット管理サービスで安全に管理し、コードにハードコードしない。
*   **入力サニタイズ:** ユーザー入力（特にURL）は適切に検証・サニタイズし、インジェクション攻撃やSSRFを防ぐ。
*   **依存ライブラリ:** 脆弱性情報を定期的に確認し、必要に応じてアップデート (Dependabot等の利用推奨)。
*   **レート制限:** APIエンドポイントに適切なレート制限を設け、DoS攻撃や乱用を防ぐ。
*   **Note API利用:** 非公式API利用のリスクを認識し、過度な負荷をかけないようアクセス間隔を調整。スクレイピングはrobots.txtを尊重する。

### C. 可用性
*   システムの目標稼働率: XX.X% (サービスレベルに応じて決定)。
*   依存サービス（Note API, Gemini API）障害時のハンドリング: 適切なエラーメッセージを返し、システム全体が停止しないようにする。リトライ処理の実装。

### D. 保守性
*   Go言語の標準的な規約に従い、可読性・再利用性の高いコードを記述 (`gofmt`, `golint` / `staticcheck` の活用)。
*   適切なコメント、ドキュメンテーション (godoc形式)。
*   ユニットテスト、インテグレーションテストを実装し、コード品質を維持。CI/CDパイプラインの導入。
*   非推奨パッケージ (`io/ioutil`等) の使用回避。

### E. 運用・監視
*   **ロギング:** 構造化ログ (`log/slog`) を使用し、リクエスト/レスポンス情報、処理時間、エラー情報、Gemini API利用状況（トークン数見積もり等）を記録。
*   **モニタリング:** リソース使用率（CPU, Memory）、API応答時間、エラーレートを監視。
*   **アラート:** 異常検知時（エラーレート急増、応答時間悪化、Gemini APIコスト急増）のアラート通知設定。

### F. APIキー管理
（B. セキュリティ と重複するが再掲）Gemini APIキー等は環境変数やシークレット管理ツール（例: Google Secret Manager, HashiCorp Vault）で管理する。

## 9. デプロイメント

### A. ディレクトリ構成 (例)
```
note-article-generator/
├── cmd/server/main.go       # アプリケーションエントリーポイント
├── internal/                 # 内部パッケージ
│   ├── handlers/             # HTTPハンドラ
│   │   └── generate.go
│   ├── services/             # ビジネスロジック
│   │   ├── note/             # Note API連携
│   │   │   └── client.go
│   │   ├── analyzer/         # 文体分析 (オプション)
│   │   │   └── style.go
│   │   └── generator/        # Gemini API連携、プロンプト構築
│   │       └── gemini.go
│   └── config/               # 設定読み込み
│       └── config.go
├── static/                   # 静的ファイル (フロントエンド)
│   ├── index.html
│   ├── css/style.css
│   └── js/script.js
├── .env                      # 環境変数 (開発用, .gitignore対象)
├── go.mod
├── go.sum
├── Dockerfile                # Dockerビルド用
└── README.md
```

### B. 環境変数の設定
以下の環境変数が必要。
*   `PORT`: サーバーがリッスンするポート (例: `8080`)
*   `GOOGLE_PROJECT_ID`: Google Cloud Project ID
*   `GOOGLE_LOCATION`: Vertex AI を使用するリージョン (例: `us-central1`)
*   `GOOGLE_APPLICATION_CREDENTIALS`: (推奨)サービスアカウントキーファイルへのパス (環境変数設定で認証する場合)

`.env` ファイル (開発環境用):
```
PORT=8080
GOOGLE_PROJECT_ID=your-gcp-project-id
GOOGLE_LOCATION=us-central1
# GOOGLE_APPLICATION_CREDENTIALS=/path/to/your/service-account-key.json (ローカル実行時)
```

### C. ビルドとデプロイ (ローカル, Docker)

**ローカル開発環境:**
```bash
# 依存関係インストール
go mod tidy

# 環境変数を設定 (direnv や export コマンドなど)
export $(grep -v '^#' .env | xargs)

# サーバー起動
go run cmd/server/main.go
```

**Docker:**
```Dockerfile
# --- Builder Stage ---
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Goモジュールをキャッシュ
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# アプリケーションのビルド
# (cmd/server/main.go がエントリーポイントの場合)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server cmd/server/main.go

# --- Final Stage ---
FROM alpine:latest

# ca-certificatesを追加 (HTTPS通信用)
RUN apk --no-cache add ca-certificates

WORKDIR /app

# ビルドされたバイナリをコピー
COPY --from=builder /app/server /app/server
# 静的ファイルをコピー
COPY --from=builder /app/static ./static

# ポートを開放
EXPOSE 8080

# (オプション) 非rootユーザーで実行する場合
# RUN addgroup -S appgroup && adduser -S appuser -G appgroup
# USER appuser

# 環境変数 (コンテナ実行時に設定)
# ENV PORT=8080
# ENV GOOGLE_PROJECT_ID=your-gcp-project-id
# ENV GOOGLE_LOCATION=us-central1
# (サービスアカウントキーはVolumeマウントやSecret管理で渡すのが一般的)

# コンテナ起動コマンド
CMD ["/app/server"]
```
**Dockerビルド＆実行:**
```bash
# イメージビルド
docker build -t note-article-generator:latest .

# コンテナ実行 (環境変数を渡す)
docker run -p 8080:8080 \
  -e PORT=8080 \
  -e GOOGLE_PROJECT_ID="your-gcp-project-id" \
  -e GOOGLE_LOCATION="us-central1" \
  -e GOOGLE_APPLICATION_CREDENTIALS="/path/inside/container/to/key.json" \
  -v /path/on/host/to/key.json:/path/inside/container/to/key.json:ro \
  note-article-generator:latest
```
(注: サービスアカウントキーの扱いはデプロイ環境に合わせてください。VolumeマウントよりWorkload Identity等の方が安全です。)

## 10. テスト戦略
*   **ユニットテスト:** 各パッケージ・関数（特に Note API クライアント、文体分析ロジック、プロンプト構築、APIハンドラ内のロジック）に対して `_test.go` ファイルを作成し、テストケースを記述する。
    *   外部API（Note, Gemini）への依存はモック化する (HTTPテストサーバー、インターフェースとモック実装など)。
*   **インテグレーションテスト:** 実際にAPIサーバーを起動し、`/api/generate` エンドポイントにリクエストを送信してレスポンスを検証する。
    *   必要に応じて、テスト用のNote記事やGemini APIの挙動を模倣する仕組みを導入。
*   **カバレッジ:** テストカバレッジを計測し、主要なロジックがテストされていることを確認する。
*   **CI:** GitHub Actions等で、コードプッシュ時に自動でテストを実行する。

## 11. 付録

### A. 参考資料
*   Google AI Gemini Model Documentation (Vertex AI): [https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models](https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models)
*   Google AI Gemini API Documentation: [https://ai.google.dev/gemini-api/docs](https://ai.google.dev/gemini-api/docs)
*   Go Cloud Client Libraries for Vertex AI: [https://pkg.go.dev/cloud.google.com/go/vertexai/apiv1](https://pkg.go.dev/cloud.google.com/go/vertexai/apiv1)
*   Go `log/slog` package: [https://pkg.go.dev/log/slog](https://pkg.go.dev/log/slog)
*   goquery (HTML Parser): [https://github.com/PuerkitoBio/goquery](https://github.com/PuerkitoBio/goquery)
*   marked.js (Markdown Parser): [https://marked.js.org/](https://marked.js.org/)
*   (参照元) Note APIに関する非公式情報源: [https://note.egg-glass.jp/.../noteAPI.html](https://note.egg-glass.jp/.../noteAPI.html)

### B. Note API仕様に関する補足
*   本ドキュメントで参照している Note API v2 および v3 は、note株式会社から公式に提供・サポートされているものではありません。
*   APIのエンドポイント、リクエスト/レスポンス形式、利用可否は予告なく変更される可能性があります。
*   利用にあたっては、Noteの利用規約を遵守し、サーバーに過度な負荷を与えないよう注意が必要です (適切なアクセス間隔の設定、エラーハンドリング、再試行ロジックの実装)。
*   API v3 が安定して利用できない、または利用規約上の問題がある場合は、Webスクレイピングが主要な記事本文取得手段となりますが、これもNote側のHTML構造変更により容易に動作しなくなるリスクがあります。
*   本番環境での運用においては、これらのリスクを十分に考慮し、定期的な動作確認とメンテナンス計画が必要です。