package contextos

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) GetHealth() (map[string]interface{}, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/api/v1/health")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) DispatchMCPTool(toolName string, args map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"tool_name": toolName,
		"arguments": args,
	}
	bodyBytes, _ := json.Marshal(payload)

	resp, err := c.HTTPClient.Post(c.BaseURL+"/api/v1/mcp/dispatch", "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
