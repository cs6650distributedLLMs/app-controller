package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// SummarizerClient is the client for the text summarizer service
type SummarizerClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewSummarizerClient creates a new text summarizer client
func NewSummarizerClient() *SummarizerClient {
	return &SummarizerClient{
		BaseURL: os.Getenv("TEXT_SUMMARIZER_SERVICE_URL"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SummarizeRequest represents a request to summarize text
type SummarizeRequest struct {
	DocumentID string `json:"document_id"`
	Text       string `json:"text"`
}

// SummarizeResponse represents a response from the text summarizer service
type SummarizeResponse struct {
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
}

// SummarizeText sends a request to summarize text
func (c *SummarizerClient) SummarizeText(ctx context.Context, documentID, text string) (*SummarizeResponse, error) {
	// Create the request to the text summarizer service
	url := fmt.Sprintf("%s/summarize", c.BaseURL)

	// Prepare the request payload
	reqBody, err := json.Marshal(SummarizeRequest{
		DocumentID: documentID,
		Text:       text,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("summarizer service returned status %d: %s", resp.StatusCode, respBody)
	}

	// Parse the response
	var summarizeResp SummarizeResponse
	if err := json.Unmarshal(respBody, &summarizeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &summarizeResp, nil
}

// GetSummaryStatus checks the status of a document summarization
func (c *SummarizerClient) GetSummaryStatus(ctx context.Context, documentID string) (*SummarizeResponse, error) {
	// Create a new HTTP request to the check-status endpoint
	url := fmt.Sprintf("%s/check-status/%s", c.BaseURL, documentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send the request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("summarizer service returned status %d: %s", resp.StatusCode, respBody)
	}

	// Parse the response
	var statusResp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Map the status to our format
	summarizeResp := &SummarizeResponse{
		Status: statusResp.Status,
	}

	// If the status is "completed", get the result
	if statusResp.Status == "completed" {
		result, err := c.GetSummaryResult(ctx, documentID)
		if err != nil {
			return nil, err
		}
		summarizeResp.Status = "COMPLETE"
		summarizeResp.Result = result
	} else if statusResp.Status == "processing" {
		summarizeResp.Status = "PROCESSING"
	} else if statusResp.Status == "error" {
		summarizeResp.Status = "ERROR"
	}

	return summarizeResp, nil
}

// GetSummaryResult retrieves the summarization result
func (c *SummarizerClient) GetSummaryResult(ctx context.Context, documentID string) (string, error) {
	// Create a new HTTP request to the result endpoint
	url := fmt.Sprintf("%s/result/%s", c.BaseURL, documentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Send the request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("summarizer service returned status %d: %s", resp.StatusCode, respBody)
	}

	// Parse the response with updated structure
	var resultResp struct {
		DocumentID string `json:"document_id"`
		Status     string `json:"status"`
		Summary    string `json:"summary"`
	}
	if err := json.Unmarshal(respBody, &resultResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Return the summary field instead of result
	return resultResp.Summary, nil
}

// GetSummary gets the summary for a document
func (c *SummarizerClient) GetSummary(ctx context.Context, documentID string) (string, error) {
	resp, err := c.GetSummaryStatus(ctx, documentID)
	if err != nil {
		return "", err
	}

	if resp.Status != "COMPLETE" {
		return "", fmt.Errorf("summary not ready yet, current status: %s", resp.Status)
	}

	return resp.Result, nil
}
