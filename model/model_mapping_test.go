package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 测试意图：覆盖单跳、链式、自映射、未命中、循环和非法 JSON。
// 链式解析与循环检测是能力判断正确性的前提，不能删减这些用例。
func TestResolveMappedModelName(t *testing.T) {
	mapped, changed, err := ResolveMappedModelName(
		`{"public-seedance":"doubao-seedance-2-5-260628"}`,
		"public-seedance",
	)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "doubao-seedance-2-5-260628", mapped)

	mapped, changed, err = ResolveMappedModelName(
		`{"a":"b","b":"doubao-seedance-2-0-260128"}`,
		"a",
	)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "doubao-seedance-2-0-260128", mapped)
}

func TestResolveMappedModelNameSelfAndUnmapped(t *testing.T) {
	mapped, changed, err := ResolveMappedModelName(`{"a":"a"}`, "a")
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, "a", mapped)

	mapped, changed, err = ResolveMappedModelName(`{"other":"x"}`, "a")
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, "a", mapped)
}

func TestResolveMappedModelNameCycle(t *testing.T) {
	_, _, err := ResolveMappedModelName(`{"a":"b","b":"a"}`, "a")
	require.ErrorContains(t, err, "cycle")
}

func TestResolveMappedModelNameInvalidJSON(t *testing.T) {
	_, _, err := ResolveMappedModelName(`not-json`, "a")
	require.ErrorContains(t, err, "unmarshal_model_mapping_failed")
}
