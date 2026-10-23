package onboard

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/git"
	"github.com/charmbracelet/huh"
)

// provider describes a VCS-capable OAuth connection type: the public hosts it serves (none for self-hosted) and its authorize URL segment.
type provider struct {
	hosts            []string
	authorizeSegment string
}

// providers are the connection types that can back a VCS root, keyed by providerType.
var providers = map[string]provider{
	"GitHub":          {hosts: []string{"github.com"}, authorizeSegment: "github"},
	"GitHubApp":       {hosts: []string{"github.com"}, authorizeSegment: "githubapp"},
	"GHE":             {authorizeSegment: "github"},
	"GitLabCom":       {hosts: []string{"gitlab.com"}, authorizeSegment: "gitlab"},
	"GitLabCEorEE":    {authorizeSegment: "gitlab"},
	"BitBucketCloud":  {hosts: []string{"bitbucket.org"}, authorizeSegment: "bitbucket"},
	"AzureDevOps":     {hosts: []string{"dev.azure.com", "ssh.dev.azure.com"}, authorizeSegment: "azure"},
	"JetBrains Space": {authorizeSegment: "space"},
}

// connection is a VCS-capable OAuth connection and the instance project that owns it; VCS roots using it must live there or below.
type connection struct {
	id           string
	name         string
	providerType string
	projectID    string
	serverHosts  []string // hosts of URL-valued properties, for self-hosted providers
}

// label renders a connection as "Name (providerType)" for prompts.
func (c connection) label() string {
	return c.name + " (" + c.providerType + ")"
}

// listConnections returns the VCS-capable connections visible from projectID.
func listConnections(client api.ClientInterface, projectID string) ([]connection, error) {
	feats, err := client.GetProjectConnections(projectID)
	if err != nil {
		return nil, fmt.Errorf("list connections in %s: %w", projectID, err)
	}
	conns := []connection{}
	for _, feat := range feats.ProjectFeature {
		c := connection{id: feat.ID, name: feat.ID, projectID: projectID}
		if feat.Properties != nil {
			for _, p := range feat.Properties.Property {
				switch p.Name {
				case "providerType":
					c.providerType = p.Value
				case "displayName":
					c.name = p.Value
				}
				if u, err := url.Parse(p.Value); err == nil && u.Host != "" {
					c.serverHosts = append(c.serverHosts, strings.ToLower(u.Hostname()))
				}
			}
		}
		if _, ok := providers[c.providerType]; ok {
			conns = append(conns, c)
		}
	}
	return conns, nil
}

// matchConnections returns the connections that serve the repo's host: public providers by known host, self-hosted ones by a URL in their settings.
func matchConnections(conns []connection, remoteURL string) []connection {
	host := repoHost(remoteURL)
	if host == "" {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(conns), func(c connection) bool {
		return !slices.Contains(providers[c.providerType].hosts, host) && !slices.Contains(c.serverHosts, host)
	})
}

// repoHost is the lowercase host of remoteURL (git@github.com:acme/app.git → github.com), or "" when unparseable.
func repoHost(remoteURL string) string {
	host, _, _ := strings.Cut(git.CanonicalURL(remoteURL), "/")
	return host
}

// chooseConnection picks the connection for remoteURL: the only match as-is, a prompt among several matches, or a prompt among all when none match.
func chooseConnection(f *cmdutil.Factory, conns []connection, remoteURL string) (connection, error) {
	candidates := matchConnections(conns, remoteURL)
	// TODO: copy pending design review
	title := "Select a connection for " + remoteURL
	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		candidates = conns
		title = "Couldn't match a connection to " + remoteURL + ". Select one"
	}

	options := make([]huh.Option[int], len(candidates))
	for i, c := range candidates {
		options[i] = huh.NewOption(c.label(), i)
	}
	var picked int
	if err := cmdutil.Select(f.Printer, title, options, &picked); err != nil {
		return connection{}, err
	}
	return candidates[picked], nil
}

// authorizeURL is the per-user OAuth page for conn, or "" for providers without one.
func authorizeURL(serverURL string, conn connection) string {
	seg := providers[conn.providerType].authorizeSegment
	if seg == "" {
		return ""
	}
	return fmt.Sprintf("%s/oauth/%s/repositories.html?projectId=%s&connectionId=%s&updateToken=true&showMode=popup",
		serverURL, seg, url.QueryEscape(conn.projectID), url.QueryEscape(conn.id))
}
