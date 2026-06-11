package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"microservice/pkg/apperror"
)

var validate = newValidate()

func newValidate() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return fld.Name
		}
		return name
	})
	return v
}

// Validate runs struct validation. Returns nil on success, or a populated
// *appError.AppError (with field-level details) on failure.
func Validate(s any) *appError.AppError {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return appError.InvalidInput("invalid request")
	}
	fields := make([]appError.FieldError, 0, len(verrs))
	for _, fe := range verrs {
		fields = append(fields, appError.FieldError{
			Field:   fe.Field(),
			Message: message(fe),
		})
	}
	return appError.Validation("validation failed", fields)
}

func message(fe validator.FieldError) string {
	name := titleCase(fe.Field())
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", name)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", name)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters long", name, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters long", name, fe.Param())
	default:
		return fmt.Sprintf("%s is invalid (%s)", name, fe.Tag())
	}
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
