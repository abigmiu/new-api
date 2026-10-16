package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTokenAuthManagedGroupTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.UpstreamGroupBinding{}, &model.TokenGroupBinding{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	model.ReloadCommonColumns()
	require.NoError(t, i18n.Init())
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
	})
	return db
}

// runTokenAuth binds a token to a managed group and drives TokenAuth over
// /v1/chat/completions, returning the HTTP status and the error code the key
// owner would receive.
func runTokenAuth(t *testing.T, db *gorm.DB, group model.UpstreamGroupBinding, acceptedPriceVersion int64) (int, string, string) {
	t.Helper()
	require.NoError(t, db.Create(&group).Error)
	user := model.User{Username: "managed-user", Status: common.UserStatusEnabled, Group: "default", Quota: 1000000}
	require.NoError(t, db.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "managedkey", Name: "managed", Status: common.TokenStatusEnabled, UnlimitedQuota: true}
	require.NoError(t, db.Create(&token).Error)
	require.NoError(t, db.Create(&model.TokenGroupBinding{
		TokenId:              token.Id,
		BindingId:            group.Id,
		Enabled:              true,
		AcceptedPriceVersion: acceptedPriceVersion,
		AcceptedSourceRatio:  "1",
		AcceptedSaleRatio:    "1.18",
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Authorization", "Bearer sk-managedkey")

	TokenAuth()(ctx)

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return recorder.Code, body.Error.Code, body.Error.Message
}

func newManagedGroupBinding(id int64, state string) model.UpstreamGroupBinding {
	return model.UpstreamGroupBinding{
		Id:                id,
		Identity:          "newapi:1:pro",
		UpstreamChannelId: 1,
		LocalGroup:        "uo-1-pro",
		DesiredEnabled:    true,
		State:             state,
		SourceRatio:       "1",
		SaleRatio:         "1.18",
		PriceVersion:      2,
	}
}

// A key bound to a channel whose upstream price moved must be rejected with the
// dedicated reason, and the message must not reveal the operator's channel key.
func TestTokenAuthRejectsStaleManagedGroupPrice(t *testing.T) {
	db := setupTokenAuthManagedGroupTest(t)
	group := newManagedGroupBinding(11, model.UpstreamGroupStateActive)

	status, code, message := runTokenAuth(t, db, group, group.PriceVersion-1)

	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "token_group_price_changed", code)
	assert.NotContains(t, message, "uo-")
	assert.NotContains(t, message, "http")
}

func TestTokenAuthReportsRemovedManagedGroup(t *testing.T) {
	db := setupTokenAuthManagedGroupTest(t)
	group := newManagedGroupBinding(12, model.UpstreamGroupStateGroupRemoved)

	status, code, message := runTokenAuth(t, db, group, group.PriceVersion)

	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "token_group_removed", code)
	assert.NotContains(t, message, "uo-")
}

func TestTokenAuthReportsManagedGroupUnavailable(t *testing.T) {
	db := setupTokenAuthManagedGroupTest(t)
	group := newManagedGroupBinding(13, model.UpstreamGroupStateAdminDisabled)

	status, code, message := runTokenAuth(t, db, group, group.PriceVersion)

	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "token_group_unavailable", code)
	assert.NotContains(t, message, "uo-")
}

func TestTokenAuthAdmitsCurrentPriceManagedGroup(t *testing.T) {
	db := setupTokenAuthManagedGroupTest(t)
	group := newManagedGroupBinding(14, model.UpstreamGroupStateActive)

	require.NoError(t, db.Create(&group).Error)
	user := model.User{Username: "managed-user", Status: common.UserStatusEnabled, Group: "default", Quota: 1000000}
	require.NoError(t, db.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "managedkey", Name: "managed", Status: common.TokenStatusEnabled, UnlimitedQuota: true}
	require.NoError(t, db.Create(&token).Error)
	require.NoError(t, db.Create(&model.TokenGroupBinding{
		TokenId: token.Id, BindingId: group.Id, Enabled: true,
		AcceptedPriceVersion: group.PriceVersion,
		AcceptedSourceRatio:  group.SourceRatio,
		AcceptedSaleRatio:    group.SaleRatio,
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Authorization", "Bearer sk-managedkey")

	TokenAuth()(ctx)

	assert.NotEqual(t, http.StatusForbidden, recorder.Code)
	assert.NotEqual(t, http.StatusServiceUnavailable, recorder.Code)
	assert.False(t, ctx.IsAborted())
}
