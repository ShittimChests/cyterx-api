package model_setting

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
)

// ModelAliasSetting 全局模型别名映射：用户请求的模型名（别名）-> 真实模型名。
// 别名解析发生在渠道选择之前（middleware/distributor.go），
// 因此渠道路由、计费、日志主模型名都使用真实模型名。
type ModelAliasSetting struct {
	Mapping *types.RWMap[string, string] `json:"mapping"`
}

var modelAliasSetting = ModelAliasSetting{
	Mapping: types.NewRWMap[string, string](),
}

func init() {
	config.GlobalConfig.Register("model_alias_setting", &modelAliasSetting)
}

// ResolveModelAlias 把全局别名解析为真实模型名，支持链式映射（a->b->c）并检测环。
// 自映射（a->a）视为未配置别名。带 compact 后缀的模型名先剥离后缀参与解析，结果再补回后缀。
// 返回（解析后的模型名, 是否发生了别名替换, error）。
func ResolveModelAlias(requested string) (string, bool, error) {
	if requested == "" || modelAliasSetting.Mapping.Len() == 0 {
		return requested, false, nil
	}
	baseName := requested
	hasCompactSuffix := strings.HasSuffix(requested, ratio_setting.CompactModelSuffix)
	if hasCompactSuffix {
		baseName = strings.TrimSuffix(requested, ratio_setting.CompactModelSuffix)
	}
	current := baseName
	visited := map[string]bool{current: true}
	applied := false
	for {
		mapped, exists := modelAliasSetting.Mapping.Get(current)
		if !exists || mapped == "" || mapped == current {
			break
		}
		if visited[mapped] {
			return requested, false, errors.New("model alias mapping contains a cycle")
		}
		visited[mapped] = true
		current = mapped
		applied = true
	}
	if !applied {
		return requested, false, nil
	}
	if hasCompactSuffix {
		current = ratio_setting.WithCompactModelSuffix(current)
	}
	return current, true, nil
}

// ValidateModelAliasMapping 校验待保存的别名映射 JSON：
// 必须是 {"别名": "真实模型"} 形式的对象，别名与目标均为非空字符串，
// 不允许自映射，不允许链式循环。
func ValidateModelAliasMapping(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)
	if trimmed == "" {
		return nil
	}
	mapping := make(map[string]string)
	if err := common.UnmarshalJsonStr(trimmed, &mapping); err != nil {
		return errors.New("模型别名映射必须是 {\"别名\": \"真实模型\"} 形式的 JSON 对象")
	}
	for alias, target := range mapping {
		if strings.TrimSpace(alias) == "" {
			return errors.New("模型别名映射的别名不能为空")
		}
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("别名 %s 的目标模型不能为空", alias)
		}
		if alias == target {
			return fmt.Errorf("别名 %s 不能映射到自身", alias)
		}
	}
	for alias := range mapping {
		visited := map[string]bool{alias: true}
		current := alias
		for {
			next, ok := mapping[current]
			if !ok || next == "" || next == current {
				break
			}
			if visited[next] {
				return fmt.Errorf("模型别名映射存在循环：%s -> %s", current, next)
			}
			visited[next] = true
			current = next
		}
	}
	return nil
}
