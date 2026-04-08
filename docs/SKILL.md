# Skill System

## Overview

スキルは再利用可能な自動化モジュール。bash や python のスクリプトとして実装され、GoEmon のツールとして LLM から直接呼び出せる。

## ディレクトリ構造

```
~/.goemon/skills/
├── web-search/
│   ├── SKILL.md      # メタデータ定義（必須）
│   └── main.py       # エントリポイント
├── claude-code/
│   ├── SKILL.md
│   └── main.sh
└── hello-world/
    ├── SKILL.md
    └── main.py
```

## SKILL.md フォーマット

```markdown
# スキル名

## Description
スキルの説明（1行）。LLM のツール説明文として使用される。

## Trigger
- manual: "トリガーフレーズ"

## Entry Point
main.sh

## Language
bash

## Input
- field_name: フィールドの説明
- optional_field: (optional) オプションフィールドの説明

## Output
- result_field: 出力フィールドの説明

## Dependencies
- 必要な外部コマンド
```

### セクション詳細

| セクション | 必須 | 説明 |
|-----------|------|------|
| Description | Yes | LLM に渡されるツール説明文 |
| Trigger | No | 手動/自動トリガーの定義 |
| Entry Point | Yes | 実行するスクリプトファイル名 |
| Language | Yes | 実行方法の判定に使用（`bash`, `python` 等） |
| Input | No | 入力パラメータ定義。LLM のツールパラメータに変換される |
| Output | No | 出力フォーマットの説明（ドキュメント用） |
| Dependencies | No | 必要な外部コマンドの一覧（ドキュメント用） |

## Input パラメータ

`## Input` セクションの各行が LLM ツールの JSON Schema パラメータに変換される。

### 記法

```markdown
## Input
- query: Search query string
- max_results: (optional) Maximum number of results. Default 5.
```

### パース規則

- `- field_name: description` → 必須パラメータ
- `- field_name: (optional) description` → オプションパラメータ
- フィールド名はコロンの前、説明はコロンの後
- `(optional)` で始まる説明文はオプション扱い

### 生成される JSON Schema

上記の例から以下の JSON Schema が生成され、LLM に渡される:

```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "Search query string"
    },
    "max_results": {
      "type": "string",
      "description": "Maximum number of results. Default 5."
    }
  },
  "required": ["query"]
}
```

## 実行モデル

### 入出力

- **入力**: LLM からのパラメータが JSON として stdin に渡される
- **出力**: stdout に結果を出力（JSON 推奨）
- **エラー**: stderr はログに記録される
- **タイムアウト**: 60秒

### 実行方法

Language フィールドに基づいて実行方法が決まる:

| Language | 実行コマンド |
|----------|-------------|
| `bash`, `sh` | `bash <entry_point>` |
| `python`, `python3` | `python3 <entry_point>` |
| その他 | `<entry_point>`（直接実行） |

## LLM ツールとしての登録

スキルは GoEmon 起動時に自動的に LLM ツールとして登録される。

- ツール名: `skill_<スキル名>`（例: `skill_web-search`）
- 説明文: SKILL.md の Description セクション
- パラメータ: SKILL.md の Input セクションから自動生成

### 動的発見

スキルは `ToolProvider` インターフェースを通じて動的に発見される。GoEmon の実行中にスキルを追加・削除しても、次の LLM 呼び出しから反映される（再起動不要）。

## 標準スキル

`goemon init` で `~/.goemon/skills/` に展開される。ソースは `templates/skills/` にあり、バイナリに埋め込まれている。

| スキル | 説明 |
|--------|------|
| `web-search` | DuckDuckGo で Web 検索。API キー不要 |
| `claude-code` | 複雑なコーディングタスクを Claude Code CLI に委譲 |
| `github-pr` | GitHub リポジトリに PR を作成 |
| `hello-world` | 最小限のサンプルスキル |

## CLI コマンド

```bash
goemon skill list                    # スキル一覧
goemon skill run <name> [input-json] # スキル実行
goemon skill install <github-url>    # GitHub からインストール
goemon skill remove <name>           # 削除
```

## 実行ログ

スキルの実行結果は SQLite の `skill_runs` テーブルに記録される。

| カラム | 説明 |
|--------|------|
| `skill_name` | スキル名 |
| `input` | 入力 JSON |
| `output` | 出力 |
| `success` | 成功/失敗 |
| `error_message` | エラーメッセージ |
| `duration_ms` | 実行時間（ミリ秒） |

## スキルの作成

### 手動作成

1. `~/.goemon/skills/<name>/` ディレクトリを作成
2. `SKILL.md` を作成（上記フォーマットに従う）
3. エントリポイントスクリプトを作成
4. 再起動不要で即座に利用可能

### GitHub からインストール

```bash
goemon skill install https://github.com/user/repo
```

リポジトリが `~/.goemon/skills/<repo-name>/` にクローンされる。`SKILL.md` が含まれている必要がある。
