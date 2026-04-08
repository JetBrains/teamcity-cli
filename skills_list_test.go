package teamcitycli

import (
	"io/fs"
	"testing"
)

func TestSkillSubFSScopedToSingleSkill(t *testing.T) {
	t.Parallel()

	sub, ok := SkillSubFS(DefaultSkill)
	if !ok {
		t.Fatal("SkillSubFS returned false for default skill")
	}

	var skills []string
	_ = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return err
		}
		skills = append(skills, p)
		return fs.SkipDir
	})

	if len(skills) != 1 {
		t.Errorf("expected exactly 1 SKILL.md in sub-FS, got %d: %v", len(skills), skills)
	}
}

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
