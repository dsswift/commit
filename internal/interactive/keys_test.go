package interactive

import "testing"

func TestSelectStepHelp(t *testing.T) {
	km := DefaultKeyMap()
	bindings := km.SelectStepHelp()
	if len(bindings) != 5 {
		t.Errorf("SelectStepHelp() returned %d bindings, want 5", len(bindings))
	}
}

func TestEditStepHelp(t *testing.T) {
	km := DefaultKeyMap()
	bindings := km.EditStepHelp()
	if len(bindings) != 8 {
		t.Errorf("EditStepHelp() returned %d bindings, want 8", len(bindings))
	}
}

func TestConfirmStepHelp(t *testing.T) {
	km := DefaultKeyMap()
	bindings := km.ConfirmStepHelp()
	if len(bindings) != 5 {
		t.Errorf("ConfirmStepHelp() returned %d bindings, want 5", len(bindings))
	}
}
