package git

import (
	"testing"
)

func FuzzParseDiffStat(f *testing.F) {
	// Seed with representative inputs
	f.Add(" src/handler.go | 15 ++++++++-------")
	f.Add(" docs/readme.md | 50 ++++++++++++++++++++++++++++++++++++++++++++++++++")
	f.Add(" config.yaml    |  3 ++-")
	f.Add(" binary.dat     | Bin 0 -> 1234 bytes")
	f.Add("")
	f.Add("not a valid diff stat line")

	f.Fuzz(func(t *testing.T, input string) {
		// Should never panic
		parseDiffStat(input)
	})
}

func FuzzParseNumstat(f *testing.F) {
	// Seed with representative inputs
	f.Add("10\t5\tsrc/handler.go")
	f.Add("-\t-\tbinary.dat")
	f.Add("0\t0\tempty.go")
	f.Add("")
	f.Add("not valid numstat")

	f.Fuzz(func(t *testing.T, input string) {
		// Should never panic
		parseNumstat(input)
	})
}
