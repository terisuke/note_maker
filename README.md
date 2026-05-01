# Note記事下書きジェネレーター

Note記事のURLまたはユーザー名を入力し、ローカルLLMが新しい記事の下書きを生成するWebアプリケーションです。

今後の大規模改修方針は [ADR 0001](docs/adrs/0001-three-phase-local-article-generation.md) と [3フェーズ実装計画](docs/implementation-plans/three-phase-local-article-generation.md) に整理しています。単一フォームで記事を生成する方式から、文体分析、記事条件の一問一答、記事生成と評価の3フェーズへ移行します。

実装 issue と ADR の対応、各層の責務、テスト条件は [Issue and ADR guardrails](docs/implementation-plans/issue-adr-guardrails.md) にまとめています。

## 主な機能

- Note記事URLまたはユーザー名からの記事生成
- キーワード、テーマ、読者層の指定
- 文体、トーン、目標文字数、記事目的の指定
- 記事構成のカスタマイズ
- マークダウン形式での出力、プレビュー、コピー

## 使用している技術

- フロントエンド: HTML, CSS, JavaScript
- バックエンド: Go
- ローカルLLM: llama.cpp `llama-server`
- モデル: `gemma4:31b` alias for `ggml-org/gemma-4-31B-it-GGUF` Q4_K_M
- Note取得: 公開記事ページ/RSS優先、非公式APIは互換フォールバック

## 必要な環境

- Go 1.23以上
- llama.cpp の `llama-server`
- Gemma4 31B Q4_K_M を動かせるメモリまたはVRAM

`ggml-org/gemma-4-31B-it-GGUF` の Q4_K_M は約18.7GB級のモデルです。初回ダウンロードとモデルロードには時間がかかります。

## セットアップ

`.env` を作成します。

```bash
cp .env.example .env
```

`.env.example` の既定値:

```bash
PORT=8080
LLAMACPP_BASE_URL=http://127.0.0.1:8081/v1
LLAMACPP_MODEL=gemma4:31b
```

Gemma4 31B を `llama-server` で起動します。

```bash
llama-server \
  --hf-repo ggml-org/gemma-4-31B-it-GGUF \
  --hf-file gemma-4-31B-it-Q4_K_M.gguf \
  --alias gemma4:31b \
  --host 127.0.0.1 \
  --port 8081
```

別ターミナルでGoサーバーを起動します。

```bash
go run ./cmd/server
```

ブラウザで `http://localhost:8080` にアクセスします。

## API

### `POST /api/generate`

リクエスト:

```json
{
  "note_url": "https://note.com/example/n/n123",
  "username": "",
  "keywords": ["AI", "生産性"],
  "theme": "中小企業におけるローカルLLM活用",
  "target_audience": "中小企業の経営者",
  "exclusions": "過度に専門的な数式",
  "style_choice": "ですます調",
  "tone_choice": "客観的",
  "word_count": 1500,
  "article_purpose": "情報提供",
  "desired_content": "具体例を含める",
  "introduction_points": "読者の課題",
  "main_points": "導入手順と注意点",
  "conclusion_message": "小さく試す重要性"
}
```

`note_url` または `username` のどちらかと、`theme` が必須です。

成功レスポンス:

```json
{
  "draft": "生成されたMarkdown本文"
}
```

`draft` はNoteにそのまま貼り付けやすいよう、`# タイトル` から始まるMarkdown本文だけを返します。モデルが前置きやコードフェンスを返した場合は、サーバー側で正規化またはエラーとして扱います。

エラーレスポンス:

```json
{
  "error": {
    "code": "ARTICLE_GENERATION_FAILED",
    "message": "Failed to generate article",
    "details": "..."
  }
}
```

### `GET /api/models`

`llama-server` の `/v1/models` を呼び、利用可能なローカルモデルIDを返します。

## 注意事項

- このアプリケーションはGoogle Gemini APIを使用しません。
- Note本文取得は公開ページとRSSを優先します。note.comの非公式APIは仕様変更や利用制限のリスクがあります。
- 生成された記事は参考として使用し、公開前に必ず内容を確認してください。

## ライセンス

MIT
