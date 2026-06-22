package xhttp

import (
	"errors"

	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// ValidationError describes one failed field.
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Message string `json:"message"`
}

var validate = validator.New(validator.WithRequiredStructEnabled())

// ReadAndValidateRequest binds the request into req (body/query/path/header per
// echo binding tags), applies `default:"..."` tags, then validates `validate:"..."`
// tags. Returns nil on success, or a slice of ValidationError on failure.
//
// Bind / defaults errors are NOT echoed verbatim — Echo's bind errors include
// the offending byte offset and the raw underlying type, which leak parser
// internals to the caller. We surface a generic message and rely on the
// request-logging middleware to record the full error server-side.
func ReadAndValidateRequest(c echo.Context, req any) []ValidationError {
	if err := c.Bind(req); err != nil {
		c.Logger().Debugf("bind error: %v", err)
		return []ValidationError{{Field: "_body", Tag: "bind", Message: "payload không hợp lệ"}}
	}
	if err := defaults.Set(req); err != nil {
		c.Logger().Debugf("defaults error: %v", err)
		return []ValidationError{{Field: "_defaults", Tag: "defaults", Message: "payload không hợp lệ"}}
	}
	if err := validate.Struct(req); err != nil {
		var ve validator.ValidationErrors
		if !errors.As(err, &ve) {
			c.Logger().Debugf("validate error: %v", err)
			return []ValidationError{{Field: "_validate", Tag: "validate", Message: "payload không hợp lệ"}}
		}
		out := make([]ValidationError, 0, len(ve))
		for _, fe := range ve {
			out = append(out, ValidationError{
				Field:   fe.Field(),
				Tag:     fe.Tag(),
				Message: humanize(fe),
			})
		}
		return out
	}
	return nil
}

// RegisterValidation registers a custom validator (call at startup).
func RegisterValidation(tag string, fn validator.Func) error {
	return validate.RegisterValidation(tag, fn)
}

func humanize(fe validator.FieldError) string {
	field := fieldNameVN(fe.Field())
	switch fe.Tag() {
	case "required":
		return field + " là bắt buộc"
	case "min":
		return field + " phải có ít nhất " + fe.Param() + " ký tự"
	case "max":
		return field + " không được vượt quá " + fe.Param() + " ký tự"
	case "email":
		return field + " không phải email hợp lệ"
	case "oneof":
		return field + " phải là một trong: " + fe.Param()
	default:
		return field + " không hợp lệ (" + fe.Tag() + ")"
	}
}

func fieldNameVN(f string) string {
	switch f {
	case "Username":
		return "Tên đăng nhập"
	case "Password":
		return "Mật khẩu"
	case "FullName":
		return "Họ tên"
	case "Email":
		return "Email"
	case "Phone":
		return "Số điện thoại"
	case "Role":
		return "Vai trò"
	case "StaffCode":
		return "Mã nhân viên"
	case "Code":
		return "Mã"
	case "Name":
		return "Tên"
	case "Title":
		return "Tiêu đề"
	default:
		return f
	}
}
