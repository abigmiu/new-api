package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUpstreamGroupTestDB(t *testing.T) {
	t.Helper()
	originalDB := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Token{}, &UpstreamGroupBinding{}, &TokenGroupBinding{}, &UpstreamGroupEvent{}))
	DB = db
	t.Cleanup(func() { DB = originalDB })
}

func TestTokenGroupBindingBecomesPriceChangedWithoutRowUpdate(t *testing.T) {
	setupUpstreamGroupTestDB(t)
	group := UpstreamGroupBinding{
		Identity:         "newapi:1:pro",
		LocalGroup:       "uo-1-pro",
		LocalDisplayName: "channel1-pro",
		DesiredEnabled:   true,
		State:            UpstreamGroupStateActive,
		SourceRatio:      "0.075",
		SaleRatio:        "0.089",
		PriceVersion:     1,
	}
	require.NoError(t, DB.Create(&group).Error)
	token := Token{UserId: 1, Key: "price-version-token", Name: "test", Status: common.TokenStatusEnabled}
	require.NoError(t, DB.Create(&token).Error)
	require.NoError(t, ReplaceTokenGroupBindings(DB, token.Id, []int64{group.Id}))

	require.NoError(t, DB.Model(&UpstreamGroupBinding{}).Where("id = ?", group.Id).Updates(map[string]any{
		"source_ratio":  "0.076",
		"sale_ratio":    "0.090",
		"price_version": 2,
	}).Error)

	views, err := ListTokenGroupBindingViews(token.Id)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, "price_changed", views[0].State)
	assert.False(t, views[0].EffectiveEnabled)
	assert.Equal(t, int64(1), views[0].AcceptedPriceVersion)
	assert.Equal(t, int64(2), views[0].CurrentPriceVersion)

	var relation TokenGroupBinding
	require.NoError(t, DB.First(&relation, "token_id = ?", token.Id).Error)
	assert.True(t, relation.Enabled)
	assert.Equal(t, int64(1), relation.AcceptedPriceVersion)
}

func TestListActiveUpstreamGroupBindingsOnlyReturnsGloballyEnabledActiveGroups(t *testing.T) {
	setupUpstreamGroupTestDB(t)
	localChannelID := 1
	groups := []UpstreamGroupBinding{
		{Identity: "newapi:1:active", LocalGroup: "uo-1-active", DesiredEnabled: true, State: UpstreamGroupStateActive, LocalChannelId: &localChannelID},
		{Identity: "newapi:1:disabled", LocalGroup: "uo-1-disabled", DesiredEnabled: false, State: UpstreamGroupStateActive, LocalChannelId: &localChannelID},
		{Identity: "newapi:1:error", LocalGroup: "uo-1-error", DesiredEnabled: true, State: UpstreamGroupStateError, LocalChannelId: &localChannelID},
	}
	require.NoError(t, DB.Create(&groups).Error)

	active, err := ListActiveUpstreamGroupBindings()

	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, "uo-1-active", active[0].LocalGroup)
}

func TestAcceptPriceOnlyUpdatesTargetTokenGroup(t *testing.T) {
	setupUpstreamGroupTestDB(t)
	group := UpstreamGroupBinding{
		Identity:         "newapi:1:pro",
		LocalGroup:       "uo-1-pro",
		LocalDisplayName: "channel1-pro",
		DesiredEnabled:   true,
		State:            UpstreamGroupStateActive,
		SourceRatio:      "0.076",
		SaleRatio:        "0.090",
		PriceVersion:     2,
	}
	require.NoError(t, DB.Create(&group).Error)
	tokens := []Token{
		{UserId: 1, Key: "token-one", Name: "one", Status: common.TokenStatusEnabled},
		{UserId: 2, Key: "token-two", Name: "two", Status: common.TokenStatusEnabled},
	}
	require.NoError(t, DB.Create(&tokens).Error)
	for _, token := range tokens {
		require.NoError(t, DB.Create(&TokenGroupBinding{
			TokenId:              token.Id,
			BindingId:            group.Id,
			Enabled:              true,
			AcceptedPriceVersion: 1,
			AcceptedSourceRatio:  "0.075",
			AcceptedSaleRatio:    "0.089",
		}).Error)
	}

	require.NoError(t, SetTokenGroupEnabled(tokens[0].Id, tokens[0].UserId, group.Id, true, 2))

	first, err := ListTokenGroupBindingViews(tokens[0].Id)
	require.NoError(t, err)
	second, err := ListTokenGroupBindingViews(tokens[1].Id)
	require.NoError(t, err)
	assert.True(t, first[0].EffectiveEnabled)
	assert.Equal(t, "price_changed", second[0].State)
}

func TestApplyUpstreamGroupObservationTreatsEquivalentDecimalsAsSamePrice(t *testing.T) {
	setupUpstreamGroupTestDB(t)
	group := UpstreamGroupBinding{
		Identity:       "newapi:1:pro",
		LocalGroup:     "uo-1-pro",
		SourceRatio:    "0.10",
		SaleRatio:      "0.118",
		PriceVersion:   1,
		LastSeenAt:     1,
		LastSyncedAt:   1,
		DesiredEnabled: true,
		State:          UpstreamGroupStateActive,
	}
	require.NoError(t, DB.Create(&group).Error)

	updated, _, changed, err := ApplyUpstreamGroupObservation(UpstreamGroupObservation{
		Identity:    group.Identity,
		LocalGroup:  group.LocalGroup,
		SourceRatio: "0.1",
		SaleRatio:   "0.118",
		ObservedAt:  2,
	})
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, int64(1), updated.PriceVersion)
}

func TestBatchDeleteTokensDeletesOnlyOwnedGroupBindings(t *testing.T) {
	setupUpstreamGroupTestDB(t)
	group := UpstreamGroupBinding{Identity: "newapi:1:pro", LocalGroup: "uo-1-pro"}
	require.NoError(t, DB.Create(&group).Error)
	tokens := []Token{
		{UserId: 1, Key: "owned-token", Name: "owned"},
		{UserId: 2, Key: "other-token", Name: "other"},
	}
	require.NoError(t, DB.Create(&tokens).Error)
	for _, token := range tokens {
		require.NoError(t, DB.Create(&TokenGroupBinding{TokenId: token.Id, BindingId: group.Id}).Error)
	}

	count, err := BatchDeleteTokens([]int{tokens[0].Id, tokens[1].Id}, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	var relations []TokenGroupBinding
	require.NoError(t, DB.Find(&relations).Error)
	require.Len(t, relations, 1)
	assert.Equal(t, tokens[1].Id, relations[0].TokenId)
}

func TestReplaceTokenGroupBindingsPreservesAcceptedPriceAndEnabledState(t *testing.T) {
	setupUpstreamGroupTestDB(t)
	groups := []UpstreamGroupBinding{
		{Identity: "newapi:1:a", LocalGroup: "uo-1-a", DesiredEnabled: true, State: UpstreamGroupStateActive, SourceRatio: "1", SaleRatio: "1.180", PriceVersion: 2},
		{Identity: "newapi:1:b", LocalGroup: "uo-1-b", DesiredEnabled: true, State: UpstreamGroupStateActive, SourceRatio: "2", SaleRatio: "2.360", PriceVersion: 1},
	}
	require.NoError(t, DB.Create(&groups).Error)
	token := Token{UserId: 1, Key: "update-token", Name: "update"}
	require.NoError(t, DB.Create(&token).Error)
	relation := TokenGroupBinding{
		TokenId:              token.Id,
		BindingId:            groups[0].Id,
		Position:             0,
		Enabled:              false,
		AcceptedPriceVersion: 1,
		AcceptedSourceRatio:  "0.9",
		AcceptedSaleRatio:    "1.062",
	}
	require.NoError(t, DB.Create(&relation).Error)

	require.NoError(t, ReplaceTokenGroupBindings(DB, token.Id, []int64{groups[1].Id, groups[0].Id}))

	var relations []TokenGroupBinding
	require.NoError(t, DB.Where("token_id = ?", token.Id).Order("position ASC").Find(&relations).Error)
	require.Len(t, relations, 2)
	assert.Equal(t, groups[1].Id, relations[0].BindingId)
	assert.Equal(t, int64(1), relations[0].AcceptedPriceVersion)
	assert.Equal(t, groups[0].Id, relations[1].BindingId)
	assert.False(t, relations[1].Enabled)
	assert.Equal(t, int64(1), relations[1].AcceptedPriceVersion)
	assert.Equal(t, "1.062", relations[1].AcceptedSaleRatio)
}

func TestReplaceTokenGroupBindingsRejectsTooManyGroups(t *testing.T) {
	setupUpstreamGroupTestDB(t)
	token := Token{UserId: 1, Name: "limited", Key: "limited-key"}
	require.NoError(t, DB.Create(&token).Error)

	bindingIds := make([]int64, 0, MaxTokenManagedGroups+1)
	for i := 0; i <= MaxTokenManagedGroups; i++ {
		group := UpstreamGroupBinding{
			Identity:       fmt.Sprintf("newapi:limit:%d", i),
			LocalGroup:     fmt.Sprintf("uo-limit-%d", i),
			DesiredEnabled: true,
			State:          UpstreamGroupStateActive,
			SourceRatio:    "1",
			SaleRatio:      "1.18",
			PriceVersion:   1,
		}
		require.NoError(t, DB.Create(&group).Error)
		bindingIds = append(bindingIds, group.Id)
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		return ReplaceTokenGroupBindings(tx, token.Id, bindingIds)
	})
	require.ErrorContains(t, err, "must not exceed")

	var count int64
	require.NoError(t, DB.Model(&TokenGroupBinding{}).Where("token_id = ?", token.Id).Count(&count).Error)
	assert.Zero(t, count)
}

func TestRecordUpstreamChannelSyncTransitionOnlyRecordsChanges(t *testing.T) {
	setupUpstreamGroupTestDB(t)
	group := UpstreamGroupBinding{Identity: "newapi:sync:a", UpstreamChannelId: 9, LocalGroup: "uo-sync-a"}
	require.NoError(t, DB.Create(&group).Error)

	transitioned, err := RecordUpstreamChannelSyncTransition(9, false, "healthy")
	require.NoError(t, err)
	assert.False(t, transitioned)

	transitioned, err = RecordUpstreamChannelSyncTransition(9, true, "timeout")
	require.NoError(t, err)
	assert.True(t, transitioned)
	transitioned, err = RecordUpstreamChannelSyncTransition(9, true, "timeout again")
	require.NoError(t, err)
	assert.False(t, transitioned)
	transitioned, err = RecordUpstreamChannelSyncTransition(9, false, "recovered")
	require.NoError(t, err)
	assert.True(t, transitioned)

	var events []UpstreamGroupEvent
	require.NoError(t, DB.Where("binding_id = ?", group.Id).Order("id ASC").Find(&events).Error)
	require.Len(t, events, 2)
	assert.Equal(t, "sync_failed", events[0].EventType)
	assert.Equal(t, "sync_recovered", events[1].EventType)
}
