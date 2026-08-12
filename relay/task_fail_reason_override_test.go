package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Async tasks persist the upstream failure reason (service/task_polling.go sets
// task.FailReason = taskResult.Reason) and users read it back through the task query
// endpoints. That is a second outlet for the same upstream text the relay response
// override hides, so TaskModel2Dto must mask it for user-facing callers while the admin
// view keeps the original for diagnosis.
func TestTaskModel2Dto_FailReasonOverride(t *testing.T) {
	origEnabled := operation_setting.ErrorOverrideEnabled
	origKeywords := operation_setting.ErrorOverrideKeywords
	t.Cleanup(func() {
		operation_setting.ErrorOverrideEnabled = origEnabled
		operation_setting.ErrorOverrideKeywords = origKeywords
	})
	operation_setting.ErrorOverrideEnabled = true
	operation_setting.ErrorOverrideKeywords = []string{"no available", "quota", "credits", "top-up"}

	upstreamReason := "insufficient credits, please top up your account"

	t.Run("user view masks an upstream reason", func(t *testing.T) {
		task := &model.Task{TaskID: "t-1", FailReason: upstreamReason}
		assert.Equal(t, operation_setting.ErrorOverrideMessage,
			TaskModel2Dto(task, true).FailReason)
	})

	t.Run("admin view keeps the original reason", func(t *testing.T) {
		task := &model.Task{TaskID: "t-1", FailReason: upstreamReason}
		assert.Equal(t, upstreamReason, TaskModel2Dto(task, false).FailReason)
	})

	// Local reasons produced by this site must survive the user-facing path. These are
	// the strings sweepTimedOutTasks and FailTaskInfo actually write.
	t.Run("local reasons are not masked", func(t *testing.T) {
		for _, reason := range []string{
			"任务超时（30分钟）",
			"任务超时（旧系统遗留任务，不进行退款，请联系管理员）",
			"upstream returned error",
			"upstream returned unrecognized message",
		} {
			task := &model.Task{TaskID: "t-2", FailReason: reason}
			assert.Equal(t, reason, TaskModel2Dto(task, true).FailReason, reason)
		}
	})

	t.Run("empty reason stays empty", func(t *testing.T) {
		task := &model.Task{TaskID: "t-3"}
		require.Empty(t, TaskModel2Dto(task, true).FailReason)
	})

	t.Run("disabled override keeps the upstream reason", func(t *testing.T) {
		operation_setting.ErrorOverrideEnabled = false
		defer func() { operation_setting.ErrorOverrideEnabled = true }()

		task := &model.Task{TaskID: "t-4", FailReason: upstreamReason}
		assert.Equal(t, upstreamReason, TaskModel2Dto(task, true).FailReason)
	})
}

// /v1/videos/{id} builds its response from the stored upstream payload through each
// adaptor's ConvertToOpenAIVideo, bypassing TaskModel2Dto entirely. It is therefore a
// third outlet for the same upstream failure text and must be masked at the JSON level:
// the sora converter passes the whole upstream object through, so a struct round-trip
// would silently drop fields the upstream returned.
func TestOverrideOpenAIVideoUpstreamError(t *testing.T) {
	origEnabled := operation_setting.ErrorOverrideEnabled
	origKeywords := operation_setting.ErrorOverrideKeywords
	t.Cleanup(func() {
		operation_setting.ErrorOverrideEnabled = origEnabled
		operation_setting.ErrorOverrideKeywords = origKeywords
	})
	operation_setting.ErrorOverrideEnabled = true
	operation_setting.ErrorOverrideKeywords = []string{"no available", "quota", "credits", "top-up"}

	t.Run("masks the message and keeps every other upstream field", func(t *testing.T) {
		body := []byte(`{"id":"video_1","status":"failed","seconds":"8","error":{"code":"billing_hard_limit_reached","message":"You have insufficient credits, please top-up","upstream_only":"keep me"}}`)

		got := overrideOpenAIVideoUpstreamError(body)

		assert.Equal(t, operation_setting.ErrorOverrideMessage, gjson.GetBytes(got, "error.message").String())
		// Status code equivalents and any extra upstream fields survive the rewrite.
		assert.Equal(t, "billing_hard_limit_reached", gjson.GetBytes(got, "error.code").String())
		assert.Equal(t, "keep me", gjson.GetBytes(got, "error.upstream_only").String())
		assert.Equal(t, "video_1", gjson.GetBytes(got, "id").String())
		assert.Equal(t, "failed", gjson.GetBytes(got, "status").String())
		assert.Equal(t, "8", gjson.GetBytes(got, "seconds").String())
	})

	t.Run("leaves bodies without a matching message byte-identical", func(t *testing.T) {
		for _, body := range []string{
			`{"id":"video_2","status":"completed"}`,
			`{"id":"video_3","status":"failed","error":{"code":"moderation_blocked","message":"content policy violation"}}`,
			`{"id":"video_4","error":null}`,
			`{"id":"video_5","error":"insufficient credits"}`,
			`not json at all, please top-up`,
			``,
		} {
			assert.Equal(t, body, string(overrideOpenAIVideoUpstreamError([]byte(body))), body)
		}
	})

	t.Run("disabled override keeps the upstream message", func(t *testing.T) {
		operation_setting.ErrorOverrideEnabled = false
		defer func() { operation_setting.ErrorOverrideEnabled = true }()

		body := `{"error":{"message":"You have insufficient credits, please top-up"}}`
		assert.Equal(t, body, string(overrideOpenAIVideoUpstreamError([]byte(body))))
	})
}
