package contextos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

// ExecutionResponse represents the response details of a task execution.
type ExecutionResponse struct {
	SessionID    string                 `json:"session_id"`
	CurrentState string                 `json:"current_state"`
	Payload      map[string]interface{} `json:"payload"`
}

// CheckpointResponse represents the persisted state checkpoint of a session.
type CheckpointResponse struct {
	SessionID    string                 `json:"session_id"`
	CurrentState string                 `json:"current_state"`
	Payload      map[string]interface{} `json:"payload"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

// ExecuteAgent submits a task execution or resumption request to the microkernel.
func (c *Client) ExecuteAgent(sessionID string, task string, contextData map[string]interface{}) (*ExecutionResponse, error) {
	reqBody := map[string]interface{}{
		"session_id": sessionID,
		"task":       task,
		"context":    contextData,
	}

	bytesBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v1/agent/execute", strings.TrimSuffix(c.BaseURL, "/"))
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(bytesBody))
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var execResp ExecutionResponse
	if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &execResp, nil
}

// GetState retrieves the active execution checkpoint attributes from the Go microkernel.
func (c *Client) GetState(sessionID string) (*CheckpointResponse, error) {
	apiURL := fmt.Sprintf("%s/api/v1/agent/state/%s", strings.TrimSuffix(c.BaseURL, "/"), url.PathEscape(sessionID))
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var checkResp CheckpointResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &checkResp, nil
}

// StreamTrace connects to the WebSocket stream and forwards all session-specific traces to the handler.
func (c *Client) StreamTrace(sessionID string, eventHandler func(event map[string]interface{})) error {
	// Map http:// to ws:// and https:// to wss://
	wsBase := strings.TrimSuffix(c.BaseURL, "/")
	wsBase = strings.Replace(wsBase, "http://", "ws://", 1)
	wsBase = strings.Replace(wsBase, "https://", "wss://", 1)

	wsURL := fmt.Sprintf("%s/ws/trace", wsBase)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		var event map[string]interface{}
		if err := json.Unmarshal(message, &event); err == nil {
			// Dispatch only if it relates to our active session ID
			if payload, ok := event["payload"].(map[string]interface{}); ok {
				if sid, ok := payload["session_id"].(string); ok && (sessionID == "" || sid == sessionID) {
					eventHandler(event)
				}
			} else {
				eventHandler(event) // fallback
			}
		}
	}
}
