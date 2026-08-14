package teamcitycli

import (
	"embed"
	"io/fs"

	teamcityclisk "github.com/JetBrains/TeamCitySkills/skills/teamcity-cli"
	"github.com/tiulpin/instill"
)

//go:embed all:skills
var localSkillsFS embed.FS

// skillFilesystems are the trees the CLI ships skills from, queried in order.
var skillFilesystems = []fs.FS{teamcityclisk.FS, localSkillsFS}

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
