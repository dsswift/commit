package git

import (
	"strings"
	"testing"
)

func BenchmarkParseDiffStat(b *testing.B) {
	input := strings.Join([]string{
		" src/api/handler.go | 15 ++++++++-------",
		" src/api/router.go  |  8 ++++----",
		" docs/readme.md     | 50 ++++++++++++++++++++++++++++++++++++++++++++++++++",
		" config/app.yaml    |  3 ++-",
		" tests/handler_test.go | 22 ++++++++++++++++------",
	}, "\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseDiffStat(input)
	}
}

func BenchmarkParseNumstat(b *testing.B) {
	input := strings.Join([]string{
		"10\t5\tsrc/api/handler.go",
		"4\t4\tsrc/api/router.go",
		"50\t0\tdocs/readme.md",
		"2\t1\tconfig/app.yaml",
		"16\t6\ttests/handler_test.go",
	}, "\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseNumstat(input)
	}
}

func BenchmarkTruncateDiff(b *testing.B) {
	// Create a realistic diff of ~10KB
	diff := strings.Repeat("diff --git a/file.go b/file.go\n+added line\n-removed line\n context line\n", 200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TruncateDiff(diff, 5000)
	}
}
