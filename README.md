# pathfinder

AIが下調べしたレビュープランに沿ってPRをレビューするためのTUIツール。

PRレビューでよくある「どのファイルから見ればいいかわからない」「この関数はどこから呼ばれていて、何に依存しているのかわからない」という課題を、**AIが先に探索して意味的に整理した状態**でレビューを始められるようにする。

```
┌──────────────┬──────────────────────────────┬────────────────────┐
│ ファイル      │ 差分                          │ レビューガイド       │
│ (レビュー順)  │ (git diff / 埋め込みdiff)      │ (変更の意図・観点・   │
│              │                              │  依存関係)          │
└──────────────┴──────────────────────────────┴────────────────────┘
```

pathfinder自体はTUIの「枠」だけを提供する。中身のデータ(レビュー順、各ファイルの解説、レビュー観点、依存関係)は、Claude CodeなどのAIエージェントが生成するプランファイル(JSON)として注入する。

## インストール

```sh
go install github.com/uho-wq/pathfinder@latest
```

## 使い方

### 1. AIにレビュープランを生成させる

レビューしたいPRのブランチをチェックアウトした状態で、プラン生成プロンプトをAIに渡す:

```sh
pathfinder prompt | claude -p --allowedTools "Bash(git:*) Read Grep Glob Write"
```

AIがPRの差分を探索し、以下を調べて `.pathfinder/review.json` を書き出す:

- 依存関係に基づく**意味的なレビュー順**(モデル → ロジック → API境界 → テスト)
- 各ファイルの差分で**何が起こっているか**(変更の意図)
- ファイルごとの**レビュー観点**(その差分固有の確認事項)
- 変更箇所の**依存先**と**呼び出し元**(コード検索で確認した事実)

### 2. TUIでレビューする

```sh
pathfinder                     # .pathfinder/review.json / review.json を自動検出
pathfinder path/to/review.json # 明示指定
pathfinder -C /path/to/repo    # git diff の実行ディレクトリを指定
```

### キー操作

| キー | 動作 |
|------|------|
| `tab` / `h` / `l` | ペイン切替(ツリー → 差分 → ガイド) |
| `j` / `k` | ファイル選択(ツリー) / スクロール(差分・ガイド) |
| `d` / `u` | 半ページスクロール |
| `g` / `G` | 先頭 / 末尾へ |
| `space` | レビュー済みにして次のファイルへ |
| `r` | レビュー済みトグル |
| `n` / `p` | 次 / 前のファイル |
| `q` | 終了 |

レビュー済みマークは `review.state.json` に自動保存され、中断・再開できる。

## プランファイル

`pathfinder example` でサンプルを出力できる。スキーマの要点:

```jsonc
{
  "version": 1,
  "title": "PRタイトル",
  "summary": "PR全体の概要",
  "base": "main",                  // diff のベース (マージ先)
  "head": "feature/x",             // PRブランチ。空なら作業ツリーと比較
  "steps": [                       // 意味的なまとまりごとのレビューステップ (順序 = レビュー順)
    {
      "name": "データモデル",
      "description": "なぜこの順で見るのか",
      "files": [
        {
          "path": "internal/model/invitation.go",
          "status": "added",                  // added | modified | deleted | renamed
          "summary": "この差分で起こっている変化",
          "review_points": ["確認すべき観点"],
          "dependencies": ["この変更の依存先"],
          "dependents": ["呼び出し元・影響範囲"],
          "notes": "補足 (任意)",
          "diff": ""                          // 省略時は git diff で実行時に取得
        }
      ]
    }
  ]
}
```

diffは通常プランに埋め込まず、pathfinderが実行時に `git diff base...head -- <path>` で取得する(three-dot比較なのでPRと同じ差分になる)。gitリポジトリ外でレビューする場合は各ファイルの `diff` フィールドにunified diffを埋め込める。

## 開発

```sh
go build ./...
go test ./...
```

構成:

- `internal/plan` — プランファイルのスキーマ・読み込み・レビュー状態の永続化
- `internal/gitdiff` — gitからのファイル単位diff取得
- `internal/ui` — Bubble Teaベースの3ペインTUI
- `prompts/generate-plan.md` — AI用プラン生成プロンプト(`pathfinder prompt` で出力)
- `examples/review.json` — プランのサンプル(`pathfinder example` で出力)
