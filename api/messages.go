package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

// BuildMessage represents a single message from the build log.
type BuildMessage struct {
	ID               int    `json:"id"`
	Text             string `json:"text"`
	Level            int    `json:"level"`
	Status           int    `json:"status"`
	Timestamp        string `json:"timestamp,omitempty"`
	ServerTimestamp   string `json:"serverTimestamp,omitempty"`
	FlowID           int    `json:"flowId,omitempty"`
	ParentID         int    `json:"parentId,omitempty"`
	BlockType        string `json:"blockType,omitempty"`
	ContainsMessages bool   `json:"containsMessages,omitempty"`
	Verbose          bool   `json:"verbose,omitempty"`
}

// BuildMessagesResponse is the response from the /app/messages endpoint.
type BuildMessagesResponse struct {
	Messages            []BuildMessage `json:"messages"`
	LastMessageIndex    int            `json:"lastMessageIndex"`
	FocusIndex          int            `json:"focusIndex"`
	LastMessageIncluded bool           `json:"lastMessageIncluded"`
}

// BuildMessagesOptions controls how messages are fetched.
type BuildMessagesOptions struct {
	// Count is the number of messages to fetch. Negative values fetch from the tail.
	Count int
	// SinceID skips messages up to this ID (used as the first param of messagesCount).
	SinceID int
	// Tail mode fetches from the end of the log.
	Tail bool
	// ExpandAll looks inside block messages.
	ExpandAll bool
}

// GetBuildMessages fetches structured log messages via /app/messages.
func (c *Client) GetBuildMessages(buildID string, opts BuildMessagesOptions) (*BuildMessagesResponse, error) {
	id, err := c.ResolveBuildID(buildID)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("buildId", id)
	params.Set("messagesCount", fmt.Sprintf("%d,%d", opts.SinceID, opts.Count))
	if opts.Tail {
		params.Set("target", "tail")
	}
	if opts.ExpandAll {
		params.Set("expandAll", "true")
	}

	path := "/app/messages?" + params.Encode()

	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get build messages: status %d: %s", resp.StatusCode, string(body))
	}

	var result BuildMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode build messages: %w", err)
	}

	return &result, nil
}
