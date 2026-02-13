package llm

import (
	"testing"
)

func FuzzCleanContent(f *testing.F) {
	// Seed with representative inputs
	f.Add("```json\n{\"key\": \"value\"}\n```")
	f.Add("{\"commits\": []}")
	f.Add("```\nplain content\n```")
	f.Add("")
	f.Add("no fences here")
	f.Add("```json\n```")

	f.Fuzz(func(t *testing.T, input string) {
		// Should never panic
		cleanContent(input)
	})
}

func FuzzParseCommitPlan(f *testing.F) {
	// Seed with representative inputs
	f.Add("{\"commits\": []}")
	f.Add("{\"commits\": [{\"type\": \"feat\", \"message\": \"add feature\", \"files\": [\"file.go\"]}]}")
	f.Add("```json\n{\"commits\": []}\n```")
	f.Add("")
	f.Add("not json at all")
	f.Add("{}")
	f.Add("{\"commits\": null}")

	f.Fuzz(func(t *testing.T, input string) {
		// Should never panic (errors are expected for invalid input)
		parseCommitPlan(input) //nolint:errcheck
	})
}
