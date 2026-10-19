package onboard

import "github.com/JetBrains/teamcity-cli/api"

// isRepositoryAuthorized reports whether the server already has a VCS root pointing at repo.
func isRepositoryAuthorized(_ api.ClientInterface, _ repository) (bool, error) {
	// TODO: match git.CanonicalURL on both sides
	return false, nil
}
