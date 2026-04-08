package teamcitycli

import (
	"io/fs"
	"path"
	"strings"
)

const DefaultSkill = "teamcity-cli"

type SkillInfo struct {
	Name        string
	Version     string
	Description string
}

func ListSkills() []SkillInfo {
	var skills []SkillInfo
	_ = fs.WalkDir(SkillsFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return err
		}
		data, err := fs.ReadFile(SkillsFS, p)
		if err != nil {
			return err
		}
		info := parseSkillInfo(data)
		if info.Name != "" {
			skills = append(skills, info)
		}
		return fs.SkipDir
	})
	return skills
}

func SkillSubFS(name string) (fs.FS, bool) {
	var skillPath string
	_ = fs.WalkDir(SkillsFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return err
		}
		data, err := fs.ReadFile(SkillsFS, p)
		if err != nil {
			return err
		}
		info := parseSkillInfo(data)
		if info.Name == name {
			skillPath = path.Dir(p)
			return fs.SkipAll
		}
		return fs.SkipDir
	})
	if skillPath == "" {
		return nil, false
	}
	parent := path.Dir(skillPath)
	if parent == "." {
		return SkillsFS, true
	}
	sub, err := fs.Sub(SkillsFS, parent)
	if err != nil {
		return nil, false
	}
	return sub, true
}

func parseSkillInfo(data []byte) SkillInfo {
	var info SkillInfo
	fm := parseFrontmatter(data)
	info.Name = fm["name"]
	info.Version = strings.Trim(fm["version"], `"'`)
	info.Description = fm["description"]
	return info
}

func parseFrontmatter(data []byte) map[string]string {
	s := string(data)
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "---") {
		return nil
	}
	parts := strings.SplitN(s, "---", 3)
	if len(parts) < 3 {
		return nil
	}
	fields := map[string]string{}
	for line := range strings.SplitSeq(parts[1], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if ok {
			fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return fields
}
