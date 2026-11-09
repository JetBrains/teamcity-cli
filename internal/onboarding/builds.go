package onboarding

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/atomicfile"
)

// Builds queued inside the onboarding are remembered until the CLI sees them finish, so later commands can report their start and finish once.
const (
	buildsFileName = "onboarding-builds.json"
	buildsMaxAge   = 7 * 24 * time.Hour
	buildsMax      = 100
)

type rememberedBuild struct {
	ID         int       `json:"id"`
	Repository string    `json:"repository"`
	AddedAt    time.Time `json:"added_at"`
	Started    bool      `json:"started"` // build.started recorded; build.finished forgets the build
}

// BuildEvents says which onboarding build events to record for a remembered build.
type BuildEvents struct {
	Repository string
	Started    bool
	Finished   bool
}

// RememberBuild remembers a build queued inside the onboarding of an rt repository; never with analytics opted out.
func RememberBuild(id int, rt api.OnboardingRepositoryType) {
	if !Enabled() {
		return
	}
	if path, err := buildsPath(); err == nil {
		_ = rememberBuildAt(path, time.Now(), id, string(rt))
	}
}

// ObserveBuild returns the events to record now that a command sees build id in state ("running" or "finished"), and marks them recorded.
// ok is false for a build that isn't remembered.
func ObserveBuild(id int, state string) (BuildEvents, bool) {
	if !Enabled() {
		return BuildEvents{}, false
	}
	path, err := buildsPath()
	if err != nil {
		return BuildEvents{}, false
	}
	return observeBuildAt(path, time.Now(), id, state)
}

func buildsPath() (string, error) {
	dir, err := analytics.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, buildsFileName), nil
}

func rememberBuildAt(path string, now time.Time, id int, repository string) error {
	builds := readBuilds(path)
	if slices.ContainsFunc(builds, func(b rememberedBuild) bool { return b.ID == id }) {
		return nil
	}
	return writeBuilds(path, now, append(builds, rememberedBuild{ID: id, Repository: repository, AddedAt: now}))
}

func observeBuildAt(path string, now time.Time, id int, state string) (BuildEvents, bool) {
	if state != "running" && state != "finished" {
		return BuildEvents{}, false // nothing to report, so don't read the file
	}
	builds := readBuilds(path)
	i := slices.IndexFunc(builds, func(b rememberedBuild) bool { return b.ID == id })
	if i < 0 {
		return BuildEvents{}, false
	}
	b := builds[i]
	ev := BuildEvents{Repository: b.Repository, Started: !b.Started}
	switch {
	case state == "finished":
		ev.Finished = true
		builds = slices.Delete(builds, i, i+1)
	case b.Started:
		return ev, true // running, and build.started already recorded
	default:
		builds[i].Started = true
	}
	_ = writeBuilds(path, now, builds)
	return ev, true
}

// readBuilds returns the remembered builds; a missing, unreadable or corrupted file counts as empty.
func readBuilds(path string) []rememberedBuild {
	data, err := os.ReadFile(path)
	if err != nil {
		return []rememberedBuild{}
	}
	builds := []rememberedBuild{}
	if err := json.Unmarshal(data, &builds); err != nil {
		return []rememberedBuild{}
	}
	return builds
}

// writeBuilds saves builds without the expired ones, keeping only the newest buildsMax.
func writeBuilds(path string, now time.Time, builds []rememberedBuild) error {
	builds = slices.DeleteFunc(builds, func(b rememberedBuild) bool { return now.Sub(b.AddedAt) > buildsMaxAge })
	slices.SortStableFunc(builds, func(a, b rememberedBuild) int { return cmp.Compare(a.AddedAt.UnixNano(), b.AddedAt.UnixNano()) })
	if len(builds) > buildsMax {
		builds = builds[len(builds)-buildsMax:]
	}
	data, err := json.Marshal(builds)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, data)
}
