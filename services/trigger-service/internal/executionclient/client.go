package executionclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/YuanJey/crypto-knights/services/trigger-service/internal/trigger"
)

const maxResponseBytes int64 = 1 << 20

type Client struct {
	endpoint   string
	httpClient *http.Client
}

func New(baseURL string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return nil, fmt.Errorf("parse execution service URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("execution service URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("execution service URL requires a host")
	}
	if httpClient == nil {
		return nil, errors.New("HTTP client is required")
	}
	return &Client{
		endpoint:   parsed.String() + "/v1/executions",
		httpClient: httpClient,
	}, nil
}

func (c *Client) Submit(
	ctx context.Context,
	request trigger.ExecutionRequest,
) (trigger.ExecutionReceipt, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return trigger.ExecutionReceipt{}, fmt.Errorf("encode execution request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return trigger.ExecutionReceipt{}, fmt.Errorf("create execution request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return trigger.ExecutionReceipt{}, fmt.Errorf("submit execution: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return trigger.ExecutionReceipt{}, fmt.Errorf("read execution response: %w", err)
	}
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return trigger.ExecutionReceipt{}, fmt.Errorf(
			"execution service returned %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(responseBody)),
		)
	}

	var receipt trigger.ExecutionReceipt
	if err := json.Unmarshal(responseBody, &receipt); err != nil {
		return trigger.ExecutionReceipt{}, fmt.Errorf("decode execution response: %w", err)
	}
	if strings.TrimSpace(receipt.ExecutionID) == "" {
		return trigger.ExecutionReceipt{}, errors.New("execution response is missing execution_id")
	}
	return receipt, nil
}
