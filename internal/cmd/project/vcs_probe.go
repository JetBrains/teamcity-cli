package project

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"golang.org/x/net/html"
)

// testSavedVcsRoot invokes the editor's test action without saving any settings.
func testSavedVcsRoot(f *cmdutil.Factory, client api.ClientInterface, root *api.VcsRoot) error {
	unavailable := func(reason string) error {
		return api.Validation("cannot test saved VCS root: "+reason,
			"Use Test connection in the TeamCity UI: "+vcsRootEditURL(root.ID)+" (CLI testing requires access to /admin/editVcsRoot.html)")
	}
	path := "/admin/editVcsRoot.html?" + (url.Values{"action": {"editVcsRoot"}, "vcsRootId": {root.ID}}).Encode()
	response, err := client.RawRequest(f.Context(), http.MethodGet, path, nil, map[string]string{"Accept": "text/html"})
	if err != nil {
		return fmt.Errorf("load VCS root test form: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return unavailable(fmt.Sprintf("editor returned HTTP %d", response.StatusCode))
	}
	form := url.Values{}
	tokenizer := html.NewTokenizer(bytes.NewReader(response.Body))
	for tokenType := tokenizer.Next(); tokenType != html.ErrorToken; tokenType = tokenizer.Next() {
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		if token.Data != "input" {
			continue
		}
		var name, value, inputType string
		for _, attribute := range token.Attr {
			switch attribute.Key {
			case "name":
				name = attribute.Val
			case "value":
				value = attribute.Val
			case "type":
				inputType = attribute.Val
			}
		}
		if strings.EqualFold(inputType, "hidden") && (name == "publicKey" || strings.HasPrefix(name, "prop:encrypted:")) {
			form.Set(name, value)
		}
	}
	if form.Get("publicKey") == "" {
		return unavailable("server did not return a VCS root test form (it may require browser sign-in)")
	}
	if root.Properties == nil || root.Project == nil {
		return unavailable("server did not return the root's properties and owning project")
	}
	for _, property := range root.Properties.Property {
		if form.Has("prop:encrypted:" + property.Name) {
			continue
		}
		// REST masks stored secrets; never replace one with an empty or masked value.
		if strings.HasPrefix(property.Name, "secure:") {
			return unavailable("server did not return the root's encrypted credentials")
		}
		form.Set("prop:"+property.Name, property.Value)
	}
	form.Set("editingScope", "editProject:"+root.Project.ID)
	form.Set("submitVcsRoot", "testConnection")
	response, err = client.RawRequest(f.Context(), http.MethodPost, path, strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded", "Accept": "application/xml",
	})
	if err != nil {
		return fmt.Errorf("test saved VCS root: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return unavailable(fmt.Sprintf("test action returned HTTP %d", response.StatusCode))
	}
	var result struct {
		XMLName xml.Name `xml:"response"`
		Result  *string  `xml:"testConnectionResult"`
		Errors  []struct {
			ID      string `xml:"id,attr"`
			Message string `xml:",chardata"`
		} `xml:"errors>error"`
	}
	if err := xml.Unmarshal(response.Body, &result); err != nil {
		return unavailable("server did not return a test result (it may require browser sign-in)")
	}
	if len(result.Errors) != 0 {
		messages := make([]string, 0, len(result.Errors))
		for _, item := range result.Errors {
			if item.ID == "publicKeyExpired" {
				return unavailable("the server encryption key changed; retry the command")
			}
			message := strings.TrimSpace(item.Message)
			if message == "" {
				message = item.ID
			}
			messages = append(messages, message)
		}
		return fmt.Errorf("test connection failed: %s", strings.Join(messages, "; "))
	}
	if result.Result == nil {
		return unavailable("server response did not contain a test result")
	}
	return nil
}
