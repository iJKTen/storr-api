package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// RuntimeClient communicates with pipeline containers over HTTP to query status
// and forward user interactions like theme selection and input.
type RuntimeClient struct {
	token      string
	httpClient *http.Client
}

// RuntimeStatus represents the JSON response from a pipeline container's /status endpoint.
type RuntimeStatus struct {
	Status           string   `json:"status"`
	State            string   `json:"state"`
	Error            string   `json:"error,omitempty"`
	AuthURL          string   `json:"authUrl,omitempty"`
	Themes           []Theme  `json:"themes,omitempty"`
	ExistingThemeIDs []string `json:"existingThemeIds,omitempty"`
	Output           string   `json:"output,omitempty"`
	PreviewURL       string   `json:"previewUrl,omitempty"`
	PreviewPort      int      `json:"previewPort,omitempty"`
}

// Theme represents a Shopify theme returned by the pipeline container's runtime status.
type Theme struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// ThemeSelection holds the user's theme choice to be forwarded to the pipeline container.
type ThemeSelection struct {
	ThemeID     string
	UseExisting bool
}

// NewRuntimeClient creates a RuntimeClient that authenticates with the given bearer token.
func NewRuntimeClient(token string) *RuntimeClient {
	return &RuntimeClient{
		token:      token,
		httpClient: &http.Client{Timeout: RuntimeHTTPTimeout},
	}
}

func (c *RuntimeClient) newRequest(ctx context.Context, method, ip, path string) (*http.Request, error) {
	url := fmt.Sprintf("http://%s:%d%s", ip, RuntimePort, path)
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

func (c *RuntimeClient) doJSON(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("runtime returned status %d", resp.StatusCode)
	}
	return resp, nil
}

// GetStatus queries the pipeline container's /status endpoint and returns the parsed response.
func (c *RuntimeClient) GetStatus(ctx context.Context, ip string) (*RuntimeStatus, error) {
	req, err := c.newRequest(ctx, http.MethodGet, ip, StatusPath)
	if err != nil {
		return nil, err
	}
	resp, err := c.doJSON(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var status RuntimeStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// SelectTheme posts the user's theme selection to the pipeline container's /select-theme endpoint.
func (c *RuntimeClient) SelectTheme(ctx context.Context, ip string, sel ThemeSelection) error {
	body, err := json.Marshal(selectThemePayload{
		ThemeID:     sel.ThemeID,
		UseExisting: sel.UseExisting,
	})
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, http.MethodPost, ip, SelectThemePath)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Body = newReadCloser(body)
	resp, err := c.doJSON(req)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}

// ProvideInput posts user-provided text input to the pipeline container's /provide-input endpoint.
func (c *RuntimeClient) ProvideInput(ctx context.Context, ip, input string) error {
	body, err := json.Marshal(inputPayload{Input: input})
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, http.MethodPost, ip, ProvideInputPath)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Body = newReadCloser(body)
	resp, err := c.doJSON(req)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}

func newReadCloser(data []byte) *readCloser {
	return &readCloser{Reader: bytes.NewReader(data)}
}

type readCloser struct {
	*bytes.Reader
}

func (rc *readCloser) Close() error {
	return nil
}

type selectThemePayload struct {
	ThemeID     string `json:"themeId"`
	UseExisting bool   `json:"useExisting"`
}

type inputPayload struct {
	Input string `json:"input"`
}
