package registrationcaptcha

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const RequiredClicks = 5

var ErrInvalidChallenge = errors.New("invalid or expired registration captcha")

type RateLimitError struct {
	RetryAfter time.Duration
}

func (err *RateLimitError) Error() string { return "too many registration captcha challenges" }

// Challenge contains only raster images and public display parameters. It must
// never expose glyph IDs, target coordinates, generation seeds or answers.
type Challenge struct {
	ID             string `json:"id"`
	Image          string `json:"image"`
	PromptImage    string `json:"prompt_image"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	RequiredClicks int    `json:"required_clicks"`
	ExpiresIn      int    `json:"expires_in"`
}

type Click struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	TimeMS int64   `json:"time_ms"`
}

type target struct {
	X      float64
	Y      float64
	Radius float64
}

type answer struct {
	Targets  []target
	IssuedAt int64
}

type Grants struct {
	EmailToken            string `json:"email_token"`
	RegistrationToken     string `json:"registration_token"`
	EmailExpiresIn        int    `json:"email_expires_in"`
	RegistrationExpiresIn int    `json:"registration_expires_in"`
}

func Create(ctx context.Context, scope Scope) (*Challenge, error) {
	if scope.Session == "" || scope.IP == "" {
		return nil, ErrInvalidChallenge
	}
	wait, err := reserveWindow(ctx, []string{keyNamespace + "issue_ip:" + digest(scope.IP)}, 5, time.Minute)
	if err != nil {
		return nil, err
	}
	if wait > 0 {
		return nil, &RateLimitError{RetryAfter: wait}
	}
	challenge, targets, err := generateImages()
	if err != nil {
		return nil, err
	}
	challenge.ID, err = NewSessionID()
	if err != nil {
		return nil, err
	}
	encoded, err := common.Marshal(answer{Targets: targets, IssuedAt: time.Now().UnixMilli()})
	if err != nil {
		return nil, err
	}
	if err = putToken(ctx, "challenge", challenge.ID, scope, string(encoded), ChallengeTTL); err != nil {
		return nil, err
	}
	return challenge, nil
}

func Verify(ctx context.Context, id string, scope Scope, clicks []Click) (*Grants, error) {
	// Each submitted answer gets one attempt, even if incorrect. A wrong scope
	// does not consume another browser's challenge.
	encoded, exists, err := readToken(ctx, "challenge", id, scope, true)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrInvalidChallenge
	}
	var expected answer
	if err = common.UnmarshalJsonStr(encoded, &expected); err != nil {
		return nil, ErrStoreUnavailable
	}
	if !matchesAnswer(expected, clicks, time.Now()) {
		return nil, ErrInvalidChallenge
	}
	grants := &Grants{
		EmailExpiresIn:        int(EmailTokenTTL.Seconds()),
		RegistrationExpiresIn: int(RegistrationTTL.Seconds()),
	}
	grants.EmailToken, err = NewSessionID()
	if err != nil {
		return nil, err
	}
	grants.RegistrationToken, err = NewSessionID()
	if err != nil {
		return nil, err
	}
	if err = putToken(ctx, EmailPurpose, grants.EmailToken, scope, "ok", EmailTokenTTL); err != nil {
		return nil, err
	}
	if err = putToken(ctx, RegistrationPurpose, grants.RegistrationToken, scope, "ok", RegistrationTTL); err != nil {
		return nil, err
	}
	return grants, nil
}

func matchesAnswer(expected answer, clicks []Click, now time.Time) bool {
	if len(clicks) != RequiredClicks || len(expected.Targets) != RequiredClicks {
		return false
	}
	elapsed := now.UnixMilli() - expected.IssuedAt
	if elapsed < 800 || elapsed >= ChallengeTTL.Milliseconds() {
		return false
	}
	var previous int64
	for i, click := range clicks {
		if math.IsNaN(click.X) || math.IsNaN(click.Y) || math.IsInf(click.X, 0) || math.IsInf(click.Y, 0) {
			return false
		}
		if click.X < 0 || click.X > imageWidth || click.Y < 0 || click.Y > imageHeight {
			return false
		}
		// Timing is only an additional signal, not a claim that fabricated mouse
		// traces can be distinguished from real users.
		if click.TimeMS < previous+60 || click.TimeMS > elapsed+1000 {
			return false
		}
		previous = click.TimeMS
		target := expected.Targets[i]
		if math.Hypot(click.X-target.X, click.Y-target.Y) > target.Radius {
			return false
		}
	}
	return previous >= 800
}
