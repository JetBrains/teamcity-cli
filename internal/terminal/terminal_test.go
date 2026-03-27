package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	c := NewClient("https://tc.example.com/", "admin", "token123", func(string, ...any) {})
	assert.Equal(t, "https://tc.example.com", c.baseURL)
	assert.Equal(t, "admin", c.username)
	assert.Equal(t, "token123", c.token)
	assert.NotNil(t, c.httpClient)
	assert.NotNil(t, c.httpClient.Jar)
}

func TestNewClientEmptyUsername(t *testing.T) {
	c := NewClient("http://localhost:8111", "", "tok", func(string, ...any) {})
	assert.Equal(t, "http://localhost:8111", c.baseURL)
	assert.Empty(t, c.username)
}
