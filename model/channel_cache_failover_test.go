package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupFailoverChannelCache 直接构造内存渠道缓存，隔离测试
// GetRandomSatisfiedChannel 的排除逻辑，测试结束后恢复原状态。
func setupFailoverChannelCache(t *testing.T, channels []*Channel, group string, model string, channelIds []int) {
	t.Helper()

	newIDM := make(map[int]*Channel)
	for _, ch := range channels {
		newIDM[ch.Id] = ch
	}
	newG2M := map[string]map[string][]int{
		group: {model: channelIds},
	}

	oldMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true

	channelSyncLock.Lock()
	oldIDM := channelsIDM
	oldG2M := group2model2channels
	oldAdvanced := channel2advancedCustomConfig
	channelsIDM = newIDM
	group2model2channels = newG2M
	channel2advancedCustomConfig = make(map[int]*dto.AdvancedCustomConfig)
	channelSyncLock.Unlock()

	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCache
		channelSyncLock.Lock()
		channelsIDM = oldIDM
		group2model2channels = oldG2M
		channel2advancedCustomConfig = oldAdvanced
		channelSyncLock.Unlock()
	})
}

func newFailoverTestChannel(id int, priority int64, weight uint) *Channel {
	return &Channel{
		Id:       id,
		Status:   common.ChannelStatusEnabled,
		Priority: &priority,
		Weight:   &weight,
	}
}

func TestGetRandomSatisfiedChannelWithoutExclusionKeepsPriorityIndex(t *testing.T) {
	// 保护"开关关闭时零行为变化"契约：排除集为空时 retry 仍是优先级索引。
	channels := []*Channel{
		newFailoverTestChannel(1, 10, 1),
		newFailoverTestChannel(2, 10, 1),
		newFailoverTestChannel(3, 5, 1),
	}
	setupFailoverChannelCache(t, channels, "default", "cinax-pro", []int{1, 2, 3})

	// retry=0 -> 最高优先级组(10)
	ch, err := GetRandomSatisfiedChannel("default", "cinax-pro", 0, "", nil)
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Contains(t, []int{1, 2}, ch.Id)

	// retry=1 -> 次高优先级组(5)
	ch, err = GetRandomSatisfiedChannel("default", "cinax-pro", 1, "", nil)
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, 3, ch.Id)

	// retry 超界 -> clamp 到最低优先级组
	ch, err = GetRandomSatisfiedChannel("default", "cinax-pro", 9, "", nil)
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, 3, ch.Id)
}

func TestGetRandomSatisfiedChannelExcludesFailedChannels(t *testing.T) {
	channels := []*Channel{
		newFailoverTestChannel(1, 10, 1),
		newFailoverTestChannel(2, 5, 1),
	}
	setupFailoverChannelCache(t, channels, "default", "cinax-pro", []int{1, 2})

	// 最高优先级唯一渠道被排除 -> 落到次高优先级渠道
	ch, err := GetRandomSatisfiedChannel("default", "cinax-pro", 0, "", map[int]bool{1: true})
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, 2, ch.Id)
}

func TestGetRandomSatisfiedChannelAllExcludedReturnsNil(t *testing.T) {
	channels := []*Channel{
		newFailoverTestChannel(1, 10, 1),
		newFailoverTestChannel(2, 5, 1),
	}
	setupFailoverChannelCache(t, channels, "default", "cinax-pro", []int{1, 2})

	ch, err := GetRandomSatisfiedChannel("default", "cinax-pro", 0, "", map[int]bool{1: true, 2: true})
	require.NoError(t, err)
	assert.Nil(t, ch)
}

func TestGetRandomSatisfiedChannelExclusionIgnoresRetryIndex(t *testing.T) {
	// exclude 只排除一个最高优先级渠道，retry=1：
	// 若仍按 retry 索引取剔除后的次高优先级组会返回渠道 3；
	// 排除集非空时应忽略索引、从剩余最高优先级组(10)中选取渠道 2。
	channels := []*Channel{
		newFailoverTestChannel(1, 10, 1),
		newFailoverTestChannel(2, 10, 1),
		newFailoverTestChannel(3, 5, 1),
	}
	setupFailoverChannelCache(t, channels, "default", "cinax-pro", []int{1, 2, 3})

	ch, err := GetRandomSatisfiedChannel("default", "cinax-pro", 1, "", map[int]bool{1: true})
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, 2, ch.Id)
}

func TestGetRandomSatisfiedChannelExclusionDoesNotMutateCache(t *testing.T) {
	channels := []*Channel{
		newFailoverTestChannel(1, 10, 1),
		newFailoverTestChannel(2, 10, 1),
		newFailoverTestChannel(3, 5, 1),
	}
	cached := []int{1, 2, 3}
	setupFailoverChannelCache(t, channels, "default", "cinax-pro", cached)

	_, err := GetRandomSatisfiedChannel("default", "cinax-pro", 0, "", map[int]bool{1: true, 2: true})
	require.NoError(t, err)

	channelSyncLock.RLock()
	got := group2model2channels["default"]["cinax-pro"]
	channelSyncLock.RUnlock()
	assert.Equal(t, []int{1, 2, 3}, got, "cached slice must not be mutated by exclusion filtering")

	// 排除后再次不带排除集调用，仍能选到全部候选中的最高优先级渠道
	ch, err := GetRandomSatisfiedChannel("default", "cinax-pro", 0, "", nil)
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Contains(t, []int{1, 2}, ch.Id)
}
