package teamcitycli

import (
	"fmt"
	"io/fs"
	"slices"

	migratesk "github.com/JetBrains/teamcity-skills/skills/migrate-to-teamcity"
	teamcityclisk "github.com/JetBrains/teamcity-skills/skills/teamcity-cli"
	onboardsk "github.com/JetBrains/teamcity-skills/skills/teamcity-onboard"
	"github.com/tiulpin/instill"
)

// skillFilesystems are the trees the CLI ships skills from, queried in order.
// Every skill now comes from JetBrains/teamcity-skills, so the CLI embeds none
// of its own (TW-101969, TW-103762).
//
// They stay separate rather than merging into one fs.FS: each package is rooted
// at its own skill, so both put a SKILL.md at the filesystem root. instill
// returns fs.SkipDir once it finds one, which would end the walk after the
// first skill — and merging by path would collide the two SKILL.md files
// anyway.
var skillFilesystems = []fs.FS{teamcityclisk.FS, migratesk.FS, onboardsk.FS}

// ListSkills returns metadata for every skill bundled with this build.
func ListSkills() []instill.SkillMeta {
	var out []instill.SkillMeta
	for _, fsys := range skillFilesystems {
		out = append(out, instill.ListSkills(fsys)...)
	}
	return out
}

// SkillCommand is the CLI name installed stubs call back: `teamcity skill guide <skill>`.
const SkillCommand = "teamcity"

// InstallSkills installs stubs for the bundled skills selected by opts, returning the
// results from every tree.
func InstallSkills(opts instill.Options) ([]instill.Result, error) {
	opts.Command = SkillCommand
	var out []instill.Result
	for _, fsys := range skillFilesystems {
		results, err := instill.Install(fsys, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, results...)
	}
	return out, nil
}

// SkillGuide returns a page of a bundled skill's guide; topic "" is the overview.
func SkillGuide(skill, topic string) (string, error) {
	fsys, err := skillFS(skill)
	if err != nil {
		return "", err
	}
	return instill.Guide(fsys, SkillCommand, skill, topic)
}

// SkillTopics lists the guide topics of a bundled skill.
func SkillTopics(skill string) []string {
	fsys, err := skillFS(skill)
	if err != nil {
		return nil
	}
	return instill.Topics(fsys, skill)
}

func skillFS(skill string) (fs.FS, error) {
	for _, fsys := range skillFilesystems {
		if slices.ContainsFunc(instill.ListSkills(fsys), func(s instill.SkillMeta) bool { return s.Name == skill }) {
			return fsys, nil
		}
	}
	return nil, fmt.Errorf("unknown skill %q; run 'teamcity skill list' to see available skills", skill)
}
