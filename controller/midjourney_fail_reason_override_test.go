package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The Midjourney task list is a user-facing outlet for the same upstream failure text
// the /mj/task fetch path already masks (relay.coverMidjourneyTaskDto). The user list
// must mask it while the admin list keeps the original for diagnosis.
func TestMidjourneyListFailReasonOverride(t *testing.T) {
	enableTaskErrorOverride(t)
	gin.SetMode(gin.TestMode)

	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Midjourney{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	const userID = 7
	const upstreamReason = "insufficient credits, please top-up your account"
	const localReason = "获取渠道信息失败，请联系管理员，渠道ID：3"

	require.NoError(t, db.Create(&model.Midjourney{
		UserId: userID, MjId: "mj-upstream", Status: "FAILURE", FailReason: upstreamReason,
	}).Error)
	require.NoError(t, db.Create(&model.Midjourney{
		UserId: userID, MjId: "mj-local", Status: "FAILURE", FailReason: localReason,
	}).Error)

	failReasons := func(handler gin.HandlerFunc) map[string]string {
		t.Helper()
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/mj/self", nil)
		c.Set("id", userID)

		handler(c)
		require.Equal(t, http.StatusOK, recorder.Code)

		var response struct {
			Data struct {
				Items []struct {
					MjId       string `json:"mj_id"`
					FailReason string `json:"fail_reason"`
				} `json:"items"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		reasons := make(map[string]string, len(response.Data.Items))
		for _, item := range response.Data.Items {
			reasons[item.MjId] = item.FailReason
		}
		return reasons
	}

	userReasons := failReasons(GetUserMidjourney)
	assert.Equal(t, operation_setting.ErrorOverrideMessage, userReasons["mj-upstream"])
	// This site's own failure text must survive the user-facing path.
	assert.Equal(t, localReason, userReasons["mj-local"])

	adminReasons := failReasons(GetAllMidjourney)
	assert.Equal(t, upstreamReason, adminReasons["mj-upstream"])
	assert.Equal(t, localReason, adminReasons["mj-local"])
}
