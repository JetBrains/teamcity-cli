package teamcitycli

import "testing"

func TestListSkillsContainsDefault(t *testing.T) {
	t.Parallel()

	skills := ListSkills()
	for _, s := range skills {
		if s.Name == DefaultSkill {
			return
		}
	}
	t.Errorf("ListSkills did not include default skill %q", DefaultSkill)
}

func TestHasSkill(t *testing.T) {
	t.Parallel()

	if !HasSkill(DefaultSkill) {
		t.Errorf("HasSkill(%q) = false, want true", DefaultSkill)
	}
	if HasSkill("nonexistent") {
		t.Error("HasSkill(\"nonexistent\") = true, want false")
	}
}
