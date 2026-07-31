package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestStreamOptionsIsolation(t *testing.T) {
	stream := true

	t.Run("responses chat stream injects usage", func(t *testing.T) {
		request := &dto.GeneralOpenAIRequest{
			Model:  "gpt-test",
			Stream: &stream,
		}
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeResponsesChat,
			UpstreamModelName: "gpt-test",
		}}

		converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
		require.NoError(t, err)
		require.Same(t, request, converted)
		require.NotNil(t, request.StreamOptions)
		require.True(t, request.StreamOptions.IncludeUsage)
	})

	t.Run("responses chat non-stream does not inject usage", func(t *testing.T) {
		stream := false
		request := &dto.GeneralOpenAIRequest{
			Model:         "gpt-test",
			Stream:        &stream,
			StreamOptions: &dto.StreamOptions{IncludeUsage: true},
		}
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeResponsesChat,
			UpstreamModelName: "gpt-test",
		}}

		_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
		require.NoError(t, err)
		require.Nil(t, request.StreamOptions)
	})

	t.Run("other compatibility channel still removes stream options", func(t *testing.T) {
		request := &dto.GeneralOpenAIRequest{
			Model:         "gpt-test",
			Stream:        &stream,
			StreamOptions: &dto.StreamOptions{IncludeUsage: true},
		}
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeGemini,
			UpstreamModelName: "gpt-test",
		}}

		_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
		require.NoError(t, err)
		require.Nil(t, request.StreamOptions)
	})
}
