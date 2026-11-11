package middleware

import (
	"net/http"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

var (
	relaySlotsOnce sync.Once
	relaySlots     chan struct{}
)

func RelayConcurrencyLimit() gin.HandlerFunc {
	if common.RelayMaxConcurrency <= 0 {
		return defNext
	}
	relaySlotsOnce.Do(func() {
		relaySlots = make(chan struct{}, common.RelayMaxConcurrency)
	})
	return relayConcurrencyLimitWithSlots(relaySlots)
}

func relayConcurrencyLimit(max int) gin.HandlerFunc {
	if max <= 0 {
		return defNext
	}
	return relayConcurrencyLimitWithSlots(make(chan struct{}, max))
}

func relayConcurrencyLimitWithSlots(slots chan struct{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			c.Next()
		default:
			setRetryAfter(c, 1)
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, "relay concurrency limit reached")
		}
	}
}
