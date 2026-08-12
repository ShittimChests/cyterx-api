package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
