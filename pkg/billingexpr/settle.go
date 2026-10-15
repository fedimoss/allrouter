package billingexpr

import "github.com/QuantumNous/new-api/common"

// quotaConversion converts raw expression output to quota based on the
// expression version. This is the central dispatch point for future versions
// that may use a different conversion formula.
func quotaConversion(exprOutput float64, snap *BillingSnapshot) float64 {
	switch snap.ExprVersion {
	default: // v1: coefficients are $/1M tokens prices
		return exprOutput / 1_000_000 * snap.QuotaPerUnit
	}
}

// ComputeTieredQuota runs the Expr from a frozen BillingSnapshot against
// actual token counts and returns the settlement result.
func ComputeTieredQuota(snap *BillingSnapshot, params TokenParams) (TieredResult, error) {
	return ComputeTieredQuotaWithRequest(snap, params, RequestInput{})
}

func ComputeTieredQuotaWithRequest(snap *BillingSnapshot, params TokenParams, request RequestInput) (TieredResult, error) {
	originalCost, _, err := RunExprByHashWithRequest(snap.ExprString, snap.ExprHash, params, request)
	if err != nil {
		return TieredResult{}, err
	}

	discount := snap.UserDiscount
	if discount <= 0 || discount > 1 {
		discount = 1
	}
	discountableP := params.P
	discountableC := params.C
	if params.HasBreakdown {
		discountableP = params.DiscountableP
		discountableC = params.DiscountableC
	}
	if discountableP < 0 || discountableP > params.P {
		discountableP = params.P
	}
	if discountableC < 0 || discountableC > params.C {
		discountableC = params.C
	}
	discountedParams := params
	discountedParams.UseBillingPC = true
	discountedParams.BillingP = discountableP*discount + (params.P - discountableP)
	discountedParams.BillingC = discountableC*discount + (params.C - discountableC)
	cost, trace, err := RunExprByHashWithRequest(snap.ExprString, snap.ExprHash, discountedParams, request)
	if err != nil {
		return TieredResult{}, err
	}

	noCacheParams := params
	noCacheParams.UseBillingPC = true
	noCacheParams.BillingP = params.P - params.CacheFallbackP
	if noCacheParams.BillingP < 0 {
		noCacheParams.BillingP = 0
	}
	noCacheParams.BillingC = params.C
	noCacheParams.CR = 0
	noCacheParams.CC = 0
	noCacheParams.CC1h = 0
	noCacheCost, _, err := RunExprByHashWithRequest(snap.ExprString, snap.ExprHash, noCacheParams, request)
	if err != nil {
		return TieredResult{}, err
	}
	cacheCost := originalCost - noCacheCost
	if cacheCost < 0 || cacheCost > originalCost {
		cacheCost = 0
	}

	noDiscountableParams := params
	noDiscountableParams.UseBillingPC = true
	noDiscountableParams.BillingP = params.P - discountableP
	noDiscountableParams.BillingC = params.C - discountableC
	noDiscountableCost, _, err := RunExprByHashWithRequest(snap.ExprString, snap.ExprHash, noDiscountableParams, request)
	if err != nil {
		return TieredResult{}, err
	}
	discountableCost := originalCost - noDiscountableCost
	if discountableCost < 0 || discountableCost > originalCost {
		discountableCost = 0
	}

	quotaBeforeGroup := quotaConversion(cost, snap)
	originalQuotaBeforeGroup := quotaConversion(originalCost, snap)
	cacheQuotaBeforeGroup := quotaConversion(cacheCost, snap)
	discountableQuotaBeforeGroup := quotaConversion(discountableCost, snap)
	afterGroup, clamp := common.QuotaRoundChecked(quotaBeforeGroup * snap.GroupRatio)
	crossed := trace.MatchedTier != snap.EstimatedTier

	return TieredResult{
		OriginalQuotaBeforeGroup:     originalQuotaBeforeGroup,
		ActualQuotaBeforeGroup:       quotaBeforeGroup,
		ActualQuotaAfterGroup:        afterGroup,
		CacheQuotaBeforeGroup:        cacheQuotaBeforeGroup,
		DiscountableQuotaBeforeGroup: discountableQuotaBeforeGroup,
		DiscountableTokens:           discountableP + discountableC,
		DiscountAmountBeforeGroup:    originalQuotaBeforeGroup - quotaBeforeGroup,
		AppliedUserDiscount:          discount,
		MatchedTier:                  trace.MatchedTier,
		CrossedTier:                  crossed,
		Clamp:                        clamp,
	}, nil
}
