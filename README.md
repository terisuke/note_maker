# Note記事下書きジェネレーター

Note記事のURLまたはユーザー名を入力し、ローカルLLMが新しい記事の下書きを生成するWebアプリケーションです。

今後の大規模改修方針は [ADR 0001](docs/adrs/0001-three-phase-local-article-generation.md) と [3フェーズ実装計画](docs/implementation-plans/three-phase-local-article-generation.md) に整理しています。単一フォームで記事を生成する方式から、文体分析、記事条件の一問一答、記事生成と評価の3フェーズへ移行します。

ADR 0001 を踏まえた次の進化方針は [ADR 0002 — Multi-Persona, Multi-Format Article Generation](docs/adrs/0002-multi-persona-multi-format-extension.md) と [Multi-persona / multi-format 実装計画](docs/implementation-plans/multi-persona-multi-format.md) にまとめています。てりすけ本人と架空キャラ「宇宙野クラウディア」を別人格として扱い、note / cor-jp.com ブログ / Zenn / Qiita / ホームページHTML を切り替え可能にします。

実装 issue と ADR の対応、各層の責務、テスト条件は [Issue and ADR guardrails](docs/implementation-plans/issue-adr-guardrails.md) にまとめています。現在のアプリ化後の引き継ぎ、起動方法、main昇格チェックリストは [Note Maker app handoff](docs/handoffs/app-handoff-2026-05-03.md) に整理しています。次に着手する実装順は [Next implementation cut](docs/implementation-plans/next-implementation-cut.md) に整理しています。

## 主な機能

- Note記事URLまたはユーザー名からの記事生成
- キーワード、テーマ、読者層の指定
- 文体、トーン、目標文字数、記事目的の指定
- 記事構成のカスタマイズ
- マークダウン形式での出力、プレビュー、コピー

## 使用している技術

- フロントエンド: HTML, CSS, JavaScript
- バックエンド: Go
- LLM runtime: Evo X2 Ollama OpenAI互換API over Tailnetをprimary、Evo X2 llama.cppと作業端末ローカル llama.cppをfallback
- 主要モデル: `gemma4:e2b`（文体/ソース整理）、`qwen3.6:27b`（深掘り質問）、`gemma4:31b`（日本語下書き）、`gemma4:latest`（最終検証）
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

### アプリ風 launcher で起動する

通常利用は launcher 経由を推奨します。launcher は空いているローカルポートを選び、Evo X2 Tailnet の primary LLM health を確認し、Go サーバーをビルドして起動し、終了時に子プロセスを停止します。既定では Mac 側のローカル fallback LLM は起動しません。

```bash
make launcher
```

または mise を使う場合:

```bash
mise run launcher
```

`make app` も同じ launcher を起動する alias です。

起動後はブラウザが自動で開きます。終了するときは launcher を実行したターミナルで `Ctrl-C` を押します。`PORT` が使用中の場合は、既定で次の空きポートを選びます。固定ポートで失敗させたい場合は `./scripts/launcher.sh --strict-port` を使います。

launcher の既定保存先:

- macOS: `~/Library/Application Support/Note Maker`
- Linux/その他: `$XDG_DATA_HOME/note-maker` または `~/.local/share/note-maker`

この配下に `app_config.json`、`workflow_store.db`、`logs/`、ビルド済みサーバーバイナリを置きます。保存先を変える場合は `NOTE_MAKER_DATA_DIR=/path/to/dir make launcher` または `./scripts/launcher.sh --data-dir /path/to/dir` を指定します。旧 JSON store が同じディレクトリの `workflow_store.json` にある場合、SQLite store が空の初回起動時に既存の文体・取材・ブリーフ・カスタム書き手を取り込みます。

Evo X2 Tailnet が到達不能な場合、既定ではアプリを起動しません。UIだけを起動したい検証時は `make launcher-status` で状態を確認し、必要に応じて `./scripts/launcher.sh --allow-degraded` を使います。ローカル `llama-server` を明示的に起動して primary として使う場合だけ、次を実行します。

```bash
make launcher-local
```

`llama-server` の場所やモデルを変える場合は `.env` の `LLAMA_SERVER`、`LLAMACPP_HF_REPO`、`LLAMACPP_HF_FILE`、`LLAMACPP_MODEL` を変更します。

launcher 自体の検証:

```bash
make launcher-check
```

### 旧 dev script でまとめて起動する

従来の `scripts/dev.sh` は残しています。`LLM_RUNTIME=local` を明示した検証では `llama-server` と Go サーバーをまとめて起動できます。

```bash
make dev
```

ブラウザで `http://localhost:8080` にアクセスします。終了するときは `Ctrl-C` で両方のプロセスを停止できます。

### Evo X2 の Ollama を Tailscale VPN 経由で使って起動する

Evo X2 の Ollama を使う場合は、Tailscale VPN/MagicDNS 上の OpenAI互換APIを primary とし、Mac側のローカルLLMは起動しません。fallback は順番を固定します。

1. Evo X2 Ollama OpenAI互換API: `http://evo-x2.tailb30e58.ts.net/v1`
2. Evo X2 llama.cpp OpenAI互換API: `http://evo-x2.tailb30e58.ts.net/llama/v1`
3. 最終手段の作業端末ローカル llama.cpp: `http://127.0.0.1:8081/v1`

前提として、利用端末が同じ Tailnet に参加しており、Evo X2 の Caddy/OpenAI互換APIが Tailnet 内から到達できる必要があります。これにより、SSH の個別認証や端末ごとの port forward に依存せず、他の許可済みデバイスからも同じ Evo X2 を利用できます。

```bash
make evo-x2
```

または mise を使う場合:

```bash
mise trust
mise run evo-x2
```

既定では Tailnet 上の `http://evo-x2.tailb30e58.ts.net/v1` に接続します。モデルを変える場合は `.env.evo-x2.example` を参考に `LLM_MODEL`、`STYLE_LLM_MODEL`、`BRIEF_LLM_MODEL`、`ARTICLE_LLM_MODEL`、`DRAFT_LLM_MODEL`、`VERIFY_LLM_MODEL` を設定してください。120B級のモデルを使う場合は `LLM_TIMEOUT_SECONDS` を長めに設定します。

画面上部の「設定」から、フェーズ別に使うモデルと一問一答の質問を変更できます。質問は選択したペルソナと出力形式に応じた固定テンプレートが表示され、追加質問も下書き生成のブリーフに含まれます。

文体分析結果、取材セッションの回答、完成ブリーフ、記事・ドラフト履歴は `WORKFLOW_STORE_PATH` に永続化されます。既定値は SQLite の `data/workflow_store.db` です。

保存方式は設定画面の「保存方式」から選べます。UIで変更した内容は `data/app_config.json` に保存され、サーバー再起動後に反映されます。JSON store は互換性と軽量退避のため残していますが、プロジェクト・記事・ドラフト履歴まで安定して扱う既定は SQLite です。

開発・検証で強制したい場合は `WORKFLOW_STORE_DRIVER=sqlite` を指定できます。この環境変数がある場合、設定画面では保存方式がロック表示になります。

フェーズ別モデルの目安:

- `STYLE_LLM_MODEL`: Note記事取得後の文体ガイド整理用。既定は軽量な `gemma4:e2b`。
- `BRIEF_LLM_MODEL`: 深掘り質問生成用。既定は推論力重視の `qwen3.6:27b`。
- `ARTICLE_LLM_MODEL`: 旧 `/api/generate` 用。既定は軽量な `gemma4:e2b`。
- `DRAFT_LLM_MODEL`: 一問一答後の最終下書き生成用。既定は日本語下書き品質重視の `gemma4:31b`。
- `VERIFY_LLM_MODEL`: 下書き後の最終一貫性チェック用。既定は軽量な `gemma4:latest`。
- `EVO_X2_TAILNET_HOST`: Tailnet/MagicDNS 上の Evo X2 ホスト名です。既定値は `evo-x2.tailb30e58.ts.net`。
- `EVO_X2_LLM_BASE_URL`: Evo X2 Ollama primary の OpenAI互換APIです。既定値は `http://evo-x2.tailb30e58.ts.net/v1`。
- `LLM_FALLBACK_BASE_URLS`: カンマ区切りの fallback chain です。既定は Evo X2 llama.cpp、最後にローカル llama.cpp。
- `STYLE_LLM_FALLBACK_MODELS` など: fallback endpoint ごとのモデル名をカンマ区切りで指定します。

Tailnet primary の接続確認だけ行う場合:

```bash
make evo-x2-models
```

SSH tunnel を使った個人端末向け診断を行う場合だけ、明示的に SSH ターゲットを使います。これはアプリの primary 経路ではありません。

```bash
make evo-x2-ssh-models
```

3,000字前後の統合シナリオを Evo X2 で実行する場合:

```bash
make scenario-evo-x2
```

このシナリオは文体分析、一問一答、深掘り、下書き生成を通し、文体スコア80点以上と一定以上の本文量を確認します。

### ローカル llama.cpp fallback だけを検証する

Evo X2 primary とは別に、作業端末上で既に起動済みの `llama.cpp` fallback だけを検証する場合は専用ターゲットを使います。このターゲットは Ollama や `llama-server` を起動せず、Evo X2 への fallback chain も無効化します。

まず plan/report だけを作る場合:

```bash
make scenario-local-llamacpp-fallback
```

実際に draft 生成まで走らせる場合は、誤って Evo X2 primary 検証と混同しないように明示的な gate が必要です。`LOCAL_LLAMACPP_FALLBACK_BASE_URL` は loopback の OpenAI互換 `/v1` endpoint だけを受け付けます。

```bash
RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1 \
LOCAL_LLAMACPP_FALLBACK_BASE_URL=http://127.0.0.1:8081/v1 \
LOCAL_LLAMACPP_FALLBACK_MODEL=qwen3:30b-a3b \
LOCAL_LLAMACPP_FALLBACK_LOAD_FLAGS='--host 127.0.0.1 --port 8081 --alias qwen3:30b-a3b --reasoning off ...' \
make scenario-local-llamacpp-fallback
```

結果は `tmp/local_llamacpp_fallback/report.md` と `tmp/local_llamacpp_fallback/report.json` に記録されます。Issue #36 の合格条件は `score >= 82.0`、`keyword_overlap >= 70`、`runes >= 2800` です。記録テンプレートは `docs/validation/issue-36-local-llamacpp-fallback-template.md` です。

### 個別に起動する

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
