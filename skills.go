package teamcitycli

import (
	"io/fs"

	migratesk "github.com/JetBrains/teamcity-skills/skills/migrate-to-teamcity"
	teamcityclisk "github.com/JetBrains/teamcity-skills/skills/teamcity-cli"
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
var skillFilesystems = []fs.FS{teamcityclisk.FS, migratesk.FS}

// ListSkills returns metadata for every skill bundled with this build.
func ListSkills() []instill.SkillMeta {
	var out []instill.SkillMeta
	for _, fsys := range skillFilesystems {
		out = append(out, instill.ListSkills(fsys)...)
	}
	return out
}

// InstallSkills installs the bundled skills selected by opts, returning the
// results from every tree.
func InstallSkills(opts instill.Options) ([]instill.Result, error) {
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
