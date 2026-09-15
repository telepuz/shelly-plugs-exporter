package shelly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

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
	reqURL := c.baseURL + "/Switch.GetStatus?id=0"
	var status SwitchStatus
	err := c.withRetry(ctx, func() error {
		return c.get(ctx, reqURL, &status)
	})
	if err != nil {
		return nil, fmt.Errorf("Switch.GetStatus: %w", err)
	}
	return &status, nil
}

func (c *Client) GetSysStatus(ctx context.Context) (*SysStatus, error) {
	reqURL := c.baseURL + "/Sys.GetStatus"
	var status SysStatus
	err := c.withRetry(ctx, func() error {
		return c.get(ctx, reqURL, &status)
	})
	if err != nil {
		return nil, fmt.Errorf("Sys.GetStatus: %w", err)
	}
	return &status, nil
}

func (c *Client) withRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
		err := fn()
		if err == nil {
			return nil
		}
		if isNetworkError(err) {
			lastErr = err
			continue
		}
		return err
	}
	return lastErr
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() || urlErr.Temporary() {
			return true
		}
		if errors.Is(urlErr.Err, io.EOF) || errors.Is(urlErr.Err, io.ErrUnexpectedEOF) {
			return true
		}
		var opErr *net.OpError
		if errors.As(urlErr.Err, &opErr) {
			return true
		}
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	return false
}

func (c *Client) get(ctx context.Context, reqURL string, dst interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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
