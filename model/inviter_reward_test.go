package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProcessInviterRewardCreditsAffiliateQuotaExactlyOnce(t *testing.T) {
	truncateTables(t)
	previousConfig := common.GetInviterRewardConfig()
	common.SetInviterRewardConfig(common.InviterRewardConfig{Type: "percentage", Value: 10})
	t.Cleanup(func() {
		common.SetInviterRewardConfig(previousConfig)
	})

	inviter := User{Id: 9001, Username: "reward-inviter", Password: "password", AffCode: "reward-inviter-code", Status: common.UserStatusEnabled}
	invitee := User{Id: 9002, Username: "reward-invitee", Password: "password", AffCode: "reward-invitee-code", Status: common.UserStatusEnabled, InviterId: inviter.Id}
	require.NoError(t, DB.Create(&inviter).Error)
	require.NoError(t, DB.Create(&invitee).Error)

	topUp := TopUp{UserId: invitee.Id, TradeNo: "inviter-reward-order", Status: common.TopUpStatusPending}
	require.NoError(t, DB.Create(&topUp).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := processInviterReward(tx, invitee.Id, 500, &topUp)
		return err
	}))
	var persistedTopUp TopUp
	require.NoError(t, DB.First(&persistedTopUp, topUp.Id).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := processInviterReward(tx, invitee.Id, 500, &persistedTopUp)
		return err
	}))

	var updatedInviter User
	var updatedTopUp TopUp
	require.NoError(t, DB.First(&updatedInviter, inviter.Id).Error)
	require.NoError(t, DB.First(&updatedTopUp, topUp.Id).Error)
	assert.Equal(t, 50, updatedInviter.AffQuota)
	assert.Equal(t, 50, updatedInviter.AffHistoryQuota)
	assert.True(t, updatedTopUp.InviterRewardSent)
}

func TestProcessInviterRewardSkipsOverflowWithoutFailingTopUp(t *testing.T) {
	truncateTables(t)
	previousConfig := common.GetInviterRewardConfig()
	common.SetInviterRewardConfig(common.InviterRewardConfig{Type: "fixed", Value: 1})
	t.Cleanup(func() {
		common.SetInviterRewardConfig(previousConfig)
	})

	inviter := User{Id: 9011, Username: "full-reward-inviter", Password: "password", AffCode: "full-reward-inviter-code", Status: common.UserStatusEnabled, AffQuota: common.MaxQuota, AffHistoryQuota: common.MaxQuota}
	invitee := User{Id: 9012, Username: "full-reward-invitee", Password: "password", AffCode: "full-reward-invitee-code", Status: common.UserStatusEnabled, InviterId: inviter.Id}
	require.NoError(t, DB.Create(&inviter).Error)
	require.NoError(t, DB.Create(&invitee).Error)

	topUp := TopUp{UserId: invitee.Id, TradeNo: "full-inviter-reward-order", Status: common.TopUpStatusPending}
	require.NoError(t, DB.Create(&topUp).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		reward, err := processInviterReward(tx, invitee.Id, 500, &topUp)
		require.NoError(t, err)
		assert.Nil(t, reward)
		return nil
	}))

	var updatedTopUp TopUp
	require.NoError(t, DB.First(&updatedTopUp, topUp.Id).Error)
	assert.True(t, updatedTopUp.InviterRewardSent)
}
