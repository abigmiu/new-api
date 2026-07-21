package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMidjourneyUserLogsHideChannelID(t *testing.T) {
	originalDB := DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() {
		DB = originalDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, db.AutoMigrate(&Midjourney{}))
	task := Midjourney{UserId: 1, ChannelId: 42, MjId: "task-1"}
	require.NoError(t, db.Create(&task).Error)

	userTasks := GetAllUserTask(1, 0, 10, TaskQueryParams{})
	require.Len(t, userTasks, 1)
	assert.Zero(t, userTasks[0].ChannelId)

	adminTasks := GetAllTasks(0, 10, TaskQueryParams{})
	require.Len(t, adminTasks, 1)
	assert.Equal(t, 42, adminTasks[0].ChannelId)
}
