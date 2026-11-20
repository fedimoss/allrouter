package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// 测试意图：公开模型名先经渠道 model_mapping 解析，再决定面板和能力。
// 不能回退：若改回按模型名前缀判断，重定向后的模型会被误判。
func TestBuildVideoCapabilitiesUsesResolvedUpstreamModel(t *testing.T) {
	panels, capabilities := buildVideoCapabilities(
		[]string{"seedance-model", "chat-model"},
		map[string]model.PreferredModelOwnerChannel{
			"seedance-model": {
				ChannelType:  constant.ChannelTypeDoubaoVideo,
				ModelMapping: `{"seedance-model":"doubao-seedance-2-5-260628"}`,
			},
			"chat-model": {
				ChannelType: constant.ChannelTypeOpenAI,
			},
		},
	)
	require.Equal(t, "seedance", panels["seedance-model"])
	require.Equal(t, "2.5", capabilities["seedance-model"].Family)
	require.Equal(t, 30, capabilities["seedance-model"].MaxImages)
	require.NotContains(t, panels, "chat-model")
	require.NotContains(t, capabilities, "chat-model")
}
