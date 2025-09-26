package notebook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client provides an API client for notebook operations
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient creates a new notebook API client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

// GenerateSharePin generates a new share pin
func (c *Client) GenerateSharePin() (string, error) {
	resp, err := c.client.Post(c.baseURL+"/api/share/generate-pin", "application/json", nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate pin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result struct {
		Pin string `json:"pin"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Pin, nil
}

// ListNotebooks lists all notebooks
func (c *Client) ListNotebooks() ([]*Notebook, error) {
	resp, err := c.client.Get(c.baseURL + "/api/notebooks")
	if err != nil {
		return nil, fmt.Errorf("failed to list notebooks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var notebooks []*Notebook
	if err := json.NewDecoder(resp.Body).Decode(&notebooks); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return notebooks, nil
}

// CreateNotebook creates a new notebook
func (c *Client) CreateNotebook(name string) (*Notebook, error) {
	reqBody := map[string]string{"name": name}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.client.Post(c.baseURL+"/api/notebooks", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create notebook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var notebook Notebook
	if err := json.NewDecoder(resp.Body).Decode(&notebook); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &notebook, nil
}

// GetNotebook retrieves a notebook by ID
func (c *Client) GetNotebook(id string) (*Notebook, []*Cell, error) {
	resp, err := c.client.Get(c.baseURL + "/api/notebooks/" + id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get notebook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result struct {
		*Notebook
		Cells []*Cell `json:"cells"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Notebook, result.Cells, nil
}
