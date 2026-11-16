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
	"github.com/JetBrains/teamcity-cli/internal/output"
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
	stack    string // shown as a tag next to the name in the picker
	tag      string // what the pipeline will be like, shown as a tag after the stack
	url      string
	revision string // optional commit SHA to pin to, so onboarding sees the same files every time
}

// sampleRepos lists the sample repositories offered by the picker.
var sampleRepos = []sampleRepo{
	{name: "Conductor", stack: "Java, Spring Boot, Gradle", url: "https://github.com/conductor-oss/conductor", revision: "ab2de6da6c7b5935e6c6d09da29953b8d4cfe291", tag: "4,549 tests, potentially long pipeline"},
	{name: "FluentValidation", stack: "C#, .NET", url: "https://github.com/FluentValidation/FluentValidation", revision: "299ef377bde844af7d28b7aa21786a6853f930e3", tag: "839 tests, small and fast pipeline"},
	{name: "tRPC", stack: "TypeScript, Node.js, pnpm", url: "https://github.com/trpc/trpc", revision: "5b993cfc71211a204fe2962f0433eb88b6be6543", tag: "2,421 tests, longer pipeline"},
}

// label renders a sample as "Name [stack] [tag]" for the picker.
func (d sampleRepo) label() string {
	label := d.name
	if d.stack != "" {
		label += " " + output.Faint("["+d.stack+"]")
	}
	if d.tag != "" {
		label += " " + output.Cyan("["+d.tag+"]")
	}
	return label
}

// chooseRepository asks whether to use the current directory, another path, or a sample repository.
func chooseRepository(f *cmdutil.Factory) (repository, error) {
	source, err := promptSource(f, currentRepoRoot())
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

// currentRepoRoot returns the absolute current directory when it is the root of a git repository, "" otherwise.
// It never walks up: a plain folder inside another repository (for example a home folder under git) is not offered.
func currentRepoRoot() string {
	dir, err := filepath.Abs(".")
	if err != nil || !git.IsRepoRoot(dir) {
		return ""
	}
	return dir
}

// resolveRepository takes dir as the checkout, which must be the root of a git repository, and finds its preferred remote URL (origin first).
func resolveRepository(source repoSource, dir string) (repository, error) {
	root, err := filepath.Abs(dir)
	if err != nil || !git.IsRepoRoot(root) {
		return repository{}, api.Validation(
			dir+" is not the root of a git repository",
			"Pick the directory that contains the .git folder",
		)
	}
	repo := repository{source: source, dir: root}
	if remotes := git.RemoteURLsIn(root); len(remotes) > 0 {
		repo.remoteURL = remotes[0]
	}
	return repo, nil
}

// promptSource asks where the repository comes from; "current" is offered only when the current directory is the root of a git repository, root is that directory.
func promptSource(f *cmdutil.Factory, root string) (repoSource, error) {
	var options []huh.Option[repoSource]
	if root != "" {
		options = append(options, huh.NewOption("Current directory ("+filepath.Base(root)+")", sourceCurrent))
	}
	options = append(options,
		huh.NewOption("Repository on your computer", sourcePath),
		huh.NewOption("Sample repository", sourceSample),
	)
	var source repoSource
	if err := cmdutil.Select(f.Printer, "Which repository would you like to build?", options, &source); err != nil {
		return "", err
	}
	return source, nil
}

// promptPath asks for an absolute or relative path and resolves it as a git repository.
func promptPath(f *cmdutil.Factory) (repository, error) {
	// filepath.Join gives ~\projects\my-app on Windows; normalizePath expands both separators after ~
	example := filepath.Join("~", "projects", "my-app")
	input, err := promptDir(f, "Where is your local Git repository?", "Enter a local path, for example "+example+".", validateRepoPath)
	if err != nil {
		return repository{}, err
	}
	path, err := normalizePath(input)
	if err != nil {
		return repository{}, err
	}
	return resolveRepository(sourcePath, path)
}

// validateRepoPath rejects input that is not an existing directory at the root of a git repository.
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
	if !git.IsRepoRoot(path) {
		return fmt.Errorf("%s is not the root of a git repository", path)
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
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
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
