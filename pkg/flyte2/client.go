package flyte2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const defaultTimeout = 20 * time.Second

type Client struct {
	baseURL *url.URL
	apiKey  string
	http    *http.Client
}

type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("Flyte2 request failed with status %d", e.Status)
	}
	return e.Message
}

type Envelope[T any] struct {
	Status  int    `json:"status"`
	Data    T      `json:"data"`
	Message string `json:"message"`
}

type ResourceDefinition struct {
	CPU             string `json:"cpu"`
	Memory          string `json:"memory"`
	GPU             uint32 `json:"gpu"`
	GPUResourceKey  string `json:"gpuResourceKey,omitempty"`
	GPUNodeLabelKey string `json:"gpuNodeLabelKey,omitempty"`
}

type CodeSource struct {
	ID              string `json:"id"`
	Branch          string `json:"branch,omitempty"`
	Path            string `json:"path,omitempty"`
	Token           string `json:"token,omitempty"`
	TokenConfigured bool   `json:"tokenConfigured,omitempty"`
}

type CreateModelRequest struct {
	Name               string             `json:"name"`
	ID                 string             `json:"id"`
	Code               string             `json:"code"`
	Profile            string             `json:"profile,omitempty"`
	Image              string             `json:"image,omitempty"`
	Param              string             `json:"param,omitempty"`
	Project            string             `json:"project"`
	Domain             string             `json:"domain"`
	ResourceDefinition ResourceDefinition `json:"resourceDefinition"`
	ModelCacheSize     string             `json:"modelCacheSize"`
	Codes              []CodeSource       `json:"codes,omitempty"`
}

type UpdateModelRequest struct {
	Name               string             `json:"name"`
	Image              string             `json:"image,omitempty"`
	Param              string             `json:"param,omitempty"`
	ResourceDefinition ResourceDefinition `json:"resourceDefinition"`
	ModelCacheSize     string             `json:"modelCacheSize"`
}

type ModelSummary struct {
	ID               string `json:"id"`
	Org              string `json:"org"`
	Project          string `json:"project"`
	Domain           string `json:"domain"`
	Name             string `json:"name"`
	Code             string `json:"code"`
	Type             string `json:"type"`
	Image            string `json:"image"`
	DeploymentStatus int    `json:"deploymentStatus"`
	Substate         int    `json:"substate"`
	Message          string `json:"message"`
	CurrentReplicas  uint32 `json:"currentReplicas"`
	URL              string `json:"url"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type ModelCachePVC struct {
	Name             string `json:"name"`
	StorageClassName string `json:"storageClassName"`
	RequestedSize    string `json:"requestedSize"`
	Capacity         string `json:"capacity"`
	Expandable       bool   `json:"expandable"`
}

type ModelConfig struct {
	Name               string             `json:"name"`
	Code               string             `json:"code"`
	Image              string             `json:"image"`
	Param              string             `json:"param"`
	Codes              []CodeSource       `json:"codes"`
	ResourceDefinition ResourceDefinition `json:"resourceDefinition"`
	ModelCachePVC      *ModelCachePVC     `json:"modelCachePvc"`
}

type ModelDetail struct {
	ModelSummary
	DesiredState int         `json:"desiredState"`
	Config       ModelConfig `json:"config"`
}

type ModelList struct {
	Items    []ModelSummary `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

type LogLine struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

type LogPage struct {
	Items []LogLine `json:"items"`
	Total int       `json:"total"`
	Page  int       `json:"page"`
	Size  int       `json:"size"`
}

func NewClient(baseURL, apiKey string) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" {
		return nil, errors.New("invalid Flyte2 base URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("Flyte2 base URL must use HTTP or HTTPS")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("Flyte2 base URL must not contain credentials, query, or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.EscapedPath(), "/")
	if !strings.HasSuffix(parsed.Path, "/v2") {
		return nil, errors.New("Flyte2 base URL must end with /v2")
	}
	parsed.RawPath = ""
	return &Client{
		baseURL: parsed,
		apiKey:  strings.TrimSpace(apiKey),
		http:    &http.Client{Timeout: defaultTimeout},
	}, nil
}

func (c *Client) ListModels(ctx context.Context, project, domain, keyword, status string, page, pageSize int) (*ModelList, error) {
	query := url.Values{
		"project":   {project},
		"domain":    {domain},
		"p":         {strconv.Itoa(page)},
		"page_size": {strconv.Itoa(pageSize)},
	}
	if keyword != "" {
		query.Set("keyword", keyword)
	}
	if status != "" {
		query.Set("status", status)
	}
	return request[ModelList](c, ctx, http.MethodGet, "/api/aione/models?"+query.Encode(), nil)
}

func (c *Client) CreateModel(ctx context.Context, requestBody CreateModelRequest) (*ModelSummary, error) {
	return request[ModelSummary](c, ctx, http.MethodPost, "/api/aione/model/run", requestBody)
}

func (c *Client) GetModel(ctx context.Context, id, project, domain string) (*ModelDetail, error) {
	return request[ModelDetail](c, ctx, http.MethodGet, c.modelPath(id, project, domain), nil)
}

func (c *Client) UpdateModel(ctx context.Context, id, project, domain string, requestBody UpdateModelRequest) (*ModelSummary, error) {
	return request[ModelSummary](c, ctx, http.MethodPut, c.modelPath(id, project, domain), requestBody)
}

func (c *Client) StartModel(ctx context.Context, id, project, domain string) (*ModelSummary, error) {
	return request[ModelSummary](c, ctx, http.MethodPost, c.modelActionPath(id, "start", project, domain), nil)
}

func (c *Client) StopModel(ctx context.Context, id, project, domain string) (*ModelSummary, error) {
	return request[ModelSummary](c, ctx, http.MethodPost, c.modelActionPath(id, "stop", project, domain), nil)
}

func (c *Client) DeleteModel(ctx context.Context, id, project, domain string) error {
	_, err := request[map[string]any](c, ctx, http.MethodDelete, c.modelPath(id, project, domain), nil)
	return err
}

func (c *Client) GetModelLogs(ctx context.Context, id, project, domain string, page, size int) (*LogPage, error) {
	path := "/api/aione/model/" + url.PathEscape(id) + "/log"
	query := url.Values{
		"project": {project},
		"domain":  {domain},
		"page":    {strconv.Itoa(page)},
		"size":    {strconv.Itoa(size)},
	}
	return request[LogPage](c, ctx, http.MethodGet, path+"?"+query.Encode(), nil)
}

func (c *Client) modelPath(id, project, domain string) string {
	query := url.Values{"project": {project}, "domain": {domain}}
	return "/api/aione/model/" + url.PathEscape(id) + "?" + query.Encode()
}

func (c *Client) modelActionPath(id, action, project, domain string) string {
	query := url.Values{"project": {project}, "domain": {domain}}
	return "/api/aione/model/" + url.PathEscape(id) + "/" + action + "?" + query.Encode()
}

func request[T any](c *Client, ctx context.Context, method, path string, body any) (*T, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := common.Marshal(body)
		if err != nil {
			return nil, errors.New("failed to encode Flyte2 request")
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL.String()+path, reader)
	if err != nil {
		return nil, errors.New("failed to create Flyte2 request")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("Flyte2 is unavailable")
	}
	defer response.Body.Close()
	var envelope Envelope[T]
	if err := common.DecodeJson(io.LimitReader(response.Body, 4<<20), &envelope); err != nil {
		return nil, errors.New("invalid response from Flyte2")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || envelope.Status >= 400 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return nil, &APIError{Status: response.StatusCode, Message: message}
	}
	return &envelope.Data, nil
}
