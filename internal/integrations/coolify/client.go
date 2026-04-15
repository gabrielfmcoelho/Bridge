package coolify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client interacts with the Coolify REST API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a Coolify API client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) do(method, path string, body any) ([]byte, int, error) {
	u := fmt.Sprintf("%s/api/v1%s", c.baseURL, path)

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, u, reqBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("coolify api %s %s: %d %s", method, path, resp.StatusCode, string(data))
	}
	return data, resp.StatusCode, nil
}

// ListServers returns all servers.
func (c *Client) ListServers() ([]Server, error) {
	data, _, err := c.do("GET", "/servers", nil)
	if err != nil {
		return nil, err
	}
	var servers []Server
	return servers, json.Unmarshal(data, &servers)
}

// GetServer returns a server by UUID.
func (c *Client) GetServer(uuid string) (*Server, error) {
	data, _, err := c.do("GET", "/servers/"+uuid, nil)
	if err != nil {
		return nil, err
	}
	var s Server
	return &s, json.Unmarshal(data, &s)
}

// CreateServer creates a new server and returns its UUID.
func (c *Client) CreateServer(req CreateServerRequest) (string, error) {
	data, _, err := c.do("POST", "/servers", req)
	if err != nil {
		return "", err
	}
	var resp struct {
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return resp.UUID, nil
}

// ValidateServer triggers async validation of a server.
func (c *Client) ValidateServer(uuid string) error {
	_, _, err := c.do("GET", "/servers/"+uuid+"/validate", nil)
	return err
}

// UpdateServer updates a server's fields.
func (c *Client) UpdateServer(uuid string, req UpdateServerRequest) error {
	_, _, err := c.do("PATCH", "/servers/"+uuid, req)
	return err
}

// DeleteServer removes a server.
func (c *Client) DeleteServer(uuid string) error {
	_, _, err := c.do("DELETE", "/servers/"+uuid, nil)
	return err
}

// ListPrivateKeys returns all private keys.
func (c *Client) ListPrivateKeys() ([]PrivateKey, error) {
	data, _, err := c.do("GET", "/security/keys", nil)
	if err != nil {
		return nil, err
	}
	var keys []PrivateKey
	return keys, json.Unmarshal(data, &keys)
}

// CreatePrivateKey uploads a private key and returns its UUID.
func (c *Client) CreatePrivateKey(req CreateKeyRequest) (string, error) {
	data, _, err := c.do("POST", "/security/keys", req)
	if err != nil {
		return "", err
	}
	var resp struct {
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return resp.UUID, nil
}

// Healthcheck calls GET /health to verify the Coolify instance is reachable and the token is valid.
func (c *Client) Healthcheck() error {
	_, _, err := c.do("GET", "/health", nil)
	return err
}

// FindServerByIP searches all servers for one matching the given IP.
func (c *Client) FindServerByIP(ip string) (*Server, error) {
	servers, err := c.ListServers()
	if err != nil {
		return nil, err
	}
	for _, s := range servers {
		if s.IP == ip {
			return &s, nil
		}
	}
	return nil, nil
}
