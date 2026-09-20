package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	MaxTokenManagedGroups            = 20
	UpstreamGroupStateDiscovered     = "discovered"
	UpstreamGroupStateProvisioning   = "provisioning"
	UpstreamGroupStateActive         = "active"
	UpstreamGroupStateAdminDisabled  = "admin_disabled"
	UpstreamGroupStateGroupRemoved   = "group_removed"
	UpstreamGroupStateKeyUnavailable = "key_unavailable"
	UpstreamGroupStateError          = "error"
)

type UpstreamGroupBinding struct {
	Id                  int64  `json:"id"`
	Identity            string `json:"-" gorm:"type:varchar(320);uniqueIndex"`
	UpstreamChannelId   int64  `json:"upstream_channel_id" gorm:"index"`
	UpstreamChannelName string `json:"upstream_channel_name" gorm:"type:varchar(128)"`
	UpstreamChannelType string `json:"upstream_channel_type" gorm:"type:varchar(32)"`
	UpstreamSiteUrl     string `json:"upstream_site_url" gorm:"type:varchar(512)"`
	RemoteGroupId       *int64 `json:"remote_group_id,omitempty"`
	RemoteGroupName     string `json:"remote_group_name" gorm:"type:varchar(256)"`
	RemoteDescription   string `json:"remote_description" gorm:"type:varchar(512)"`
	LocalGroup          string `json:"local_group" gorm:"type:varchar(64);uniqueIndex"`
	LocalDisplayName    string `json:"local_display_name" gorm:"type:varchar(384)"`
	LocalChannelId      *int   `json:"local_channel_id,omitempty" gorm:"index"`
	UpstreamKeyId       *int64 `json:"upstream_key_id,omitempty"`
	DesiredEnabled      bool   `json:"desired_enabled"`
	State               string `json:"state" gorm:"type:varchar(32);index"`
	SourceRatio         string `json:"source_ratio" gorm:"type:varchar(64)"`
	SaleRatio           string `json:"sale_ratio" gorm:"type:varchar(64)"`
	PriceVersion        int64  `json:"price_version" gorm:"default:0"`
	DisabledReason      string `json:"disabled_reason" gorm:"type:varchar(512)"`
	LastSeenAt          int64  `json:"last_seen_at" gorm:"bigint"`
	LastSyncedAt        int64  `json:"last_synced_at" gorm:"bigint"`
	CreatedTime         int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime         int64  `json:"updated_time" gorm:"bigint"`
}

type TokenGroupBinding struct {
	Id                   int64  `json:"id"`
	TokenId              int    `json:"token_id" gorm:"uniqueIndex:idx_token_group_binding;index:idx_token_group_position,priority:1"`
	BindingId            int64  `json:"binding_id" gorm:"uniqueIndex:idx_token_group_binding;index"`
	Position             int    `json:"position" gorm:"index:idx_token_group_position,priority:2"`
	Enabled              bool   `json:"enabled"`
	AcceptedPriceVersion int64  `json:"accepted_price_version"`
	AcceptedSourceRatio  string `json:"accepted_source_ratio" gorm:"type:varchar(64)"`
	AcceptedSaleRatio    string `json:"accepted_sale_ratio" gorm:"type:varchar(64)"`
	CreatedTime          int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime          int64  `json:"updated_time" gorm:"bigint"`
}

type UpstreamGroupEvent struct {
	Id               int64  `json:"id"`
	BindingId        int64  `json:"binding_id" gorm:"index"`
	EventType        string `json:"event_type" gorm:"type:varchar(32);index"`
	OldState         string `json:"old_state" gorm:"type:varchar(32)"`
	NewState         string `json:"new_state" gorm:"type:varchar(32)"`
	OldRatio         string `json:"old_ratio" gorm:"type:varchar(64)"`
	NewRatio         string `json:"new_ratio" gorm:"type:varchar(64)"`
	OldPriceVersion  int64  `json:"old_price_version"`
	NewPriceVersion  int64  `json:"new_price_version"`
	AffectedKeyCount int64  `json:"affected_key_count"`
	Message          string `json:"message" gorm:"type:text"`
	CreatedTime      int64  `json:"created_time" gorm:"bigint;index"`
}

type TokenGroupBindingView struct {
	BindingId            int64  `json:"binding_id"`
	LocalGroup           string `json:"local_group"`
	DisplayName          string `json:"display_name"`
	Position             int    `json:"position"`
	Enabled              bool   `json:"enabled"`
	EffectiveEnabled     bool   `json:"effective_enabled"`
	State                string `json:"state"`
	AcceptedPriceVersion int64  `json:"accepted_price_version"`
	CurrentPriceVersion  int64  `json:"current_price_version"`
	AcceptedSourceRatio  string `json:"accepted_source_ratio"`
	AcceptedSaleRatio    string `json:"accepted_sale_ratio"`
	CurrentSourceRatio   string `json:"current_source_ratio"`
	CurrentSaleRatio     string `json:"current_sale_ratio"`
	LocalChannelId       *int   `json:"local_channel_id,omitempty"`
}

func (binding *UpstreamGroupBinding) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if binding.CreatedTime == 0 {
		binding.CreatedTime = now
	}
	if binding.UpdatedTime == 0 {
		binding.UpdatedTime = now
	}
	if binding.State == "" {
		binding.State = UpstreamGroupStateDiscovered
	}
	return nil
}

func (binding *UpstreamGroupBinding) BeforeUpdate(_ *gorm.DB) error {
	binding.UpdatedTime = common.GetTimestamp()
	return nil
}

func (binding *TokenGroupBinding) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if binding.CreatedTime == 0 {
		binding.CreatedTime = now
	}
	if binding.UpdatedTime == 0 {
		binding.UpdatedTime = now
	}
	return nil
}

func (binding *TokenGroupBinding) BeforeUpdate(_ *gorm.DB) error {
	binding.UpdatedTime = common.GetTimestamp()
	return nil
}

func (event *UpstreamGroupEvent) BeforeCreate(_ *gorm.DB) error {
	if event.CreatedTime == 0 {
		event.CreatedTime = common.GetTimestamp()
	}
	return nil
}

func GetUpstreamGroupBinding(id int64) (*UpstreamGroupBinding, error) {
	var binding UpstreamGroupBinding
	if err := DB.First(&binding, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func GetUpstreamGroupBindingByIdentity(identity string) (*UpstreamGroupBinding, error) {
	var binding UpstreamGroupBinding
	if err := DB.Where("identity = ?", identity).First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func ListUpstreamGroupBindings() ([]UpstreamGroupBinding, error) {
	var bindings []UpstreamGroupBinding
	err := DB.Order("upstream_channel_id ASC, remote_group_name ASC").Find(&bindings).Error
	return bindings, err
}

func ListActiveUpstreamGroupBindings() ([]UpstreamGroupBinding, error) {
	var bindings []UpstreamGroupBinding
	err := DB.Where("desired_enabled = ? AND state = ?", true, UpstreamGroupStateActive).
		Order("upstream_channel_id ASC, remote_group_name ASC").Find(&bindings).Error
	return bindings, err
}

func CountAffectedTokenGroups(bindingIds []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64, len(bindingIds))
	if len(bindingIds) == 0 {
		return counts, nil
	}
	var rows []struct {
		BindingId int64
		Count     int64
	}
	err := DB.Table("token_group_bindings").
		Select("token_group_bindings.binding_id, COUNT(*) AS count").
		Joins("JOIN upstream_group_bindings ON upstream_group_bindings.id = token_group_bindings.binding_id").
		Where("token_group_bindings.binding_id IN ? AND token_group_bindings.enabled = ? AND token_group_bindings.accepted_price_version != upstream_group_bindings.price_version", bindingIds, true).
		Group("token_group_bindings.binding_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.BindingId] = row.Count
	}
	return counts, nil
}

func ListTokenGroupBindingViews(tokenId int) ([]TokenGroupBindingView, error) {
	groupsByToken, err := ListTokenGroupBindingViewsBatch([]int{tokenId})
	return groupsByToken[tokenId], err
}

func ListTokenGroupBindingViewsBatch(tokenIds []int) (map[int][]TokenGroupBindingView, error) {
	viewsByToken := make(map[int][]TokenGroupBindingView, len(tokenIds))
	if len(tokenIds) == 0 {
		return viewsByToken, nil
	}
	var rows []struct {
		TokenGroupBinding
		LocalGroup       string
		LocalDisplayName string
		GlobalState      string
		CurrentVersion   int64
		CurrentSource    string
		CurrentSale      string
		LocalChannelId   *int
		DesiredEnabled   bool
	}
	err := DB.Table("token_group_bindings").
		Select("token_group_bindings.*, upstream_group_bindings.local_group, upstream_group_bindings.local_display_name, upstream_group_bindings.state AS global_state, upstream_group_bindings.price_version AS current_version, upstream_group_bindings.source_ratio AS current_source, upstream_group_bindings.sale_ratio AS current_sale, upstream_group_bindings.local_channel_id, upstream_group_bindings.desired_enabled").
		Joins("JOIN upstream_group_bindings ON upstream_group_bindings.id = token_group_bindings.binding_id").
		Where("token_group_bindings.token_id IN ?", tokenIds).
		Order("token_group_bindings.token_id ASC, token_group_bindings.position ASC, token_group_bindings.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		state := "active"
		effective := row.Enabled && row.DesiredEnabled && row.GlobalState == UpstreamGroupStateActive && row.AcceptedPriceVersion == row.CurrentVersion
		switch {
		case !row.Enabled:
			state = "user_disabled"
		case !row.DesiredEnabled || row.GlobalState != UpstreamGroupStateActive:
			state = "group_unavailable"
		case row.AcceptedPriceVersion != row.CurrentVersion:
			state = "price_changed"
		}
		viewsByToken[row.TokenId] = append(viewsByToken[row.TokenId], TokenGroupBindingView{
			BindingId:            row.BindingId,
			LocalGroup:           row.LocalGroup,
			DisplayName:          row.LocalDisplayName,
			Position:             row.Position,
			Enabled:              row.Enabled,
			EffectiveEnabled:     effective,
			State:                state,
			AcceptedPriceVersion: row.AcceptedPriceVersion,
			CurrentPriceVersion:  row.CurrentVersion,
			AcceptedSourceRatio:  row.AcceptedSourceRatio,
			AcceptedSaleRatio:    row.AcceptedSaleRatio,
			CurrentSourceRatio:   row.CurrentSource,
			CurrentSaleRatio:     row.CurrentSale,
			LocalChannelId:       row.LocalChannelId,
		})
	}
	return viewsByToken, nil
}

func TokenHasGroupBindings(tokenId int) (bool, error) {
	var count int64
	err := DB.Model(&TokenGroupBinding{}).Where("token_id = ?", tokenId).Limit(1).Count(&count).Error
	return count > 0, err
}

func ReplaceTokenGroupBindings(tx *gorm.DB, tokenId int, bindingIds []int64) error {
	if tx == nil {
		return errors.New("transaction is required")
	}
	if len(bindingIds) == 0 {
		return tx.Where("token_id = ?", tokenId).Delete(&TokenGroupBinding{}).Error
	}
	if len(bindingIds) > MaxTokenManagedGroups {
		return fmt.Errorf("token groups must not exceed %d", MaxTokenManagedGroups)
	}
	seen := make(map[int64]struct{}, len(bindingIds))
	for _, bindingId := range bindingIds {
		if _, ok := seen[bindingId]; ok {
			return errors.New("token groups must not contain duplicates")
		}
		seen[bindingId] = struct{}{}
	}

	var existing []TokenGroupBinding
	if err := tx.Where("token_id = ?", tokenId).Find(&existing).Error; err != nil {
		return err
	}
	existingByBindingId := make(map[int64]TokenGroupBinding, len(existing))
	for _, relation := range existing {
		existingByBindingId[relation.BindingId] = relation
	}

	var groups []UpstreamGroupBinding
	if err := tx.Where("id IN ?", bindingIds).Find(&groups).Error; err != nil {
		return err
	}
	byId := make(map[int64]UpstreamGroupBinding, len(groups))
	for _, group := range groups {
		byId[group.Id] = group
	}
	newBindings := make([]TokenGroupBinding, 0, len(bindingIds))
	for position, bindingId := range bindingIds {
		group, ok := byId[bindingId]
		if !ok {
			return errors.New("token group is unavailable")
		}
		if relation, exists := existingByBindingId[bindingId]; exists {
			if err := tx.Model(&TokenGroupBinding{}).Where("id = ?", relation.Id).Updates(map[string]any{
				"position":     position,
				"updated_time": common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
			continue
		}
		if !group.DesiredEnabled || group.State != UpstreamGroupStateActive {
			return errors.New("token group is unavailable")
		}
		newBindings = append(newBindings, TokenGroupBinding{
			TokenId:              tokenId,
			BindingId:            bindingId,
			Position:             position,
			Enabled:              true,
			AcceptedPriceVersion: group.PriceVersion,
			AcceptedSourceRatio:  group.SourceRatio,
			AcceptedSaleRatio:    group.SaleRatio,
		})
	}
	if err := tx.Where("token_id = ? AND binding_id NOT IN ?", tokenId, bindingIds).Delete(&TokenGroupBinding{}).Error; err != nil {
		return err
	}
	if len(newBindings) == 0 {
		return nil
	}
	return tx.Create(&newBindings).Error
}

func RecordUpstreamChannelSyncTransition(channelId int64, failed bool, message string) (bool, error) {
	transitioned := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var binding UpstreamGroupBinding
		if err := tx.Where("upstream_channel_id = ?", channelId).Order("id ASC").First(&binding).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var latest UpstreamGroupEvent
		err := tx.Where("binding_id = ? AND event_type IN ?", binding.Id, []string{"sync_failed", "sync_recovered"}).
			Order("id DESC").First(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		eventType := "sync_recovered"
		if failed {
			eventType = "sync_failed"
		}
		if errors.Is(err, gorm.ErrRecordNotFound) && !failed {
			return nil
		}
		if err == nil && latest.EventType == eventType {
			return nil
		}
		transitioned = true
		return tx.Create(&UpstreamGroupEvent{
			BindingId: binding.Id,
			EventType: eventType,
			Message:   message,
		}).Error
	})
	return transitioned, err
}

func CreateTokenWithGroupBindings(token *Token, bindingIds []int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(token).Error; err != nil {
			return err
		}
		return ReplaceTokenGroupBindings(tx, token.Id, bindingIds)
	})
}

func UpdateTokenWithGroupBindings(token *Token, bindingIds []int64, updateGroups bool) error {
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(token).Select("name", "status", "expired_time", "remain_quota", "unlimited_quota",
			"model_limits_enabled", "model_limits", "allow_ips", "group", "cross_group_retry", "auto_groups").Updates(token).Error; err != nil {
			return err
		}
		if !updateGroups {
			return nil
		}
		return ReplaceTokenGroupBindings(tx, token.Id, bindingIds)
	})
	if shouldUpdateRedis(true, err) {
		if cacheErr := cacheSetToken(*token); cacheErr != nil {
			_ = cacheDeleteToken(token.Key)
		}
	}
	return err
}

func DeleteTokenGroupBindings(tx *gorm.DB, tokenIds []int) error {
	if len(tokenIds) == 0 {
		return nil
	}
	return tx.Where("token_id IN ?", tokenIds).Delete(&TokenGroupBinding{}).Error
}

func SetTokenGroupEnabled(tokenId int, userId int, bindingId int64, enabled bool, expectedVersion int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var token Token
		if err := tx.Select("id").Where("id = ? AND user_id = ?", tokenId, userId).First(&token).Error; err != nil {
			return err
		}
		var relation TokenGroupBinding
		if err := lockForUpdate(tx).Where("token_id = ? AND binding_id = ?", tokenId, bindingId).First(&relation).Error; err != nil {
			return err
		}
		updates := map[string]any{"enabled": enabled, "updated_time": common.GetTimestamp()}
		if enabled {
			var group UpstreamGroupBinding
			if err := lockForUpdate(tx).Where("id = ? AND desired_enabled = ? AND state = ?", bindingId, true, UpstreamGroupStateActive).First(&group).Error; err != nil {
				return err
			}
			if expectedVersion != group.PriceVersion {
				return ErrUpstreamGroupPriceChanged
			}
			updates["accepted_price_version"] = group.PriceVersion
			updates["accepted_source_ratio"] = group.SourceRatio
			updates["accepted_sale_ratio"] = group.SaleRatio
		}
		return tx.Model(&TokenGroupBinding{}).Where("id = ?", relation.Id).Updates(updates).Error
	})
}

var ErrUpstreamGroupPriceChanged = errors.New("upstream group price version changed")

type UpstreamGroupObservation struct {
	Identity            string
	UpstreamChannelId   int64
	UpstreamChannelName string
	UpstreamChannelType string
	UpstreamSiteUrl     string
	RemoteGroupId       *int64
	RemoteGroupName     string
	RemoteDescription   string
	LocalGroup          string
	LocalDisplayName    string
	SourceRatio         string
	SaleRatio           string
	ObservedAt          int64
}

func ApplyUpstreamGroupObservation(observation UpstreamGroupObservation) (*UpstreamGroupBinding, bool, bool, error) {
	var result UpstreamGroupBinding
	created := false
	priceChanged := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where("identity = ?", observation.Identity).First(&result).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created = true
			result = UpstreamGroupBinding{
				Identity:            observation.Identity,
				UpstreamChannelId:   observation.UpstreamChannelId,
				UpstreamChannelName: observation.UpstreamChannelName,
				UpstreamChannelType: observation.UpstreamChannelType,
				UpstreamSiteUrl:     observation.UpstreamSiteUrl,
				RemoteGroupId:       observation.RemoteGroupId,
				RemoteGroupName:     observation.RemoteGroupName,
				RemoteDescription:   observation.RemoteDescription,
				LocalGroup:          observation.LocalGroup,
				LocalDisplayName:    observation.LocalDisplayName,
				State:               UpstreamGroupStateDiscovered,
				SourceRatio:         observation.SourceRatio,
				SaleRatio:           observation.SaleRatio,
				PriceVersion:        1,
				LastSeenAt:          observation.ObservedAt,
				LastSyncedAt:        observation.ObservedAt,
			}
			return tx.Create(&result).Error
		}
		if err != nil {
			return err
		}

		oldRatio := result.SourceRatio
		oldVersion := result.PriceVersion
		if oldRatio != "" {
			oldDecimal, oldErr := decimal.NewFromString(oldRatio)
			newDecimal, newErr := decimal.NewFromString(observation.SourceRatio)
			if oldErr != nil || newErr != nil {
				return errors.New("invalid upstream group ratio")
			}
			priceChanged = !oldDecimal.Equal(newDecimal)
		}
		updates := map[string]any{
			"upstream_channel_name": observation.UpstreamChannelName,
			"upstream_channel_type": observation.UpstreamChannelType,
			"upstream_site_url":     observation.UpstreamSiteUrl,
			"remote_group_id":       observation.RemoteGroupId,
			"remote_group_name":     observation.RemoteGroupName,
			"remote_description":    observation.RemoteDescription,
			"local_display_name":    observation.LocalDisplayName,
			"source_ratio":          observation.SourceRatio,
			"sale_ratio":            observation.SaleRatio,
			"last_seen_at":          observation.ObservedAt,
			"last_synced_at":        observation.ObservedAt,
			"updated_time":          observation.ObservedAt,
		}
		if priceChanged {
			updates["price_version"] = gorm.Expr("price_version + ?", 1)
		}
		if result.State == UpstreamGroupStateGroupRemoved {
			updates["state"] = UpstreamGroupStateDiscovered
			updates["desired_enabled"] = false
			updates["disabled_reason"] = ""
		}
		if err := tx.Model(&UpstreamGroupBinding{}).Where("id = ?", result.Id).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.First(&result, "id = ?", result.Id).Error; err != nil {
			return err
		}
		if !priceChanged {
			return nil
		}
		var affected int64
		if err := tx.Model(&TokenGroupBinding{}).
			Where("binding_id = ? AND enabled = ? AND accepted_price_version != ?", result.Id, true, result.PriceVersion).
			Count(&affected).Error; err != nil {
			return err
		}
		return tx.Create(&UpstreamGroupEvent{
			BindingId:        result.Id,
			EventType:        "price_changed",
			OldState:         result.State,
			NewState:         result.State,
			OldRatio:         oldRatio,
			NewRatio:         result.SourceRatio,
			OldPriceVersion:  oldVersion,
			NewPriceVersion:  result.PriceVersion,
			AffectedKeyCount: affected,
			Message:          fmt.Sprintf("price version changed from %d to %d", oldVersion, result.PriceVersion),
		}).Error
	})
	if err != nil {
		return nil, false, false, err
	}
	return &result, created, priceChanged, nil
}

func MarkMissingUpstreamGroups(channelId int64, seenIdentities []string) ([]UpstreamGroupBinding, error) {
	query := DB.Where("upstream_channel_id = ? AND desired_enabled = ? AND state = ?", channelId, true, UpstreamGroupStateActive)
	if len(seenIdentities) > 0 {
		query = query.Where("identity NOT IN ?", seenIdentities)
	}
	var missing []UpstreamGroupBinding
	if err := query.Find(&missing).Error; err != nil {
		return nil, err
	}
	for i := range missing {
		if err := DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&UpstreamGroupBinding{}).Where("id = ?", missing[i].Id).Updates(map[string]any{
				"state":           UpstreamGroupStateGroupRemoved,
				"disabled_reason": "remote group was removed",
				"updated_time":    common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
			if missing[i].LocalChannelId != nil {
				if err := tx.Model(&Channel{}).Where("id = ?", *missing[i].LocalChannelId).Update("status", common.ChannelStatusManuallyDisabled).Error; err != nil {
					return err
				}
				if err := tx.Model(&Ability{}).Where("channel_id = ?", *missing[i].LocalChannelId).Update("enabled", false).Error; err != nil {
					return err
				}
			}
			return tx.Create(&UpstreamGroupEvent{BindingId: missing[i].Id, EventType: "group_removed", OldState: missing[i].State, NewState: UpstreamGroupStateGroupRemoved}).Error
		}); err != nil {
			return nil, err
		}
		missing[i].State = UpstreamGroupStateGroupRemoved
	}
	return missing, nil
}

func ListUpstreamGroupEvents(bindingId int64, limit int) ([]UpstreamGroupEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var events []UpstreamGroupEvent
	err := DB.Where("binding_id = ?", bindingId).Order("id DESC").Limit(limit).Find(&events).Error
	return events, err
}
