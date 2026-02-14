package llm

import (
	"testing"

	"github.com/dsswift/commit/pkg/types"
)

func BenchmarkCleanContent(b *testing.B) {
	content := "```json\n{\"commits\": [{\"type\": \"feat\", \"scope\": \"api\", \"message\": \"add endpoint\", \"files\": [\"handler.go\"], \"reasoning\": \"new feature\"}]}\n```"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleanContent(content)
	}
}

func BenchmarkBuildPrompt(b *testing.B) {
	req := &types.AnalysisRequest{
		Files: []types.FileChange{
			{Path: "src/api/handler.go", Status: "modified", Scope: "api", DiffSummary: "+10 -5"},
			{Path: "src/api/router.go", Status: "modified", Scope: "api", DiffSummary: "+4 -4"},
			{Path: "docs/readme.md", Status: "added", Scope: "", DiffSummary: "+50"},
			{Path: "config/app.yaml", Status: "modified", Scope: "config", DiffSummary: "+2 -1"},
			{Path: "tests/handler_test.go", Status: "modified", Scope: "api", DiffSummary: "+16 -6"},
		},
		Diff:          "diff --git a/handler.go b/handler.go\n+added\n-removed\n",
		RecentCommits: []string{"feat: add auth", "fix: handle error", "docs: update readme"},
		HasScopes:     true,
		Rules: types.CommitRules{
			Types:            []string{"feat", "fix", "docs", "chore", "refactor"},
			MaxMessageLength: 50,
			BehavioralTest:   "feat = behavior change",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildPrompt(req)
	}
}
