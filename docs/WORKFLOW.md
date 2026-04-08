# Workflow System

## Overview

ワークフローはマルチステップの自動化タスクを定義・実行する仕組み。cron スケジュールで定期実行したり、CLI から手動実行できる。

各ステップは **prompt**（LLM で実行）か **script**（シェルスクリプト/Python で実行）のいずれかで、ステップ間の状態はファイルベースの共有ワークスペースで引き継がれる。

## ディレクトリ構造

```
~/.goemon/workflows/
└── ai-news-digest/
    ├── workflow.yaml    # ワークフロー定義（必須）
    ├── search.sh        # スクリプトステップ
    ├── fetch.sh
    ├── generate.sh
    └── publish.sh
```

## workflow.yaml

YAML 形式でワークフローを定義する。

```yaml
name: AI News Digest
schedule: "0 8 * * *"    # cron式（分 時 日 月 曜日）
notify: telegram          # 完了通知先（省略可）

steps:
  - name: search
    type: script
    entry_point: search.sh

  - name: generate
    type: prompt
    prompt: |
      以下のデータを元に記事を生成してください。
      ...

  - name: publish
    type: script
    entry_point: publish.sh
```

### フィールド

| フィールド | 必須 | 説明 |
|-----------|------|------|
| `name` | Yes | ワークフロー名 |
| `schedule` | Yes | cron 式（5フィールド: 分 時 日 月 曜日） |
| `notify` | No | 完了時の通知先 adapter 名（`telegram` 等） |
| `steps` | Yes | ステップの配列（1つ以上） |

### ステップ定義

各ステップは以下のフィールドを持つ。

| フィールド | 必須 | 説明 |
|-----------|------|------|
| `name` | Yes | ステップ名（ログやファイル名に使用） |
| `type` | Yes | `prompt` または `script` |
| `prompt` | type=prompt のとき | LLM に渡すプロンプト |
| `entry_point` | type=script のとき | 実行するスクリプトファイル名 |

## ステップの種類

### prompt ステップ

LLM（ReAct ループ）でステップを実行する。ツール呼び出しが可能。

- 前ステップの stdout が `Previous step result:` として prompt に付加される
- 会話履歴には保存されない（`RunWithoutHistory`）

### script ステップ

シェルスクリプトまたは Python スクリプトを直接実行する。

- 拡張子に基づいて実行方法を自動判定（`.sh` → bash、`.py` → python3）
- タイムアウト: 5分
- 前ステップの stdout が stdin に渡される
- stdout が次ステップへの入力となる

## ワークスペース

各ワークフロー実行ごとに一時ディレクトリ（ワークスペース）が自動作成される。

### 環境変数

script ステップには以下の環境変数が設定される。

| 変数 | 説明 |
|------|------|
| `GOEMON_WORKSPACE` | ワークスペースディレクトリの絶対パス |
| `GOEMON_PREV_RESULT` | 前ステップの出力ファイルパス |

### ステップ間のデータ受け渡し

ステップ間でデータを受け渡す方法は2つある。

1. **stdin/stdout**（簡易）— 前ステップの stdout が次ステップの stdin に渡される
2. **ワークスペースファイル**（推奨）— `$GOEMON_WORKSPACE` にファイルを保存し、後続ステップで読む

ワークスペースファイルを使う方が、データ量が多い場合やバイナリデータの受け渡しに適している。

```bash
# search.sh — ワークスペースにファイル保存
echo "${RESULTS}" > "${GOEMON_WORKSPACE}/search_results.json"

# fetch.sh — ワークスペースからファイル読み込み
RESULTS="${GOEMON_WORKSPACE}/search_results.json"
cat "${RESULTS}" | python3 process.py
```

### ライフサイクル

- ワークフロー実行開始時に作成
- 各ステップの出力は `step_N_<name>.txt` として自動保存
- ワークフロー実行完了後に自動削除

## 実行方法

### 手動実行

```bash
goemon workflow run <name>
```

### スケジュール実行

`goemon serve` を起動すると、スケジューラが毎分ワークフローをスキャンし、cron 式にマッチしたものを自動実行する。

- 同じワークフローの重複実行は防止される
- ワークフローの追加・変更は `~/.goemon/workflows/` にファイルを置くだけで反映（再起動不要）

### 一覧表示

```bash
goemon workflow list
```

## 実行ログ

各ステップの実行結果は SQLite の `workflow_runs` テーブルに記録される。

| カラム | 説明 |
|--------|------|
| `workflow_name` | ワークフロー名 |
| `step_name` | ステップ名 |
| `step_type` | `prompt` or `script` |
| `input` | ステップへの入力 |
| `output` | ステップの出力 |
| `success` | 成功/失敗 |
| `error_message` | エラーメッセージ（失敗時） |
| `duration_ms` | 実行時間（ミリ秒） |
| `created_at` | 実行日時 |

## 通知

`notify` フィールドに adapter 名を指定すると、ワークフロー完了時に結果が通知される。現在対応している adapter:

- `telegram` — 設定済みの allowed_users 全員に送信

## 設計指針

- **確実に実行したい処理は script ステップにする** — git 操作、ファイル書き出し、API 呼び出し等
- **LLM の判断が必要な処理は prompt ステップにする** — テキスト生成、分析、判断等
- **ステップ間のデータはワークスペースファイルで渡す** — stdin/stdout だけに頼らない
- **各ステップは独立して理解可能にする** — 1ステップ = 1責務
