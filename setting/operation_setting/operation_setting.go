package operation_setting

import "strings"

var DemoSiteEnabled = false
var SelfUseModeEnabled = false

// ChannelFailoverEnabled 渠道故障转移：开启后请求失败时自动转移到同分组、
// 提供同一模型的其他渠道（排除已失败渠道，每个渠道只尝试一次），
// 不依赖 RetryTimes 设置。默认关闭。
var ChannelFailoverEnabled = false

var AutomaticDisableKeywords = []string{
	"Your credit balance is too low",
	"This organization has been disabled.",
	"You exceeded your current quota",
	"Permission denied",
	"The security token included in the request is invalid",
	"Operation not allowed",
	"Your account is not authorized",
}

func AutomaticDisableKeywordsToString() string {
	return strings.Join(AutomaticDisableKeywords, "\n")
}

func AutomaticDisableKeywordsFromString(s string) {
	AutomaticDisableKeywords = []string{}
	ak := strings.Split(s, "\n")
	for _, k := range ak {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			AutomaticDisableKeywords = append(AutomaticDisableKeywords, k)
		}
	}
}
