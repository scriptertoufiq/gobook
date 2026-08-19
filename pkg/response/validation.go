package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"unicode"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/scriptertoufiq/gobook/pkg/apperror"
)

var jsonNamesOnce sync.Once

// UseJSONFieldNames makes validation errors name fields the way the client sent
// them — `refresh_token`, not `RefreshToken`.
//
// Without it, error keys leak Go struct names, which a frontend cannot match
// against its own form fields. Call once at startup; app.New does.
func UseJSONFieldNames() {
	jsonNamesOnce.Do(func() {
		engine, ok := binding.Validator.Engine().(*validator.Validate)
		if !ok {
			return
		}

		engine.RegisterTagNameFunc(func(field reflect.StructField) string {
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				return field.Name // no json tag: the Go name is all we have
			}
			return name
		})
	})
}

// fieldErrors turns validator output into a field -> message map.
func fieldErrors(err error) map[string]string {
	var invalid validator.ValidationErrors
	if !errors.As(err, &invalid) {
		return nil
	}

	out := make(map[string]string, len(invalid))
	for _, fe := range invalid {
		out[fe.Field()] = describe(fe)
	}
	return out
}

// describe renders one rule failure as a sentence a person can act on.
// "Password must be at least 8 characters." beats "failed on the min tag".
func describe(fe validator.FieldError) string {
	label := humanize(fe.Field())

	switch fe.Tag() {
	case "required":
		return label + " is required."
	case "required_without":
		return fmt.Sprintf("%s is required unless %s is set.", label, strings.ToLower(humanize(fe.Param())))
	case "required_with":
		return fmt.Sprintf("%s is required when %s is set.", label, strings.ToLower(humanize(fe.Param())))
	case "email":
		return label + " must be a valid email address."
	case "url":
		return label + " must be a valid URL."
	case "uuid", "uuid4":
		return label + " must be a valid UUID."
	case "numeric", "number":
		return label + " must be a number."
	case "alphanum":
		return label + " may only contain letters and numbers."
	case "min":
		return fmt.Sprintf("%s must be %s.", label, quantity(fe, "at least"))
	case "max":
		return fmt.Sprintf("%s must be %s.", label, quantity(fe, "no more than"))
	case "len":
		return fmt.Sprintf("%s must be %s.", label, quantity(fe, "exactly"))
	case "gt":
		return fmt.Sprintf("%s must be greater than %s.", label, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be %s or more.", label, fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s.", label, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be %s or less.", label, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s.", label, strings.Join(strings.Fields(fe.Param()), ", "))
	case "eqfield":
		return fmt.Sprintf("%s must match %s.", label, strings.ToLower(humanize(fe.Param())))
	case "nefield":
		return fmt.Sprintf("%s must be different from %s.", label, strings.ToLower(humanize(fe.Param())))
	default:
		return label + " is not valid."
	}
}

// quantity phrases a length/size bound in the unit that fits the type, so a
// string reads "at least 8 characters" and a number "at least 8".
func quantity(fe validator.FieldError, prefix string) string {
	switch fe.Kind() {
	case reflect.String:
		return fmt.Sprintf("%s %s characters", prefix, fe.Param())
	case reflect.Slice, reflect.Array, reflect.Map:
		return fmt.Sprintf("%s %s items", prefix, fe.Param())
	default:
		return fmt.Sprintf("%s %s", prefix, fe.Param())
	}
}

// humanize turns a JSON field name into a sentence-leading label:
// "refresh_token" -> "Refresh token".
func humanize(field string) string {
	if field == "" {
		return "This field"
	}

	spaced := strings.ReplaceAll(strings.ReplaceAll(field, "_", " "), "-", " ")
	runes := []rune(spaced)
	return string(unicode.ToUpper(runes[0])) + string(runes[1:])
}

// bindingFault explains a body that could not be decoded at all — as opposed to
// one that decoded fine but broke a rule. These are different problems and
// deserve different answers.
func bindingFault(err error) *apperror.Error {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
	)

	switch {
	case errors.Is(err, io.EOF):
		return apperror.BadRequest("Request body is empty. Send a JSON object.")

	// A truncated body — `{"email":` — surfaces as ErrUnexpectedEOF rather than
	// a SyntaxError, so it needs its own case or it falls through to the
	// unhelpful default.
	case errors.Is(err, io.ErrUnexpectedEOF):
		return apperror.BadRequest("Request body is not valid JSON — it ends unexpectedly. Check for a missing brace or quote.")

	case errors.As(err, &syntaxErr):
		return apperror.BadRequest(fmt.Sprintf(
			"Request body is not valid JSON (problem at character %d).", syntaxErr.Offset))

	case errors.As(err, &typeErr):
		label := humanize(typeErr.Field)
		if typeErr.Field == "" {
			label = "The request body"
		}
		return apperror.BadRequest(fmt.Sprintf("%s must be %s.", label, article(typeErr.Type)))

	default:
		return apperror.BadRequest("Request body could not be read. Send a valid JSON object.")
	}
}

// article names an expected Go type in words a client would recognise.
func article(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "true or false"
	case reflect.Slice, reflect.Array:
		return "a list"
	case reflect.Map, reflect.Struct:
		return "an object"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "a number"
	default:
		return "a " + t.Kind().String()
	}
}
