package billing_setting

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/samber/lo"
)

const (
	BillingModeRatio           = "ratio"
	BillingModeTieredExpr      = "tiered_expr"
	BillingModePerSecond       = "per_second"
	BillingModeField           = "billing_mode"
	BillingExprField           = "billing_expr"
	VideoResolutionPricesField = "video_resolution_prices"
)

type VideoResolutionPricing struct {
	DefaultResolution string             `json:"default_resolution"`
	Prices            map[string]float64 `json:"prices"`
}

// BillingSetting is managed by config.GlobalConfig.Register.
// DB keys: billing_setting.billing_mode, billing_setting.billing_expr
type BillingSetting struct {
	BillingMode           map[string]string                 `json:"billing_mode"`
	BillingExpr           map[string]string                 `json:"billing_expr"`
	VideoResolutionPrices map[string]VideoResolutionPricing `json:"video_resolution_prices"`
}

var billingSetting = BillingSetting{
	BillingMode:           make(map[string]string),
	BillingExpr:           make(map[string]string),
	VideoResolutionPrices: make(map[string]VideoResolutionPricing),
}

func init() {
	config.GlobalConfig.Register("billing_setting", &billingSetting)
}

// ---------------------------------------------------------------------------
// Read accessors (hot path, must be fast)
// ---------------------------------------------------------------------------

func GetBillingMode(model string) string {
	if mode, ok := billingSetting.BillingMode[model]; ok {
		return mode
	}
	return BillingModeRatio
}

func GetBillingExpr(model string) (string, bool) {
	expr, ok := billingSetting.BillingExpr[model]
	return expr, ok
}

func GetBillingModeCopy() map[string]string {
	return lo.Assign(billingSetting.BillingMode)
}

func GetBillingExprCopy() map[string]string {
	return lo.Assign(billingSetting.BillingExpr)
}

func NormalizeResolution(resolution string) string {
	return strings.ToUpper(strings.TrimSpace(resolution))
}

func NormalizeVideoResolutionPricing(pricing VideoResolutionPricing) (VideoResolutionPricing, error) {
	pricing.DefaultResolution = NormalizeResolution(pricing.DefaultResolution)
	if pricing.DefaultResolution == "" {
		return VideoResolutionPricing{}, fmt.Errorf("default_resolution is required")
	}
	if len(pricing.Prices) == 0 {
		return VideoResolutionPricing{}, fmt.Errorf("at least one resolution price is required")
	}
	normalized := make(map[string]float64, len(pricing.Prices))
	for resolution, price := range pricing.Prices {
		resolution = NormalizeResolution(resolution)
		if resolution == "" {
			return VideoResolutionPricing{}, fmt.Errorf("resolution is required")
		}
		if _, exists := normalized[resolution]; exists {
			return VideoResolutionPricing{}, fmt.Errorf("duplicate resolution %s", resolution)
		}
		if math.IsNaN(price) || math.IsInf(price, 0) || price < 0 {
			return VideoResolutionPricing{}, fmt.Errorf("resolution %s price must be a finite non-negative number", resolution)
		}
		normalized[resolution] = price
	}
	if _, ok := normalized[pricing.DefaultResolution]; !ok {
		return VideoResolutionPricing{}, fmt.Errorf("default resolution %s has no price", pricing.DefaultResolution)
	}
	pricing.Prices = normalized
	return pricing, nil
}

func GetVideoResolutionPricing(model string) (VideoResolutionPricing, bool) {
	pricing, ok := billingSetting.VideoResolutionPrices[model]
	if !ok {
		return VideoResolutionPricing{}, false
	}
	normalized, err := NormalizeVideoResolutionPricing(pricing)
	if err != nil {
		return VideoResolutionPricing{}, false
	}
	return normalized, true
}

func GetVideoResolutionPricesCopy() map[string]VideoResolutionPricing {
	result := make(map[string]VideoResolutionPricing, len(billingSetting.VideoResolutionPrices))
	for model, pricing := range billingSetting.VideoResolutionPrices {
		if normalized, err := NormalizeVideoResolutionPricing(pricing); err == nil {
			result[model] = normalized
		}
	}
	return result
}

// SupportsPerSecondResolution reports whether the model's active billing mode
// has an exact, valid price entry for the requested resolution. A zero price
// is still a configured price and therefore supported.
func SupportsPerSecondResolution(model, resolution string) bool {
	if GetBillingMode(model) != BillingModePerSecond {
		return false
	}
	pricing, ok := GetVideoResolutionPricing(model)
	if !ok {
		return false
	}
	_, ok = pricing.Prices[NormalizeResolution(resolution)]
	return ok
}

func ResolvePerSecondPrice(model, resolution string) (string, float64, error) {
	pricing, ok := GetVideoResolutionPricing(model)
	if !ok {
		return "", 0, fmt.Errorf("model %s is configured as per_second but has no valid resolution prices", model)
	}
	resolution = NormalizeResolution(resolution)
	if resolution == "" {
		resolution = pricing.DefaultResolution
	}
	price, ok := pricing.Prices[resolution]
	if !ok && strings.EqualFold(strings.TrimSpace(model), "MiniMax-H3") && resolution == "720P" {
		// MiniMax-H3's Sora-compatible request uses the 720 tier name, while
		// completed videos report a 768-pixel short edge (for example 1344x768).
		// Keep an explicitly configured 720P tier authoritative; otherwise use
		// the actual-output 768P tier when it is available.
		if compatibilityPrice, exists := pricing.Prices["768P"]; exists {
			return "768P", compatibilityPrice, nil
		}
	}
	if !ok {
		configuredResolutions := make([]string, 0, len(pricing.Prices))
		for configuredResolution := range pricing.Prices {
			configuredResolutions = append(configuredResolutions, configuredResolution)
		}
		sort.Strings(configuredResolutions)
		return "", 0, fmt.Errorf(
			"model %s has no per-second price for resolution %s; configured resolutions: %s",
			model,
			resolution,
			strings.Join(configuredResolutions, ", "),
		)
	}
	return resolution, price, nil
}

func GetPricingSyncData(base map[string]any) map[string]any {
	extra := make(map[string]any, 3)
	if modes := GetBillingModeCopy(); len(modes) > 0 {
		extra[BillingModeField] = modes
	}
	if exprs := GetBillingExprCopy(); len(exprs) > 0 {
		extra[BillingExprField] = exprs
	}
	if prices := GetVideoResolutionPricesCopy(); len(prices) > 0 {
		extra[VideoResolutionPricesField] = prices
	}
	return lo.Assign(base, extra)
}

// ---------------------------------------------------------------------------
// Smoke test (called externally for validation before save)
// ---------------------------------------------------------------------------

func SmokeTestExpr(exprStr string) error {
	return smokeTestExpr(exprStr)
}

func smokeTestExpr(exprStr string) error {
	vectors := []billingexpr.TokenParams{
		{P: 0, C: 0, Len: 0},
		{P: 1000, C: 1000, Len: 1000},
		{P: 100000, C: 100000, Len: 100000},
		{P: 1000000, C: 1000000, Len: 1000000},
	}
	requests := []billingexpr.RequestInput{
		{},
		{
			Headers: map[string]string{
				"anthropic-beta": "fast-mode-2026-02-01",
			},
			Body: []byte(`{"service_tier":"fast","stream_options":{"include_usage":true},"messages":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21]}`),
		},
	}

	for _, v := range vectors {
		for _, request := range requests {
			result, _, err := billingexpr.RunExprWithRequest(exprStr, v, request)
			if err != nil {
				return fmt.Errorf("vector {p=%g, c=%g}: run failed: %w", v.P, v.C, err)
			}
			if result < 0 {
				return fmt.Errorf("vector {p=%g, c=%g}: result %f < 0", v.P, v.C, result)
			}
		}
	}
	return nil
}
