// Package testutil provides shared test helpers.
package testutil

// ContainsString reports whether substr is within s.
func ContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
