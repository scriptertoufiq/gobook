// Package response is the only place that writes JSON to the wire, so every
// endpoint answers with the same envelope.
package response

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
	"github.com/scriptertoufiq/go-mvc/pkg/pagination"
)

type Envelope struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
	Error   *Fault `json:"error,omitempty"`
}

type Fault struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Data: data})
}

func Message(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Envelope{Success: true, Message: msg})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func Paginated(c *gin.Context, data any, meta pagination.Meta) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data, Meta: meta})
}

// Error renders any error using its apperror mapping. The wrapped cause is
// attached to the Gin context so the logging middleware can record it while
// the client only sees the safe message.
func Error(c *gin.Context, err error) {
	appErr := apperror.From(err)
	_ = c.Error(err) //nolint:errcheck // recorded for the logger middleware

	c.AbortWithStatusJSON(appErr.Status, Envelope{
		Success: false,
		Error: &Fault{
			Code:    appErr.Code,
			Message: appErr.Message,
			Fields:  fieldErrors(err),
		},
	})
}

// ValidationError renders a failed request binding.
//
// It separates two problems clients constantly confuse:
//
//	422 — the body parsed, but a value broke a rule. `fields` says which.
//	400 — the body could not be parsed at all (empty, malformed, wrong type).
//
// Returning 422 for both would tell a developer to go hunting through field
// rules when the real answer is "your JSON has a typo".
func ValidationError(c *gin.Context, err error) {
	fields := fieldErrors(err)
	if fields == nil {
		Error(c, bindingFault(err))
		return
	}

	c.AbortWithStatusJSON(http.StatusUnprocessableEntity, Envelope{
		Success: false,
		Error: &Fault{
			Code:    "validation_failed",
			Message: summarise(fields),
			Fields:  fields,
		},
	})
}

// summarise gives the top-level message something more useful than a fixed
// string — for a single problem it simply states it.
func summarise(fields map[string]string) string {
	if len(fields) == 1 {
		for _, only := range fields {
			return only
		}
	}
	return fmt.Sprintf("%d fields need attention. See `error.fields` for details.", len(fields))
}
