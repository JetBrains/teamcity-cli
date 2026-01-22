package api

// GetCurrentUser returns the authenticated user
func (c *Client) GetCurrentUser() (*User, error) {
	var user User
	if err := c.get("/app/rest/users/current", &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetServer returns server information
func (c *Client) GetServer() (*Server, error) {
	var server Server
	if err := c.get("/app/rest/server", &server); err != nil {
		return nil, err
	}
	return &server, nil
}
