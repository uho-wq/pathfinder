// pathfinder is a TUI shell for AI-guided code review. An AI agent
// (e.g. Claude Code) explores a PR and writes a review plan file; a
// human then opens that plan here and reviews files in the suggested
// order, with per-file diffs and AI-authored guidance side by side.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/uho-wq/pathfinder/internal/gitdiff"
	"github.com/uho-wq/pathfinder/internal/plan"
	"github.com/uho-wq/pathfinder/internal/ui"
)

//go:embed prompts/generate-plan.md
var planPrompt string

//go:embed examples/review.json
var examplePlan string

const usage = `pathfinder - AIが用意したレビュープランでPRをレビューするTUI

Usage:
  pathfinder [flags] [plan.json]   プランを開く (省略時: .pathfinder/<ブランチ名>.json,
                                   .pathfinder/review.json, review.json の順に自動検出)
  pathfinder prompt                プラン生成用プロンプトを出力 (Claude Code等に渡す)
  pathfinder example               プランファイルのサンプルを出力

Flags:
  -C dir    git diff を実行するリポジトリのディレクトリ (default: カレント)

レビューの流れ:
  1. PRのブランチ上で "pathfinder prompt | claude -p" などでプランを生成
  2. "pathfinder .pathfinder/review.json" でTUIを開く
  3. 左のツリーの順に箇所を追い、右のガイドの観点でレビューする
`

func main() {
	fs := flag.NewFlagSet("pathfinder", flag.ExitOnError)
	repoDir := fs.String("C", "", "repository directory for git diff")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "prompt":
			fmt.Print(planPrompt)
			return
		case "example":
			fmt.Print(examplePlan)
			return
		case "help", "-h", "--help":
			fmt.Print(usage)
			return
		}
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	planPath := findPlan(fs.Args(), *repoDir)
	if planPath == "" {
		fmt.Fprintln(os.Stderr, "エラー: プランファイルが見つかりません (.pathfinder/<ブランチ名>.json / .pathfinder/review.json / review.json)")
		fmt.Fprintln(os.Stderr, "まず `pathfinder prompt` の内容をAIに渡してプランを生成してください。")
		os.Exit(1)
	}

	p, err := plan.Load(planPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}
	st := plan.LoadState(planPath)

	if err := ui.Run(p, st, *repoDir); err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}
}

// findPlan resolves the plan file to open: an explicit argument wins,
// then the current branch's plan (.pathfinder/<branch>.json, slashes
// replaced with "-"), then the branch-agnostic legacy locations.
func findPlan(args []string, repoDir string) string {
	if len(args) > 0 {
		return args[0]
	}
	var candidates []string
	if branch := gitdiff.CurrentBranch(repoDir); branch != "" {
		candidates = append(candidates, filepath.Join(".pathfinder", plan.BranchFileName(branch)))
	}
	candidates = append(candidates, ".pathfinder/review.json", "review.json")
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
