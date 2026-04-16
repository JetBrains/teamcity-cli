package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type CloudProfilesOptions struct {
	ProjectID string
	Limit     int
	Fields    []string
}

type CloudImagesOptions struct {
	ProjectID string
	Profile   string
	Limit     int
	Fields    []string
}

type CloudInstancesOptions struct {
	ProjectID string
	Image     string
	Limit     int
	Fields    []string
}

// cloudLocator normalizes a value into a cloud resource locator.
func cloudLocator(value, defaultDimension string) string {
	if defaultDimension == "name" {
		return cloudNameLocator(value)
	}
	return cloudIDLocator(value)
}

func cloudNameLocator(value string) string {
	switch {
	case strings.HasPrefix(value, "name:(") && strings.HasSuffix(value, ")"):
		return value
	case strings.HasPrefix(value, "name:"):
		return cloudNameValueLocator(strings.TrimPrefix(value, "name:"))
	case isCloudIDLikeLocator(value):
		return cloudIDLocator(value)
	default:
		return cloudNameValueLocator(value)
	}
}

func cloudIDLocator(value string) string {
	switch {
	case strings.HasPrefix(value, "id:(") && strings.HasSuffix(value, ")"):
		return value
	case strings.HasPrefix(value, "name:(") && strings.HasSuffix(value, ")"):
		return value
	case strings.HasPrefix(value, "name:"):
		return cloudNameValueLocator(strings.TrimPrefix(value, "name:"))
	case strings.Contains(value, ","):
		return "id:(" + value + ")"
	case isCloudIDLikeLocator(value):
		return value
	case strings.Contains(value, ":"):
		return "id:(" + value + ")"
	default:
		return "id:" + value
	}
}

func cloudNameValueLocator(value string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(value))
	return "name:(value:($base64:" + encoded + "))"
}

func isCloudIDLikeLocator(value string) bool {
	return strings.HasPrefix(value, "id:") ||
		strings.HasPrefix(value, "profileId:") ||
		strings.HasPrefix(value, "imageId:") ||
		strings.HasPrefix(value, "projectId:")
}

func (c *Client) GetCloudProfiles(opts CloudProfilesOptions) (*CloudProfileList, error) {
	locator := NewLocator().
		Add("project", opts.ProjectID).
		AddIntDefault("count", opts.Limit, 100)

	fields := opts.Fields
	if len(fields) == 0 {
		fields = CloudProfileFields.Default
	}
	fieldsParam := fmt.Sprintf("count,nextHref,cloudProfile(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/cloud/profiles?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))

	profiles, err := collectPages(c, path, opts.Limit, func(p string) ([]CloudProfile, string, error) {
		var page CloudProfileList
		if err := c.get(p, &page); err != nil {
			return nil, "", err
		}
		return page.Profiles, page.NextHref, nil
	})
	if err != nil {
		return nil, err
	}
	return &CloudProfileList{Count: len(profiles), Profiles: profiles}, nil
}

func (c *Client) GetCloudProfile(locator string) (*CloudProfile, error) {
	path := fmt.Sprintf("/app/rest/cloud/profiles/%s", cloudLocator(locator, "id"))

	var result CloudProfile
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetCloudImages(opts CloudImagesOptions) (*CloudImageList, error) {
	locator := NewLocator().
		Add("project", opts.ProjectID)
	if opts.Profile != "" {
		locator.AddRaw("profile", "("+cloudLocator(opts.Profile, "id")+")")
	}
	locator.AddIntDefault("count", opts.Limit, 100)

	fields := opts.Fields
	if len(fields) == 0 {
		fields = CloudImageFields.Default
	}
	fieldsParam := fmt.Sprintf("count,nextHref,cloudImage(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/cloud/images?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))

	images, err := collectPages(c, path, opts.Limit, func(p string) ([]CloudImage, string, error) {
		var page CloudImageList
		if err := c.get(p, &page); err != nil {
			return nil, "", err
		}
		return page.Images, page.NextHref, nil
	})
	if err != nil {
		return nil, err
	}
	return &CloudImageList{Count: len(images), Images: images}, nil
}

func (c *Client) GetCloudImage(locator string) (*CloudImage, error) {
	path := fmt.Sprintf("/app/rest/cloud/images/%s", cloudLocator(locator, "name"))

	var result CloudImage
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetCloudInstances(opts CloudInstancesOptions) (*CloudInstanceList, error) {
	locator := NewLocator().
		Add("project", opts.ProjectID)
	if opts.Image != "" {
		locator.AddRaw("image", "("+cloudLocator(opts.Image, "name")+")")
	}
	locator.AddIntDefault("count", opts.Limit, 100)

	fields := opts.Fields
	if len(fields) == 0 {
		fields = CloudInstanceFields.Default
	}
	fieldsParam := fmt.Sprintf("count,nextHref,cloudInstance(%s)", ToAPIFields(fields))
	path := fmt.Sprintf("/app/rest/cloud/instances?locator=%s&fields=%s", locator.Encode(), url.QueryEscape(fieldsParam))

	instances, err := collectPages(c, path, opts.Limit, func(p string) ([]CloudInstance, string, error) {
		var page CloudInstanceList
		if err := c.get(p, &page); err != nil {
			return nil, "", err
		}
		return page.Instances, page.NextHref, nil
	})
	if err != nil {
		return nil, err
	}
	return &CloudInstanceList{Count: len(instances), Instances: instances}, nil
}

func (c *Client) GetCloudInstance(locator string) (*CloudInstance, error) {
	path := fmt.Sprintf("/app/rest/cloud/instances/%s", cloudLocator(locator, "id"))

	var result CloudInstance
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) StartCloudInstance(imageID string) (*CloudInstance, error) {
	body, err := json.Marshal(StartCloudInstanceRequest{
		Image: CloudImageRef{ID: imageID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var result CloudInstance
	if err := c.post("/app/rest/cloud/instances", bytes.NewReader(body), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) StopCloudInstance(locator string, force bool) error {
	action := "stop"
	if force {
		action = "forceStop"
	}
	path := fmt.Sprintf("/app/rest/cloud/instances/%s/actions/%s", cloudLocator(locator, "id"), action)
	return c.doNoContent("POST", path, nil, "")
}
