package onboard

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/git"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/charmbracelet/huh"
)

type cloneChoice string

const (
	cloneHere  cloneChoice = "here"
	cloneOther cloneChoice = "other"
	cloneSkip  cloneChoice = "skip"
)

// cloneSample asks for consent and clones a sample repository locally so the agent can read its files; dir stays empty when declined.
func cloneSample(f *cmdutil.Factory, repo repository) (repository, error) {
	// TODO: copy pending design review
	name := cloneDirName(repo.remoteURL)
	cwd, err := os.Getwd()
	if err != nil {
		return repo, fmt.Errorf("resolve current directory: %w", err)
	}

	options := []huh.Option[cloneChoice]{
		huh.NewOption("Yes, into "+filepath.Join(cwd, name), cloneHere),
		huh.NewOption("Yes, into another directory", cloneOther),
		huh.NewOption("No", cloneSkip),
	}
	var choice cloneChoice
	if err := cmdutil.Select(f.Printer, "Clone "+repo.remoteURL+" so the agent can access its files?", options, &choice); err != nil {
		return repo, err
	}

	parent := cwd
	switch choice {
	case cloneSkip:
		return repo, nil
	case cloneOther:
		var input string
		if err := cmdutil.PromptValidatedString(f.Printer, "Provide a directory to clone into", "", validateCloneParent, &input); err != nil {
			return repo, err
		}
		if parent, err = normalizePath(input); err != nil {
			return repo, err
		}
	}

	target := filepath.Join(parent, name)
	if _, err := os.Stat(target); err == nil {
		overwrite, err := promptExistingTarget(f, target)
		if err != nil {
			return repo, err
		}
		if !overwrite {
			repo.dir = target // assume it's a previous clone
			return repo, nil
		}
		if err := os.RemoveAll(target); err != nil {
			return repo, fmt.Errorf("remove %s: %w", target, err)
		}
	}
	f.Printer.Progress("Cloning %s into %s... ", output.Cyan(repo.remoteURL), target)
	if err := cloneAt(repo.remoteURL, target, repo.revision); err != nil {
		f.Printer.Info("%s", output.Red(output.Sym().Cross))
		return repo, fmt.Errorf("clone %s: %w", repo.remoteURL, err)
	}
	f.Printer.Info("%s", output.Green(output.Sym().Check))
	repo.dir = target
	return repo, nil
}

// cloneAt clones remoteURL into dir and resets its default branch to revision (HEAD when empty); a full clone, since a shallow one can't reach older commits.
func cloneAt(remoteURL, dir, revision string) error {
	for _, args := range [][]string{
		{"clone", "--no-checkout", "--", remoteURL, dir},
		{"-C", dir, "reset", "--hard", cmp.Or(revision, "HEAD")},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			if msg := strings.TrimSpace(string(out)); msg != "" {
				return errors.New(msg)
			}
			return err
		}
	}
	return nil
}

// cloneDirName is the directory a clone of remoteURL lands in, e.g. "spring-petclinic".
func cloneDirName(remoteURL string) string {
	return path.Base(cmp.Or(git.RepoPath(remoteURL), "sample"))
}

// promptExistingTarget asks whether to reuse an existing clone target or delete it and clone again; true means overwrite.
func promptExistingTarget(f *cmdutil.Factory, target string) (bool, error) {
	// TODO: copy pending design review
	// Non-destructive option first so Enter doesn't delete anything.
	options := []huh.Option[bool]{
		huh.NewOption("Use existing directory", false),
		huh.NewOption("Overwrite (delete and clone again)", true),
	}
	var overwrite bool
	if err := cmdutil.Select(f.Printer, target+" already exists", options, &overwrite); err != nil {
		return false, err
	}
	return overwrite, nil
}

// validateCloneParent rejects input that is not an existing directory; an existing clone target is handled by promptExistingTarget.
func validateCloneParent(input string) error {
	parent, err := normalizePath(input)
	if err != nil {
		return err
	}
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("%s does not exist", parent)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", parent)
	}
	return nil
}
