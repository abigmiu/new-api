package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

type UpstreamGroupSyncSummary struct {
	Channels     int `json:"channels"`
	Groups       int `json:"groups"`
	Discovered   int `json:"discovered"`
	PriceChanged int `json:"price_changed"`
	Removed      int `json:"removed"`
	Failed       int `json:"failed"`
}

type upstreamGroupChannelSnapshot struct {
	Channel UpstreamOpsChannel
	Rates   []UpstreamOpsRate
	Err     error
}

func upstreamGroupIdentity(channel UpstreamOpsChannel, rate UpstreamOpsRate) (string, string) {
	if rate.RemoteGroupId != nil {
		identity := fmt.Sprintf("%s:%d:%d", channel.Type, channel.Id, *rate.RemoteGroupId)
		return identity, fmt.Sprintf("uo-%d-%d", channel.Id, *rate.RemoteGroupId)
	}
	hash := sha256.Sum256([]byte(rate.ModelName))
	identity := fmt.Sprintf("%s:%d:%s", channel.Type, channel.Id, rate.ModelName)
	return identity, fmt.Sprintf("uo-%d-%x", channel.Id, hash[:6])
}

func RunUpstreamGroupSync(parent context.Context) (UpstreamGroupSyncSummary, error) {
	summary := UpstreamGroupSyncSummary{}
	client, err := NewUpstreamOpsClientFromEnv()
	if err != nil {
		return summary, err
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	channels, err := client.ListChannels(ctx)
	if err != nil {
		return summary, err
	}
	notices := make([]string, 0)
	results := make(chan upstreamGroupChannelSnapshot, len(channels))
	semaphore := make(chan struct{}, 4)
	var waitGroup sync.WaitGroup
	for _, channel := range channels {
		if !channel.MonitorEnabled {
			continue
		}
		summary.Channels++
		waitGroup.Add(1)
		go func(channel UpstreamOpsChannel) {
			defer waitGroup.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results <- upstreamGroupChannelSnapshot{Channel: channel, Err: ctx.Err()}
				return
			}
			if refreshErr := client.RefreshRates(ctx, channel.Id); refreshErr != nil {
				results <- upstreamGroupChannelSnapshot{Channel: channel, Err: refreshErr}
				return
			}
			rates, listErr := client.ListRates(ctx, channel.Id)
			results <- upstreamGroupChannelSnapshot{Channel: channel, Rates: rates, Err: listErr}
		}(channel)
	}
	waitGroup.Wait()
	close(results)

	for snapshot := range results {
		channel := snapshot.Channel
		if snapshot.Err != nil {
			summary.Failed++
			transitioned, recordErr := model.RecordUpstreamChannelSyncTransition(channel.Id, true, snapshot.Err.Error())
			if recordErr != nil {
				summary.Failed++
			} else if transitioned {
				notices = append(notices, fmt.Sprintf("%s: synchronization failed (%s)", channel.Name, snapshot.Err.Error()))
			}
			continue
		}
		rates := snapshot.Rates
		seen := make([]string, 0, len(rates))
		for _, rate := range rates {
			source, parseErr := decimal.NewFromString(rate.Ratio.String())
			if parseErr != nil {
				summary.Failed++
				continue
			}
			sale, priceErr := CalculateUpstreamGroupSaleRatio(source.String())
			if priceErr != nil {
				summary.Failed++
				continue
			}
			identity, localGroup := upstreamGroupIdentity(channel, rate)
			seen = append(seen, identity)
			before, _ := model.GetUpstreamGroupBindingByIdentity(identity)
			binding, created, changed, applyErr := model.ApplyUpstreamGroupObservation(model.UpstreamGroupObservation{
				Identity:            identity,
				UpstreamChannelId:   channel.Id,
				UpstreamChannelName: channel.Name,
				UpstreamChannelType: channel.Type,
				UpstreamSiteUrl:     channel.SiteUrl,
				RemoteGroupId:       rate.RemoteGroupId,
				RemoteGroupName:     rate.ModelName,
				RemoteDescription:   rate.Description,
				LocalGroup:          localGroup,
				LocalDisplayName:    fmt.Sprintf("渠道%d-%s", channel.Id, rate.ModelName),
				SourceRatio:         source.String(),
				SaleRatio:           sale,
				ObservedAt:          common.GetTimestamp(),
			})
			if applyErr != nil {
				summary.Failed++
				continue
			}
			summary.Groups++
			if created {
				summary.Discovered++
			}
			if changed {
				summary.PriceChanged++
				oldRatio := ""
				oldSaleRatio := ""
				if before != nil {
					oldRatio = before.SourceRatio
					oldSaleRatio = before.SaleRatio
				}
				affected, countErr := model.CountAffectedTokenGroups([]int64{binding.Id})
				if countErr != nil {
					summary.Failed++
				}
				notices = append(notices, fmt.Sprintf(
					"%s / %s: source %s -> %s, sale %s -> %s, version %d -> %d, affected keys %d; old-price key bindings cannot route to this group",
					channel.Name, rate.ModelName, oldRatio, source.String(), oldSaleRatio, sale, binding.PriceVersion-1, binding.PriceVersion, affected[binding.Id],
				))
			}
		}
		missing, markErr := model.MarkMissingUpstreamGroups(channel.Id, seen)
		if markErr != nil {
			summary.Failed++
			continue
		}
		summary.Removed += len(missing)
		for _, binding := range missing {
			notices = append(notices, fmt.Sprintf("%s / %s: remote group removed; all key bindings for this group are unavailable", channel.Name, binding.RemoteGroupName))
		}
		transitioned, recordErr := model.RecordUpstreamChannelSyncTransition(channel.Id, false, "synchronization recovered")
		if recordErr != nil {
			summary.Failed++
		} else if transitioned {
			notices = append(notices, fmt.Sprintf("%s: synchronization recovered", channel.Name))
		}
	}
	if summary.Removed > 0 {
		model.InitChannelCache()
	}
	if len(notices) > 0 {
		SendUpstreamGroupNotice(ctx, notices)
	}
	return summary, nil
}

func SendUpstreamGroupNotice(ctx context.Context, notices []string) {
	webhook := strings.TrimSpace(common.GetEnvOrDefaultString("UPSTREAM_GROUP_FEISHU_WEBHOOK", ""))
	if webhook == "" {
		return
	}
	lines := append([]string{"Upstream group events:"}, notices...)
	payload, err := common.Marshal(map[string]any{"msg_type": "text", "content": map[string]string{"text": strings.Join(lines, "\n")}})
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

func UpstreamGroupSyncConfigured() bool {
	return strings.TrimSpace(common.GetEnvOrDefaultString("UPSTREAM_OPS_BASE_URL", "")) != "" &&
		strings.TrimSpace(common.GetEnvOrDefaultString("UPSTREAM_OPS_TOKEN", "")) != ""
}

func ParseUpstreamGroupId(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
