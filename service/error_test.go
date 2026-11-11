package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestNormalizeUpstreamStatusCode(t *testing.T) {
	t.Parallel()

	require.Equal(t, http.StatusBadGateway, NormalizeUpstreamStatusCode(http.StatusInternalServerError))
	require.Equal(t, http.StatusBadGateway, NormalizeUpstreamStatusCode(http.StatusGatewayTimeout))
	require.Equal(t, http.StatusServiceUnavailable, NormalizeUpstreamStatusCode(http.StatusServiceUnavailable))
	require.Equal(t, http.StatusTooManyRequests, NormalizeUpstreamStatusCode(http.StatusTooManyRequests))
}

func TestWriteNormalizedUpstreamStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		upstream int
		want     int
		retry    string
	}{
		{upstream: http.StatusOK, want: http.StatusOK},
		{upstream: http.StatusInternalServerError, want: http.StatusBadGateway},
		{upstream: http.StatusGatewayTimeout, want: http.StatusBadGateway},
		{upstream: http.StatusTooManyRequests, want: http.StatusTooManyRequests, retry: "1"},
		{upstream: http.StatusServiceUnavailable, want: http.StatusServiceUnavailable, retry: "5"},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.upstream), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			WriteNormalizedUpstreamStatus(recorder, tt.upstream)
			require.Equal(t, tt.want, recorder.Code)
			require.Equal(t, tt.retry, recorder.Header().Get("Retry-After"))
		})
	}
}

func TestMidjourneyErrorStatusIsPreserved(t *testing.T) {
	t.Parallel()

	response := MidjourneyErrorWithStatusCodeWrapper(4, "do_request_failed", http.StatusBadGateway)
	require.Equal(t, http.StatusBadGateway, response.StatusCode)
	require.Equal(t, http.StatusBadGateway, response.Response.StatusCode)
}

func TestTokenPreConsumeErrorStatus(t *testing.T) {
	t.Parallel()

	insufficient := tokenPreConsumeError(fmt.Errorf("%w, token remain quota: 0", ErrInsufficientTokenQuota))
	require.Equal(t, http.StatusPaymentRequired, insufficient.StatusCode)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, insufficient.GetErrorCode())

	backendFailure := tokenPreConsumeError(fmt.Errorf("database write failed"))
	require.Equal(t, http.StatusInternalServerError, backendFailure.StatusCode)
	require.Equal(t, types.ErrorCodePreConsumeTokenQuotaFailed, backendFailure.GetErrorCode())
}

func TestBillingUnavailableErrorStatus(t *testing.T) {
	t.Parallel()

	apiErr := newBillingUnavailableError("wallet quota exhausted")
	require.Equal(t, http.StatusPaymentRequired, apiErr.StatusCode)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
}
