package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 测试意图：锁定 2.0、2.0 fast、2.5 和未知模型的能力矩阵。
// 这里的数值必须与上游文档和前端面板提示同步，改动代码时先改测试再改实现。
func TestSeedanceCapabilitiesForModel(t *testing.T) {
	caps := SeedanceCapabilitiesForModel("doubao-seedance-2-0-260128")
	require.Equal(t, "2.0", caps.Family)
	require.Equal(t, 9, caps.MaxImages)
	require.Equal(t, 3, caps.MaxVideos)
	require.Equal(t, 3, caps.MaxAudios)
	require.Equal(t, 15.0, caps.MaxVideoSeconds)
	require.Equal(t, 15.0, caps.MaxAudioSeconds)
	require.False(t, caps.AllowAudioOnly)
	require.Equal(t, 15, caps.MaxDuration)
	require.Contains(t, caps.Resolutions, "4k")

	caps = SeedanceCapabilitiesForModel("doubao-seedance-2-0-fast-260128")
	require.Equal(t, []string{"480p", "720p"}, caps.Resolutions)

	caps = SeedanceCapabilitiesForModel("doubao-seedance-2-5-260628")
	require.Equal(t, "2.5", caps.Family)
	require.Equal(t, 30, caps.MaxImages)
	require.Equal(t, 10, caps.MaxVideos)
	require.Equal(t, 10, caps.MaxAudios)
	require.Equal(t, 30.0, caps.MaxVideoSeconds)
	require.Equal(t, 30.0, caps.MaxAudioSeconds)
	require.True(t, caps.AllowAudioOnly)
	require.Equal(t, 30, caps.MaxDuration)
	require.Equal(t, []string{"mp4", "mov"}, caps.OutputFormats)
}

func TestSeedanceCapabilitiesUnknownFallsBackTo20(t *testing.T) {
	caps := SeedanceCapabilitiesForModel("dreamina-seedance-custom")
	require.Equal(t, "2.0", caps.Family)
	require.Equal(t, 9, caps.MaxImages)
	require.False(t, caps.AllowAudioOnly)
}
