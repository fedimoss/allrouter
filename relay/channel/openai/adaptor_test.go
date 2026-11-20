package openai

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
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

func TestConvertOpenAIRequestSamplingParameterErrors(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		UpstreamModelName: "deepseek-v4.1-flash",
	}}
	adaptor := &Adaptor{}

	for _, tc := range []struct {
		name    string
		param   string
		value   float64
		convert func(*Adaptor, *relaycommon.RelayInfo, float64) error
	}{
		{
			name: "temperature above range", param: "temperature", value: 1.1,
			convert: func(adaptor *Adaptor, info *relaycommon.RelayInfo, value float64) error {
				_, err := adaptor.ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{Temperature: &value})
				return err
			},
		},
		{
			name: "top_p above range in Chat", param: "top_p", value: 1.1,
			convert: func(adaptor *Adaptor, info *relaycommon.RelayInfo, value float64) error {
				_, err := adaptor.ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{TopP: &value})
				return err
			},
		},
		{
			name: "top_p above range in Messages", param: "top_p", value: 1.1,
			convert: func(adaptor *Adaptor, info *relaycommon.RelayInfo, value float64) error {
				_, err := adaptor.ConvertClaudeRequest(nil, info, &dto.ClaudeRequest{TopP: &value})
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.convert(adaptor, info, tc.value)
			require.Error(t, err)
			apiErr, ok := err.(*types.NewAPIError)
			require.True(t, ok)
			require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
			require.Equal(t, types.ErrorCodeInvalidRequest, apiErr.GetErrorCode())
			require.Equal(t, "invalid_request_error", apiErr.ToOpenAIError().Type)
			require.Equal(t, tc.param, apiErr.ToOpenAIError().Param)
			require.True(t, strings.Contains(apiErr.Error(), tc.param))
		})
	}
}

func TestConvertOpenAIRequestPreservesExplicitZeroTemperature(t *testing.T) {
	zero := float64(0)
	request := &dto.GeneralOpenAIRequest{Temperature: &zero}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		UpstreamModelName: "deepseek-v4.1-flash",
	}}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	require.NoError(t, err)
	body, err := common.Marshal(converted)
	require.NoError(t, err)
	require.JSONEq(t, `{"temperature":0}`, string(body))

	claudeRequest := &dto.ClaudeRequest{Temperature: &zero}
	converted, err = (&Adaptor{}).ConvertClaudeRequest(nil, info, claudeRequest)
	require.NoError(t, err)
	body, err = common.Marshal(converted)
	require.NoError(t, err)
	require.JSONEq(t, `{"temperature":0}`, string(body))
}
