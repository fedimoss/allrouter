package service

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

const (
	MiniMaxH3SourceShortEdge  = 768
	MiniMaxH3UpscaleShortEdge = 1536
)

// MiniMaxH3RequestedShortEdge validates the gateway-only output resolution.
func MiniMaxH3RequestedShortEdge(shortEdge *int) (int, error) {
	if shortEdge == nil {
		return MiniMaxH3SourceShortEdge, nil
	}
	if *shortEdge != MiniMaxH3SourceShortEdge && *shortEdge != MiniMaxH3UpscaleShortEdge {
		return 0, fmt.Errorf("short_edge must be 768 or 1536")
	}
	return *shortEdge, nil
}

func ValidateMiniMaxH3UpscaleConfig(shortEdge int) error {
	if shortEdge != MiniMaxH3UpscaleShortEdge {
		return nil
	}
	if strings.TrimSpace(constant.MiniMaxH3UpscaleURL) == "" || strings.TrimSpace(constant.MiniMaxH3UpscaleAPIKey) == "" {
		return fmt.Errorf("1536P output requires MINIMAX_H3_UPSCALE_URL and MINIMAX_H3_UPSCALE_API_KEY")
	}
	baseURL, err := url.Parse(strings.TrimSpace(constant.MiniMaxH3UpscaleURL))
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" || baseURL.User != nil {
		return fmt.Errorf("MINIMAX_H3_UPSCALE_URL must be a valid HTTP(S) base URL")
	}
	if (baseURL.Path != "" && baseURL.Path != "/") || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return fmt.Errorf("MINIMAX_H3_UPSCALE_URL must not include an API path, query, or fragment")
	}
	if outputOrigin := strings.TrimSpace(constant.MiniMaxH3UpscaleOutputOrigin); outputOrigin != "" {
		parsedOrigin, err := url.Parse(outputOrigin)
		if err != nil || (parsedOrigin.Scheme != "http" && parsedOrigin.Scheme != "https") || parsedOrigin.Host == "" || parsedOrigin.User != nil {
			return fmt.Errorf("MINIMAX_H3_UPSCALE_OUTPUT_ORIGIN must be a valid HTTP(S) origin")
		}
		if (parsedOrigin.Path != "" && parsedOrigin.Path != "/") || parsedOrigin.RawQuery != "" || parsedOrigin.Fragment != "" {
			return fmt.Errorf("MINIMAX_H3_UPSCALE_OUTPUT_ORIGIN must not include a path, query, or fragment")
		}
	}
	return nil
}
