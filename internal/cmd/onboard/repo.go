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
	sourceSample  repoSource = "sample"
)

// repository is the repository picked for onboarding; dir is empty for samples.
type repository struct {
	source    repoSource
	dir       string
	remoteURL string
	revision  string // optional commit the clone and runs are pinned to; empty means the branch head
}

// sampleRepo is a public sample repository offered when the user has none at hand.
type sampleRepo struct {
	name     string
	stack    string // shown next to the name in the picker
	url      string
	revision string // optional commit SHA to pin to, so onboarding sees the same files every time
}

// sampleRepos lists the sample repositories offered by the picker.
var sampleRepos = []sampleRepo{
	{name: "Conductor", stack: "Java, Spring Boot, Gradle", url: "https://github.com/conductor-oss/conductor", revision: "ab2de6da6c7b5935e6c6d09da29953b8d4cfe291"},
	{name: "FluentValidation", stack: "C#, .NET", url: "https://github.com/FluentValidation/FluentValidation", revision: "299ef377bde844af7d28b7aa21786a6853f930e3"},
}

// label renders a sample as "Name (stack)" for the picker.
func (d sampleRepo) label() string {
	if d.stack == "" {
		return d.name
	}
	return d.name + " (" + d.stack + ")"
}

// chooseRepository asks whether to use the current directory, another path, or a sample repository.
func chooseRepository(f *cmdutil.Factory) (repository, error) {
	_, inRepo := git.RepoRoot(".")
	source, err := promptSource(f, inRepo)
	if err != nil {
		return repository{}, err
	}
	switch source {
	case sourcePath:
		return promptPath(f)
	case sourceSample:
		return promptSample(f)
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
		huh.NewOption("Sample repository", sourceSample),
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

// promptSample lets the user pick one of sampleRepos.
func promptSample(f *cmdutil.Factory) (repository, error) {
	options := make([]huh.Option[sampleRepo], 0, len(sampleRepos))
	for _, d := range sampleRepos {
		options = append(options, huh.NewOption(d.label(), d))
	}
	var picked sampleRepo
	if err := cmdutil.Select(f.Printer, "Select a sample repository", options, &picked); err != nil {
		return repository{}, err
	}
	return repository{source: sourceSample, remoteURL: picked.url, revision: picked.revision}, nil
}
