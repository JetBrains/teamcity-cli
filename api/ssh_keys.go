package api

import (
	"bytes"
	"fmt"
	"net/url"
)

// GetSSHKeys returns SSH keys uploaded to a project
func (c *Client) GetSSHKeys(projectID string) (*SSHKeyList, error) {
	path := fmt.Sprintf("/app/rest/projects/id:%s/sshKeys", projectID)

	var result SSHKeyList
	if err := c.get(c.ctx(), path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UploadSSHKey uploads a private SSH key to a project
func (c *Client) UploadSSHKey(projectID, name string, privateKey []byte) error {
	path := fmt.Sprintf("/app/rest/projects/id:%s/sshKeys/?fileName=%s", projectID, url.QueryEscape(name))
	return c.doNoContent(c.ctx(), "POST", path, bytes.NewReader(privateKey), "text/plain")
}

// GenerateSSHKey generates an SSH key pair in a project and returns the key with public key
func (c *Client) GenerateSSHKey(projectID, name, keyType string) (*SSHKey, error) {
	path := fmt.Sprintf("/app/rest/projects/id:%s/sshKeys/generated?keyName=%s&keyType=%s", projectID, url.QueryEscape(name), url.QueryEscape(keyType))

	var result SSHKey
	if err := c.post(c.ctx(), path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteSSHKey deletes an SSH key from a project
func (c *Client) DeleteSSHKey(projectID, name string) error {
	path := fmt.Sprintf("/app/rest/projects/id:%s/sshKeys/%s", projectID, url.PathEscape(name))
	return c.doNoContent(c.ctx(), "DELETE", path, nil, "")
}
