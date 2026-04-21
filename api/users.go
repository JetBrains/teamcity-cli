package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// GetCurrentUser returns the authenticated user
func (c *Client) GetCurrentUser() (*User, error) {
	var user User
	if err := c.get(context.Background(), "/app/rest/users/current?fields=username,name", &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUser returns a user by username
func (c *Client) GetUser(username string) (*User, error) {
	path := "/app/rest/users/username:" + username

	var user User
	if err := c.get(context.Background(), path, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// UserExists checks if a user exists
func (c *Client) UserExists(username string) bool {
	_, err := c.GetUser(username)
	return err == nil
}

// Role represents a TeamCity role assignment
type Role struct {
	RoleID string `json:"roleId"`
	Scope  string `json:"scope"` // "g" for global, "p:ProjectID" for project
}

// RoleList represents a list of role assignments
type RoleList struct {
	Role []Role `json:"role"`
}

// CreateUserRequest represents a request to create a user
type CreateUserRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Name     string   `json:"name,omitempty"`
	Email    string   `json:"email,omitempty"`
	Roles    RoleList `json:"roles"`
}

// CreateUser creates a new user
func (c *Client) CreateUser(req CreateUserRequest) (*User, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var user User
	if err := c.post(context.Background(), "/app/rest/users", bytes.NewReader(body), &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// Token represents an API token
type Token struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// CreateAPIToken creates an API token for the current user
func (c *Client) CreateAPIToken(name string) (*Token, error) {
	path := "/app/rest/users/current/tokens/" + name

	var token Token
	if err := c.post(context.Background(), path, nil, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// DeleteAPIToken deletes an API token for the current user
func (c *Client) DeleteAPIToken(name string) error {
	path := "/app/rest/users/current/tokens/" + name
	return c.doNoContent(context.Background(), "DELETE", path, nil, "")
}

// GetServer returns server information
func (c *Client) GetServer() (*Server, error) {
	var server Server
	if err := c.get(context.Background(), "/app/rest/server", &server); err != nil {
		return nil, err
	}
	return &server, nil
}
