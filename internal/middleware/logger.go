package middleware

import (
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// sensitiveParams are query keys whose values are credentials. A verification
// or reset link carries a live token in the URL, and logs get shipped,
// aggregated and read by people who should never hold one.
var sensitiveParams = map[string]bool{
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"password":      true,
	"secret":        true,
	"api_key":       true,
	"apikey":        true,
	"code":          true,
}

// redactQuery replaces sensitive values while keeping the shape of the query
// intact, so a log line still shows which parameters were sent.
func redactQuery(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		// Unparseable: redact wholesale rather than risk logging a secret.
		return "[unparseable query redacted]"
	}

	for key := range values {
		if sensitiveParams[strings.ToLower(key)] {
			for i := range values[key] {
				values[key][i] = "[REDACTED]"
			}
		}
	}

	return values.Encode()
}

// Logger emits one structured-ish line per request, including any error the
// handlers attached via c.Error — that's where response.Error stashes the
// real cause we deliberately hid from the client.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path += "?" + redactQuery(raw)
		}

		c.Next()

		entry := []any{
			c.Writer.Status(),
			c.Request.Method,
			path,
			time.Since(start).Round(time.Microsecond),
			c.ClientIP(),
			c.GetString(RequestIDKey),
		}

		if len(c.Errors) > 0 {
			log.Printf("%d %s %s %v ip=%s req=%s err=%s", append(entry, c.Errors.String())...)
			return
		}
		log.Printf("%d %s %s %v ip=%s req=%s", entry...)
	}
}
