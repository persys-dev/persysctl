package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/persys-dev/persys-cli/internal/auth"
	"github.com/persys-dev/persys-cli/internal/config"
	"github.com/persys-dev/persys-cli/internal/models"
)

type Client struct {
	cfg        config.Config
	httpClient *http.Client
	certMgr    *auth.CertificateManager
}

type ScheduleResponse struct {
	WorkloadID string `json:"workloadId"`
	NodeID     string `json:"nodeId"`
	Status     string `json:"status"`
}

func NewClient(cfg config.Config) (*Client, error) {
	// Enforce HTTPS for mTLS endpoints
	if cfg.APIEndpoint != "" && cfg.APIEndpoint[:8] != "https://" {
		return nil, fmt.Errorf("APIEndpoint must start with https:// when using mTLS, got: %s", cfg.APIEndpoint)
	}
	// Initialize certificate manager
	certMgr := auth.NewCertificateManager(
		cfg.CACertPath,
		cfg.CertPath,
		cfg.KeyPath,
		cfg.CFSSLApiURL,
		cfg.CommonName,
		cfg.Organization,
	)

	// Ensure certificate exists and is valid
	if err := certMgr.EnsureCertificate(); err != nil {
		return nil, fmt.Errorf("failed to ensure certificate: %w", err)
	}

	// Get TLS config
	tlsConfig, err := certMgr.GetTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS config: %w", err)
	}

	// Create HTTP client with TLS config
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
		certMgr:    certMgr,
	}, nil
}

func (c *Client) ScheduleWorkload(workload models.Workload) (*ScheduleResponse, error) {
	payload, err := json.Marshal(workload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workload: %v", err)
	}

	resp, err := c.makeRequest("POST", "/workloads/schedule", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Log X-Service-Name header
	serviceName := resp.Header.Get("X-Service-Name")
	if serviceName != "" {
		config.Logf("Response from service: %s", serviceName)
	}

	// Read the response body once
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	fmt.Printf("Raw response body: %s\n", string(body))

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Decode the response
	var scheduleResp ScheduleResponse
	if err := json.Unmarshal(body, &scheduleResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	config.Logf("Scheduled workload %s on node %s", scheduleResp.WorkloadID, scheduleResp.NodeID)
	return &scheduleResp, nil
}

// Helper function to make HTTP requests
func (c *Client) makeRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := c.cfg.APIEndpoint + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	return resp, nil
}

func (c *Client) ListWorkloads() ([]models.Workload, error) {
	req, err := http.NewRequest("GET", c.cfg.APIEndpoint+"/workloads", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Log X-Service-Name header
	serviceName := resp.Header.Get("X-Service-Name")
	if serviceName != "" {
		config.Logf("Response from service: %s", serviceName)
	}

	// Read the response body once
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	fmt.Printf("Raw response body: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var workloads []models.Workload
	err = json.Unmarshal(body, &workloads)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return workloads, nil
}

func (c *Client) ListNodes() ([]models.Node, error) {
	req, err := http.NewRequest("GET", c.cfg.APIEndpoint+"/nodes", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Log X-Service-Name header
	serviceName := resp.Header.Get("X-Service-Name")
	if serviceName != "" {
		config.Logf("Response from service: %s", serviceName)
	}

	// Read the response body once
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	fmt.Printf("Raw response body: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Nodes []models.Node `json:"nodes"`
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return result.Nodes, nil
}

func (c *Client) GetMetrics() (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", c.cfg.APIEndpoint+"/cluster/metrics", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Log X-Service-Name header
	serviceName := resp.Header.Get("X-Service-Name")
	if serviceName != "" {
		config.Logf("Response from service: %s", serviceName)
	}

	// Read the response body once
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	fmt.Printf("Raw response body: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var metrics map[string]interface{}
	err = json.Unmarshal(body, &metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return metrics, nil
}
