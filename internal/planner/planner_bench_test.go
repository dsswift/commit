package planner

import (
	"testing"

	"github.com/dsswift/commit/pkg/types"
)

func BenchmarkValidateAndFix(b *testing.B) {
	validator := NewValidator("/tmp", &types.RepoConfig{}, []string{
		"src/api/handler.go",
		"src/api/router.go",
		"docs/readme.md",
		"config/app.yaml",
	})

	plan := &types.CommitPlan{
		Commits: []types.PlannedCommit{
			{
				Type:    "feat",
				Message: "add new endpoint",
				Files:   []string{"src/api/handler.go", "src/api/router.go"},
			},
			{
				Type:    "docs",
				Message: "update readme",
				Files:   []string{"docs/readme.md"},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateAndFix(plan)
	}
}

func BenchmarkFilterSensitiveFiles(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plan := &types.CommitPlan{
			Commits: []types.PlannedCommit{
				{
					Type:    "feat",
					Message: "add feature",
					Files: []string{
						"src/handler.go",
						".env",
						"config/app.yaml",
						"credentials.json",
						"src/router.go",
						"secret.pem",
					},
				},
			},
		}
		FilterSensitiveFiles(plan)
	}
}
