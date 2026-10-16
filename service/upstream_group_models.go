package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// upstreamGroupModelRefreshInterval bounds how often one managed channel's model
// list is re-read from its upstream. A model set changes far less often than a
// price does, and each refresh costs one HTTP call per binding, so this runs
// slower than the per-minute rate sync.
const upstreamGroupModelRefreshInterval = 5 * time.Minute

// upstreamGroupModelRefreshBudget bounds one whole refresh pass so a slow
// upstream cannot hold the sync task open indefinitely.
const upstreamGroupModelRefreshBudget = 60 * time.Second

// managedChannelModelRefresh remembers when each binding's models were last
// re-read. The timestamp is written before the attempt, so a failing upstream is
// retried on the same cadence as a healthy one instead of being hammered.
var managedChannelModelRefresh = struct {
	mu   sync.Mutex
	last map[int64]time.Time
}{last: make(map[int64]time.Time)}

// syncManagedChannelModels re-reads the upstream model list behind every active
// managed binding and rewrites the mirrored channel when the set drifted. This
// is what stops a retired upstream model from staying in the advertised list
// (and being routed to, then answered with an upstream 404) until an operator
// re-enables the group by hand. It returns the number of channels rewritten and
// human-readable notices for the operator.
func syncManagedChannelModels(parent context.Context) (int, []string) {
	bindings, err := model.ListActiveUpstreamGroupBindings()
	if err != nil {
		logger.LogWarn(parent, "managed channel model refresh skipped: "+err.Error())
		return 0, nil
	}
	now := time.Now()
	due := make([]model.UpstreamGroupBinding, 0, len(bindings))
	managedChannelModelRefresh.mu.Lock()
	for _, binding := range bindings {
		if binding.LocalChannelId == nil {
			continue
		}
		if last, ok := managedChannelModelRefresh.last[binding.Id]; ok && now.Sub(last) < upstreamGroupModelRefreshInterval {
			continue
		}
		managedChannelModelRefresh.last[binding.Id] = now
		due = append(due, binding)
	}
	managedChannelModelRefresh.mu.Unlock()
	if len(due) == 0 {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(parent, upstreamGroupModelRefreshBudget)
	defer cancel()
	type modelRefreshOutcome struct {
		binding model.UpstreamGroupBinding
		added   []string
		removed []string
	}
	outcomes := make(chan modelRefreshOutcome, len(due))
	semaphore := make(chan struct{}, 4)
	var waitGroup sync.WaitGroup
	for _, binding := range due {
		waitGroup.Add(1)
		go func(binding model.UpstreamGroupBinding) {
			defer waitGroup.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			changed, added, removed, refreshErr := refreshManagedChannelModels(ctx, binding)
			if refreshErr != nil {
				logger.LogWarn(ctx, fmt.Sprintf("managed channel %s model refresh failed: %s", binding.LocalGroup, refreshErr.Error()))
				return
			}
			if !changed {
				return
			}
			outcomes <- modelRefreshOutcome{binding: binding, added: added, removed: removed}
		}(binding)
	}
	waitGroup.Wait()
	close(outcomes)

	refreshed := 0
	notices := make([]string, 0)
	for outcome := range outcomes {
		refreshed++
		addedText := strings.Join(outcome.added, ",")
		if addedText == "" {
			addedText = "-"
		}
		removedText := strings.Join(outcome.removed, ",")
		if removedText == "" {
			removedText = "-"
		}
		notices = append(notices, fmt.Sprintf("%s / %s: models updated (added %s, removed %s)",
			outcome.binding.UpstreamChannelName, outcome.binding.RemoteGroupName, addedText, removedText))
	}
	if refreshed > 0 {
		model.InitChannelCache()
	}
	return refreshed, notices
}

// refreshManagedChannelModels re-reads one binding's upstream model list and
// rewrites its mirrored channel when the set drifted. It reports whether the
// channel was rewritten, plus the added and removed model names.
func refreshManagedChannelModels(ctx context.Context, binding model.UpstreamGroupBinding) (bool, []string, []string, error) {
	if binding.LocalChannelId == nil {
		return false, nil, nil, nil
	}
	channel, err := model.GetChannelById(*binding.LocalChannelId, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// The mirrored channel is gone; re-enabling the group recreates it.
			return false, nil, nil, nil
		}
		return false, nil, nil, err
	}
	if channel.Tag == nil || *channel.Tag != upstreamGroupManagedTag {
		return false, nil, nil, nil
	}
	models, err := fetchManagedUpstreamModels(ctx, binding.UpstreamSiteUrl, channel.Key)
	if err != nil {
		return false, nil, nil, err
	}
	next, added, removed, changed := managedChannelModelDiff(channel.Models, models)
	if !changed {
		return false, nil, nil, nil
	}
	channel.Models = next
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		update := tx.Model(&model.Channel{}).Where("id = ? AND tag = ?", channel.Id, upstreamGroupManagedTag).
			Update("models", next)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			return nil
		}
		if err := tx.Where("channel_id = ?", channel.Id).Delete(&model.Ability{}).Error; err != nil {
			return err
		}
		return channel.AddAbilities(tx)
	})
	if err != nil {
		return false, nil, nil, err
	}
	logger.LogInfo(ctx, fmt.Sprintf("managed channel %d (%s) models updated: added [%s] removed [%s]",
		channel.Id, binding.LocalGroup, strings.Join(added, ","), strings.Join(removed, ",")))
	return true, added, removed, nil
}

// managedChannelModelDiff compares a mirrored channel's stored model list with
// the upstream listing. Both sides are comma-joined and order-insensitive. An
// empty upstream listing never clears the channel: a hiccup must not take every
// model of a managed group offline, so the caller keeps the stored list.
func managedChannelModelDiff(current string, upstream []string) (string, []string, []string, bool) {
	currentSet := make(map[string]struct{})
	for _, name := range strings.Split(current, ",") {
		if name = strings.TrimSpace(name); name != "" {
			currentSet[name] = struct{}{}
		}
	}
	upstreamSet := make(map[string]struct{}, len(upstream))
	for _, name := range upstream {
		if name = strings.TrimSpace(name); name != "" {
			upstreamSet[name] = struct{}{}
		}
	}
	if len(upstreamSet) == 0 {
		return current, nil, nil, false
	}
	added := make([]string, 0, len(upstreamSet))
	removed := make([]string, 0, len(currentSet))
	merged := make([]string, 0, len(upstreamSet))
	for name := range upstreamSet {
		merged = append(merged, name)
		if _, ok := currentSet[name]; !ok {
			added = append(added, name)
		}
	}
	for name := range currentSet {
		if _, ok := upstreamSet[name]; !ok {
			removed = append(removed, name)
		}
	}
	if len(added) == 0 && len(removed) == 0 {
		return current, nil, nil, false
	}
	sort.Strings(merged)
	sort.Strings(added)
	sort.Strings(removed)
	return strings.Join(merged, ","), added, removed, true
}
