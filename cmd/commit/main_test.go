package main

import (
	"testing"
)

func TestReverseFlag_Set(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "bare true", input: "true", want: 1},
		{name: "bare false", input: "false", want: 0},
		{name: "explicit 5", input: "5", want: 5},
		{name: "explicit 1", input: "1", want: 1},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "non-numeric", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f reverseFlag
			err := f.Set(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Set(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Set(%q) unexpected error: %v", tt.input, err)
				return
			}

			if int(f) != tt.want {
				t.Errorf("Set(%q) = %d, want %d", tt.input, int(f), tt.want)
			}
		})
	}
}

func TestReverseFlag_IsBoolFlag(t *testing.T) {
	var f reverseFlag
	if !f.IsBoolFlag() {
		t.Error("IsBoolFlag() should return true")
	}
}
