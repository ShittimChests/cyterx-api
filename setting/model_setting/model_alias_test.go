package model_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setModelAliasMapping(t *testing.T, mapping map[string]string) {
	t.Helper()
	modelAliasSetting.Mapping.Clear()
	modelAliasSetting.Mapping.AddAll(mapping)
	t.Cleanup(func() {
		modelAliasSetting.Mapping.Clear()
	})
}

func TestResolveModelAlias(t *testing.T) {
	tests := []struct {
		name         string
		mapping      map[string]string
		requested    string
		wantResolved string
		wantApplied  bool
		wantErr      bool
	}{
		{
			name:         "empty mapping returns requested as-is",
			mapping:      map[string]string{},
			requested:    "cinax",
			wantResolved: "cinax",
			wantApplied:  false,
		},
		{
			name:         "single hop alias",
			mapping:      map[string]string{"cinax": "cinax-pro"},
			requested:    "cinax",
			wantResolved: "cinax-pro",
			wantApplied:  true,
		},
		{
			name:         "model not in mapping returns as-is",
			mapping:      map[string]string{"cinax": "cinax-pro"},
			requested:    "cinax-pro",
			wantResolved: "cinax-pro",
			wantApplied:  false,
		},
		{
			name:         "chained alias resolves to tail",
			mapping:      map[string]string{"a": "b", "b": "c"},
			requested:    "a",
			wantResolved: "c",
			wantApplied:  true,
		},
		{
			name:      "cycle returns error",
			mapping:   map[string]string{"a": "b", "b": "a"},
			requested: "a",
			wantErr:   true,
		},
		{
			name:         "self mapping treated as no alias",
			mapping:      map[string]string{"cinax": "cinax"},
			requested:    "cinax",
			wantResolved: "cinax",
			wantApplied:  false,
		},
		{
			name:         "empty target treated as no alias",
			mapping:      map[string]string{"cinax": ""},
			requested:    "cinax",
			wantResolved: "cinax",
			wantApplied:  false,
		},
		{
			// 别名按完整模型名整体匹配，不对名字做任何分段拆解。
			name:         "alias matches the full model name verbatim",
			mapping:      map[string]string{"cinax-openai-compact": "cinax-pro-openai-compact"},
			requested:    "cinax-openai-compact",
			wantResolved: "cinax-pro-openai-compact",
			wantApplied:  true,
		},
		{
			// 基名映射不会命中更长的模型名：解析不再剥离任何后缀，
			// 带后缀的模型必须显式配置自己的映射。
			name:         "base-name mapping does not match a longer model name",
			mapping:      map[string]string{"cinax": "cinax-pro"},
			requested:    "cinax-openai-compact",
			wantResolved: "cinax-openai-compact",
			wantApplied:  false,
		},
		{
			name:         "empty requested model returns as-is",
			mapping:      map[string]string{"cinax": "cinax-pro"},
			requested:    "",
			wantResolved: "",
			wantApplied:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setModelAliasMapping(t, tt.mapping)

			resolved, applied, err := ResolveModelAlias(tt.requested)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantResolved, resolved)
			assert.Equal(t, tt.wantApplied, applied)
		})
	}
}

func TestValidateModelAliasMapping(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		wantErr bool
	}{
		{name: "empty string is valid", jsonStr: "", wantErr: false},
		{name: "empty object is valid", jsonStr: "{}", wantErr: false},
		{name: "valid mapping", jsonStr: `{"cinax": "cinax-pro"}`, wantErr: false},
		{name: "valid chained mapping", jsonStr: `{"a": "b", "b": "c"}`, wantErr: false},
		{name: "array is invalid", jsonStr: `["cinax"]`, wantErr: true},
		{name: "non-string value is invalid", jsonStr: `{"cinax": 1}`, wantErr: true},
		{name: "empty alias is invalid", jsonStr: `{" ": "cinax-pro"}`, wantErr: true},
		{name: "empty target is invalid", jsonStr: `{"cinax": " "}`, wantErr: true},
		{name: "self mapping is invalid", jsonStr: `{"cinax": "cinax"}`, wantErr: true},
		{name: "cycle is invalid", jsonStr: `{"a": "b", "b": "a"}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateModelAliasMapping(tt.jsonStr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
