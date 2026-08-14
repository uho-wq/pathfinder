# pathfinder

AIが下調べしたレビュープランに沿ってPRをレビューするためのTUIツール。

PRレビューでよくある「どのファイルから見ればいいかわからない」「この関数はどこから呼ばれていて、何に依存しているのかわからない」という課題を、**AIが先に探索して意味的に整理した状態**でレビューを始められるようにする。

```
┌──────────────┬──────────────────────────────┬────────────────────┐
│ ファイル/箇所 │ 差分                          │ レビューガイド       │
│ (レビュー順)  │ (選択した箇所へジャンプ・       │ (変更の意図・観点・   │
│              │  ハイライト)                  │  依存関係)          │
└──────────────┴──────────────────────────────┴────────────────────┘
```

レビューの単位はファイルではなく**ファイル内の「箇所」(セクション)**。AIが各ファイルの差分を意味的なまとまり(関数の追加、条件分岐の変更など)に分割して順序付け、レビュアーはその一覧を上から順に追っていく。箇所を選ぶと差分ペインがその行範囲へスクロールし、左端にハイライトバーが付く。

pathfinder自体はTUIの「枠」だけを提供する。中身のデータ(レビュー順、箇所の分割と解説、レビュー観点、依存関係)は、Claude CodeなどのAIエージェントが生成するプランファイル(JSON)として注入する。

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
- 各ファイルの差分を分割した**レビューすべき箇所の一覧**(行範囲つき、読む順)
- 各箇所で**何が起こっているか**(変更の意図)と**レビュー観点**(その差分固有の確認事項)
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
| `j` / `k` | 箇所/ファイル選択(ツリー) / スクロール(差分・ガイド) |
| `d` / `u` | 半ページスクロール |
| `g` / `G` | 先頭 / 末尾へ |
| `space` | レビュー済みにして次の箇所へ |
| `r` | レビュー済みトグル |
| `n` / `p` | 次 / 前のファイル(残りの箇所をスキップ) |
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
          "sections": [                       // ファイル内のレビュー箇所 (順序 = レビュー順)
            {
              "title": "箇所の名前",
              "start_line": 10,               // 変更後ファイルの行番号 (deleted は変更前)
              "end_line": 42,
              "summary": "この箇所で起こっている変化",
              "review_points": ["この箇所の確認事項"],
              "notes": "補足 (任意)"
            }
          ],
          "review_points": ["確認すべき観点"],  // sections がないファイル向け
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

`sections` を持つファイルはセクションが、持たないファイルはファイル自体がレビュー単位になる。進捗(n/N レビュー済)もこの単位で数える。

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
