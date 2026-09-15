package shelly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/icholy/digest"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(address, username, password string) *Client {
	transport := http.DefaultTransport
	if username != "" {
		transport = &digest.Transport{
			Username:  username,
			Password:  password,
			Transport: http.DefaultTransport,
		}
	}
	return &Client{
		httpClient: &http.Client{Transport: transport},
		baseURL:    "http://" + address + "/rpc",
	}
}

func (c *Client) GetSwitchStatus(ctx context.Context) (*SwitchStatus, error) {
	url := c.baseURL + "/Switch.GetStatus?id=0"
	var status SwitchStatus
	if err := c.get(ctx, url, &status); err != nil {
		return nil, fmt.Errorf("Switch.GetStatus: %w", err)
	}
	return &status, nil
}

func (c *Client) GetSysStatus(ctx context.Context) (*SysStatus, error) {
	url := c.baseURL + "/Sys.GetStatus"
	var status SysStatus
	if err := c.get(ctx, url, &status); err != nil {
		return nil, fmt.Errorf("Sys.GetStatus: %w", err)
	}
	return &status, nil
}

func (c *Client) get(ctx context.Context, url string, dst interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}
