package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const upstreamOpsMaxResponseBytes = 4 << 20

type UpstreamOpsClient struct {
	baseUrl string
	token   string
	http    *http.Client
}

type UpstreamOpsChannel struct {
	Id             int64  `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	SiteUrl        string `json:"site_url"`
	MonitorEnabled bool   `json:"monitor_enabled"`
}

type UpstreamOpsRate struct {
	Id              int64       `json:"id"`
	ChannelId       int64       `json:"channel_id"`
	RemoteGroupId   *int64      `json:"remote_group_id"`
	ModelName       string      `json:"model_name"`
	Description     string      `json:"description"`
	Ratio           json.Number `json:"ratio"`
	CompletionRatio json.Number `json:"completion_ratio"`
}

type UpstreamOpsKeyGroup struct {
	Id          *int64      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Ratio       json.Number `json:"ratio"`
}

type UpstreamOpsKey struct {
	Id                 int64    `json:"id"`
	Name               string   `json:"name"`
	Status             string   `json:"status"`
	Group              string   `json:"group"`
	GroupName          string   `json:"group_name"`
	GroupId            *int64   `json:"group_id"`
	ExpiredTime        int64    `json:"expired_time"`
	ExpiresAt          *string  `json:"expires_at"`
	AllowIps           string   `json:"allow_ips"`
	IpWhitelist        []string `json:"ip_whitelist"`
	IpBlacklist        []string `json:"ip_blacklist"`
	ModelLimitsEnabled bool     `json:"model_limits_enabled"`
}

type UpstreamOpsKeyPage struct {
	Items    []UpstreamOpsKey `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Pages    int              `json:"pages"`
}

type UpstreamOpsCreateKeyRequest struct {
	Name               string `json:"name"`
	Group              string `json:"group,omitempty"`
	GroupId            *int64 `json:"group_id,omitempty"`
	UnlimitedQuota     *bool  `json:"unlimited_quota,omitempty"`
	ExpiredTime        *int64 `json:"expired_time,omitempty"`
	ModelLimitsEnabled *bool  `json:"model_limits_enabled,omitempty"`
}

func NewUpstreamOpsClientFromEnv() (*UpstreamOpsClient, error) {
	baseUrl := strings.TrimRight(strings.TrimSpace(common.GetEnvOrDefaultString("UPSTREAM_OPS_BASE_URL", "")), "/")
	token := strings.TrimSpace(common.GetEnvOrDefaultString("UPSTREAM_OPS_TOKEN", ""))
	if baseUrl == "" || token == "" {
		return nil, errors.New("UpstreamOps is not configured")
	}
	parsed, err := url.Parse(baseUrl)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, errors.New("invalid UpstreamOps base URL")
	}
	return &UpstreamOpsClient{
		baseUrl: baseUrl,
		token:   token,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second,
			},
		},
	}, nil
}

func (client *UpstreamOpsClient) request(ctx context.Context, method string, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := common.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, client.baseUrl+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.http.Do(req)
	if err != nil {
		return fmt.Errorf("UpstreamOps request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, upstreamOpsMaxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("UpstreamOps response read failed: %w", err)
	}
	if len(data) > upstreamOpsMaxResponseBytes {
		return errors.New("UpstreamOps response is too large")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("UpstreamOps returned HTTP %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := common.Unmarshal(data, out); err != nil {
		return fmt.Errorf("UpstreamOps response decode failed: %w", err)
	}
	return nil
}

func (client *UpstreamOpsClient) ListChannels(ctx context.Context) ([]UpstreamOpsChannel, error) {
	var response struct {
		Data struct {
			Items []UpstreamOpsChannel `json:"items"`
		} `json:"data"`
	}
	err := client.request(ctx, http.MethodGet, "/api/channels?page=1&page_size=-1", nil, &response)
	return response.Data.Items, err
}

func (client *UpstreamOpsClient) RefreshRates(ctx context.Context, channelId int64) error {
	var response struct {
		Ok bool `json:"ok"`
	}
	if err := client.request(ctx, http.MethodPost, "/api/channels/"+strconv.FormatInt(channelId, 10)+"/refresh-rates", nil, &response); err != nil {
		return err
	}
	if !response.Ok {
		return errors.New("UpstreamOps rate refresh failed")
	}
	return nil
}

func (client *UpstreamOpsClient) ListRates(ctx context.Context, channelId int64) ([]UpstreamOpsRate, error) {
	var response struct {
		Data []UpstreamOpsRate `json:"data"`
	}
	err := client.request(ctx, http.MethodGet, "/api/channels/"+strconv.FormatInt(channelId, 10)+"/rates", nil, &response)
	return response.Data, err
}

func (client *UpstreamOpsClient) ListKeyGroups(ctx context.Context, channelId int64) ([]UpstreamOpsKeyGroup, error) {
	var response struct {
		Data []UpstreamOpsKeyGroup `json:"data"`
	}
	err := client.request(ctx, http.MethodGet, "/api/channels/"+strconv.FormatInt(channelId, 10)+"/api-keys/groups", nil, &response)
	return response.Data, err
}

func (client *UpstreamOpsClient) ListKeys(ctx context.Context, channelId int64, page int) (*UpstreamOpsKeyPage, error) {
	var response struct {
		Data UpstreamOpsKeyPage `json:"data"`
	}
	path := "/api/channels/" + strconv.FormatInt(channelId, 10) + "/api-keys?page=" + strconv.Itoa(page) + "&page_size=100"
	err := client.request(ctx, http.MethodGet, path, nil, &response)
	return &response.Data, err
}

func (client *UpstreamOpsClient) CreateKey(ctx context.Context, channelId int64, input UpstreamOpsCreateKeyRequest) (*UpstreamOpsKey, error) {
	var response struct {
		Data UpstreamOpsKey `json:"data"`
	}
	err := client.request(ctx, http.MethodPost, "/api/channels/"+strconv.FormatInt(channelId, 10)+"/api-keys", input, &response)
	return &response.Data, err
}

func (client *UpstreamOpsClient) RevealKey(ctx context.Context, channelId int64, keyId int64) (string, error) {
	var response struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	path := "/api/channels/" + strconv.FormatInt(channelId, 10) + "/api-keys/" + strconv.FormatInt(keyId, 10) + "/reveal"
	if err := client.request(ctx, http.MethodPost, path, nil, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Data.Key) == "" {
		return "", errors.New("UpstreamOps did not return the complete key")
	}
	return response.Data.Key, nil
}
