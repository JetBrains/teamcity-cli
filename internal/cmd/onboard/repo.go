package onboard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/git"
	"github.com/charmbracelet/huh"
)

// repoSource is where the onboarded repository comes from.
type repoSource string

const (
	sourceCurrent repoSource = "current"
	sourcePath    repoSource = "path"
	sourceDemo    repoSource = "demo"
)

// repository is the repository picked for onboarding; dir is empty for demos.
type repository struct {
	source    repoSource
	dir       string
	remoteURL string
}

// demoRepo is a public sample repository offered when the user has none at hand.
type demoRepo struct {
	name  string
	stack string // shown next to the name in the picker
	url   string
}

// demoRepos lists the sample repositories offered by the picker.
var demoRepos = []demoRepo{
	// TODO: update final demo list
	{name: "Spring PetClinic", stack: "Java, Spring Boot, Maven", url: "https://github.com/spring-projects/spring-petclinic"},
}

// label renders a demo as "Name (stack)" for the picker.
func (d demoRepo) label() string {
	if d.stack == "" {
		return d.name
	}
	return d.name + " (" + d.stack + ")"
}

// chooseRepository asks whether to use the current directory, another path, or a demo repository.
func chooseRepository(f *cmdutil.Factory) (repository, error) {
	_, inRepo := git.RepoRoot(".")
	source, err := promptSource(f, inRepo)
	if err != nil {
		return repository{}, err
	}
	switch source {
	case sourcePath:
		return promptPath(f)
	case sourceDemo:
		return promptDemo(f)
	default:
		return resolveRepository(sourceCurrent, ".")
	}
}

// resolveRepository finds the git root containing dir and its preferred remote URL (origin first).
func resolveRepository(source repoSource, dir string) (repository, error) {
	root, ok := git.RepoRoot(dir)
	if !ok {
		return repository{}, api.Validation(
			dir+" is not inside a git repository",
			"Pick a directory that contains a .git folder",
		)
	}
	repo := repository{source: source, dir: root}
	if remotes := git.RemoteURLsIn(root); len(remotes) > 0 {
		repo.remoteURL = remotes[0]
	}
	// TODO: decide behavior without remotes
	return repo, nil
}

// promptSource asks where the repository comes from; "current" is offered only inside a git repository.
func promptSource(f *cmdutil.Factory, inRepo bool) (repoSource, error) {
	// TODO: copy pending design review
	var options []huh.Option[repoSource]
	if inRepo {
		options = append(options, huh.NewOption("Current directory", sourceCurrent))
	}
	options = append(options,
		huh.NewOption("Another directory", sourcePath),
		huh.NewOption("Demo repository", sourceDemo),
	)
	var source repoSource
	if err := cmdutil.Select(f.Printer, "Choose a repository to onboard", options, &source); err != nil {
		return "", err
	}
	return source, nil
}

// promptPath asks for an absolute or relative path and resolves it as a git repository.
func promptPath(f *cmdutil.Factory) (repository, error) {
	// TODO: copy pending design review
	var input string
	if err := cmdutil.PromptValidatedString(f.Printer, "Provide a path to a git repository", "", validateRepoPath, &input); err != nil {
		return repository{}, err
	}
	path, err := normalizePath(input)
	if err != nil {
		return repository{}, err
	}
	return resolveRepository(sourcePath, path)
}

// validateRepoPath rejects input that is not an existing directory inside a git repository.
func validateRepoPath(input string) error {
	path, err := normalizePath(input)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s does not exist", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	if _, ok := git.RepoRoot(path); !ok {
		return fmt.Errorf("%s is not inside a git repository", path)
	}
	return nil
}

// normalizePath trims whitespace and quotes (drag-and-drop adds them), rejects control characters, and expands a leading ~.
func normalizePath(input string) (string, error) {
	path := strings.Trim(strings.TrimSpace(input), `"'`)
	if path == "" {
		return "", errors.New("path is required")
	}
	if strings.ContainsFunc(path, unicode.IsControl) {
		return "", errors.New("path contains invalid characters")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}
	return filepath.Abs(path)
}

// promptDemo lets the user pick one of demoRepos.
func promptDemo(f *cmdutil.Factory) (repository, error) {
	options := make([]huh.Option[string], 0, len(demoRepos))
	for _, d := range demoRepos {
		options = append(options, huh.NewOption(d.label(), d.url))
	}
	var url string
	if err := cmdutil.Select(f.Printer, "Select a demo repository", options, &url); err != nil {
		return repository{}, err
	}
	return repository{source: sourceDemo, remoteURL: url}, nil
}
