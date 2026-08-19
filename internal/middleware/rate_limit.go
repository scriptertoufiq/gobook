package middleware

import (
	"fmt"
	"math"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/gobook/pkg/apperror"
	"github.com/scriptertoufiq/gobook/pkg/ratelimit"
	"github.com/scriptertoufiq/gobook/pkg/response"
)

// RateLimit throttles requests per caller. A nil limiter is a pass-through, so
// the route table is identical whether or not throttling is switched on.
//
// Every response carries the X-RateLimit-* headers, not just the rejections —
// a client can then back off before it gets a 429 rather than after.
func RateLimit(limiter *ratelimit.Limiter) gin.HandlerFunc {
	if limiter == nil {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		result := limiter.Allow(rateLimitKey(c))

		c.Header("X-RateLimit-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(result.Reset.Unix(), 10))

		if !result.Allowed {
			// Round up: a Retry-After of 0 invites an immediate retry.
			retryAfter := max(int(math.Ceil(result.RetryAfter.Seconds())), 1)
			c.Header("Retry-After", strconv.Itoa(retryAfter))

			response.Error(c, apperror.TooManyRequests(fmt.Sprintf(
				"Too many requests. This endpoint allows %d, and you can try again in %d second(s).",
				result.Limit, retryAfter)))
			return
		}

		c.Next()
	}
}

// rateLimitKey identifies the caller.
//
// Throttling must happen before any expensive work, so this middleware is
// mounted ahead of Auth — which means there is no authenticated identity to key
// on yet, and the client IP is the only honest answer. An earlier version
// preferred a user id here; that branch could never execute given the handler
// order, so it is gone rather than left as a comforting lie.
//
// To add a per-user tier, mount a second RateLimit *after* Auth on the
// protected groups and key it from middleware.CurrentUserID.
//
// c.ClientIP() only honours X-Forwarded-For from proxies the engine has been
// told to trust — see app.New. Without that, any caller could rotate the header
// and mint themselves unlimited fresh buckets.
func rateLimitKey(c *gin.Context) string {
	return "ip:" + c.ClientIP()
}
