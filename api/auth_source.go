package api

// AuthSource identifies the origin of a Client's credentials, used to pick the right 403 tip.
type AuthSource string

const (
	AuthSourceUnknown AuthSource = ""
	AuthSourcePKCE    AuthSource = "pkce"
	AuthSourceManual  AuthSource = "manual"
	AuthSourceEnv     AuthSource = "env"
	AuthSourceBuild   AuthSource = "build"
	AuthSourceGuest   AuthSource = "guest"
)

// WithAuthSource records how the client's credentials were obtained.
func WithAuthSource(src AuthSource) ClientOption {
	return func(c *Client) {
		c.AuthSource = src
	}
}
