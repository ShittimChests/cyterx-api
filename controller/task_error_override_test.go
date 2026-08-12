package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enableTaskErrorOverride turns the override on with the shipped default keywords and
// restores the previous package state afterwards.
func enableTaskErrorOverride(t *testing.T) {
	t.Helper()
	origEnabled := operation_setting.ErrorOverrideEnabled
	origKeywords := operation_setting.ErrorOverrideKeywords
	t.Cleanup(func() {
		operation_setting.ErrorOverrideEnabled = origEnabled
		operation_setting.ErrorOverrideKeywords = origKeywords
	})
	operation_setting.ErrorOverrideEnabled = true
	operation_setting.ErrorOverrideKeywords = []string{"no available", "quota", "credits", "top-up"}
}

// respondTaskError must key the override off the positive FromUpstream marker. Local
// errors default to LocalError == false, so gating on !LocalError would mask this
// site's own billing and infrastructure failures whenever their text happens to
// contain an override keyword.
func TestRespondTaskError_OverrideGate(t *testing.T) {
	enableTaskErrorOverride(t)
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name        string
		taskErr     *taskdto.TaskError
		wantMessage string
	}{
		{
			// Regression: PreConsumeBilling produces this locally. It contains "quota",
			// so an inverted gate would hide the real reason the request failed.
			name: "local subscription quota error is not overridden",
			taskErr: service.TaskErrorFromAPIError(types.NewErrorWithStatusCode(
				errors.New("订阅额度不足或未配置订阅: subscription quota insufficient"),
				types.ErrorCodeInsufficientUserQuota, http.StatusForbidden)),
			wantMessage: "订阅额度不足或未配置订阅: subscription quota insufficient",
		},
		{
			name: "local user quota error is not overridden",
			taskErr: service.TaskErrorFromAPIError(types.NewErrorWithStatusCode(
				errors.New("用户额度不足, 剩余额度: $0.00"),
				types.ErrorCodeInsufficientUserQuota, http.StatusForbidden)),
			wantMessage: "用户额度不足, 剩余额度: $0.00",
		},
		{
			name: "local no-available-key error is not overridden",
			taskErr: service.TaskErrorWrapper(
				errors.New("no available channel for model under group default"),
				"channel_no_available_key", http.StatusServiceUnavailable),
			wantMessage: "no available channel for model under group default",
		},
		{
			name: "upstream keyword match is overridden",
			taskErr: service.TaskErrorWrapperUpstream(
				errors.New("insufficient credits, please top up your account"),
				"upstream_error", http.StatusPaymentRequired),
			wantMessage: operation_setting.ErrorOverrideMessage,
		},
		{
			name: "upstream without keyword keeps its message",
			taskErr: service.TaskErrorWrapperUpstream(
				errors.New("invalid prompt"), "upstream_error", http.StatusBadRequest),
			wantMessage: "invalid prompt",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)

			respondTaskError(c, tc.taskErr)

			assert.Equal(t, tc.wantMessage, tc.taskErr.Message)
		})
	}
}

// Data carries json:"data" and reaches the client. Overriding Message while leaving a
// field that may hold the upstream payload would defeat the override, so an overridden
// task error must clear it — and a non-overridden one must keep it.
func TestRespondTaskError_ClearsDataOnOverride(t *testing.T) {
	enableTaskErrorOverride(t)
	gin.SetMode(gin.TestMode)

	t.Run("overridden error drops Data", func(t *testing.T) {
		taskErr := service.TaskErrorWrapperUpstream(
			errors.New("insufficient credits, please top up"),
			"upstream_error", http.StatusPaymentRequired)
		taskErr.Data = map[string]string{"raw": "upstream-billing-detail"}

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)

		respondTaskError(c, taskErr)

		require.Equal(t, operation_setting.ErrorOverrideMessage, taskErr.Message)
		assert.Nil(t, taskErr.Data)
	})

	t.Run("untouched error keeps Data", func(t *testing.T) {
		taskErr := service.TaskErrorWrapperUpstream(
			errors.New("invalid prompt"), "upstream_error", http.StatusBadRequest)
		taskErr.Data = map[string]string{"field": "prompt"}

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)

		respondTaskError(c, taskErr)

		require.Equal(t, "invalid prompt", taskErr.Message)
		assert.NotNil(t, taskErr.Data)
	})
}

// A 429 rewrite happens before the override and its text is this site's own, so it
// must survive regardless of the upstream marker.
func TestRespondTaskError_RateLimitMessagePreserved(t *testing.T) {
	enableTaskErrorOverride(t)
	gin.SetMode(gin.TestMode)

	taskErr := service.TaskErrorWrapperUpstream(
		errors.New("upstream quota exhausted"), "upstream_error", http.StatusTooManyRequests)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)

	respondTaskError(c, taskErr)

	require.Equal(t, "当前分组上游负载已饱和，请稍后再试", taskErr.Message)
}
