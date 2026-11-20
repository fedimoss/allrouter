package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

//go:embed lua/rate_limit.lua
var rateLimitScript string

type RedisLimiter struct {
	client *redis.Client
}

var cachedRateLimitScript = redis.NewScript(rateLimitScript)

func New(_ context.Context, r *redis.Client) *RedisLimiter {
	return &RedisLimiter{client: r}
}

// EvalScript preserves SCRIPT LOAD + EVALSHA ACL compatibility. Script.Run
// falls back to EVAL, which existing deployments may deliberately disallow.
func EvalScript(ctx context.Context, client *redis.Client, script *redis.Script, keys []string, args ...interface{}) *redis.Cmd {
	result := script.EvalSha(ctx, client, keys, args...)
	if err := result.Err(); err != nil && strings.HasPrefix(err.Error(), "NOSCRIPT ") {
		if err := script.Load(ctx, client).Err(); err != nil {
			return redis.NewCmdResult(nil, err)
		}
		return script.EvalSha(ctx, client, keys, args...)
	}
	return result
}

func (rl *RedisLimiter) Allow(ctx context.Context, key string, opts ...Option) (bool, error) {
	// 默认配置
	config := &Config{
		Capacity:  10,
		Rate:      1,
		Requested: 1,
	}

	// 应用选项模式
	for _, opt := range opts {
		opt(config)
	}

	// 执行限流
	keys := []string{key}
	member := ""
	if config.Window > 0 {
		keys = append(keys, key+":window")
		member = common.GetUUID()
	}
	result, err := EvalScript(
		ctx,
		rl.client,
		cachedRateLimitScript,
		keys,
		config.Requested,
		config.Rate,
		config.Capacity,
		config.Window,
		member,
	).Int()

	if err != nil {
		common.SysLog(fmt.Sprintf("Failed to run rate limit script: %v", err))
		return false, fmt.Errorf("rate limit failed: %w", err)
	}
	return result == 1, nil
}

// Config 配置选项模式
type Config struct {
	Capacity  int64
	Rate      int64
	Requested int64
	Window    int64
}

type Option func(*Config)

func WithCapacity(c int64) Option {
	return func(cfg *Config) { cfg.Capacity = c }
}

func WithRate(r int64) Option {
	return func(cfg *Config) { cfg.Rate = r }
}

func WithRequested(n int64) Option {
	return func(cfg *Config) { cfg.Requested = n }
}

// WithWindow adds a strict rolling-window cap (in seconds) in addition to the
// legacy token bucket, preserving its existing state during an upgrade.
func WithWindow(seconds int64) Option {
	return func(cfg *Config) { cfg.Window = seconds }
}
