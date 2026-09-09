package project

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestResolveStoredToken(t *testing.T) {
	t.Parallel()
	for _, username := range []string{"", "x-access-token"} {
		props, req, err := resolveAuth(nil, nil, "Project", authToken, &vcsCreateOptions{tokenID: "tc_token_id:CID_test:-1:uuid", username: username}, false)
		assert.NoError(t, err)
		actual := map[string]string{}
		for _, prop := range props {
			actual[prop.Name] = prop.Value
		}
		assert.Equal(t, "ACCESS_TOKEN", actual["authMethod"])
		assert.Equal(t, "tc_token_id:CID_test:-1:uuid", actual["tokenId"])
		assert.NotContains(t, actual, "secure:password")
		assert.Empty(t, req.ConnectionID)
		if username == "" {
			assert.Equal(t, "oauth2", actual["username"])
		} else {
			assert.Equal(t, username, actual["username"])
		}
	}
}
