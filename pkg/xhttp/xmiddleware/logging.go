// Package xmiddleware holds shared Echo middlewares.
package xmiddleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

// RequestLogging logs one structured line per request and propagates a request id.
func RequestLogging(logger *xlogger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqID := c.Request().Header.Get(echo.HeaderXRequestID)
			if reqID == "" {
				reqID = uuid.NewString()
			}
			c.Response().Header().Set(echo.HeaderXRequestID, reqID)
			ctx := xhttp.WithRequestID(c.Request().Context(), reqID)
			ctx = xhttp.WithClientIP(ctx, c.RealIP())
			c.SetRequest(c.Request().WithContext(ctx))

			start := time.Now()
			err := next(c)
			lat := time.Since(start)

			// Echo ghi status vào response SAU khi middleware chain trả về (qua
			// HTTPErrorHandler). Tại đây c.Response().Status có thể vẫn là 200 mặc
			// định dù handler/router đã trả lỗi (vd 404 route-not-found). Lấy mã
			// thật từ *echo.HTTPError để log status + level chuẩn xác.
			status := c.Response().Status
			if err != nil {
				var he *echo.HTTPError
				if errors.As(err, &he) {
					status = he.Code
				} else if status < 400 {
					// Lỗi không phải HTTPError mà status chưa phản ánh -> 500.
					status = http.StatusInternalServerError
				}
			}

			fields := []xlogger.Field{
				xlogger.String("request_id", reqID),
				xlogger.String("method", c.Request().Method),
				xlogger.String("path", c.Request().URL.Path),
				xlogger.Int("status", status),
				xlogger.Dur("latency", lat),
				xlogger.String("ip", c.RealIP()),
			}
			// Surface the underlying error stashed by xhttp.AppErrorResponse so
			// generic 500s aren't logged blank when the handler swallowed `err`.
			if raw, ok := c.Get(xhttp.RawErrorContextKey).(error); ok && raw != nil {
				fields = append(fields, xlogger.Err(raw))
			}
			// Phân loại level theo status THẬT: 5xx = lỗi server (error),
			// 4xx = lỗi client / route-not-found / bot scan (warn, không phải
			// lỗi của ta), còn lại info. Tránh spam mức error vì bot quét 404.
			switch {
			case status >= 500:
				if err != nil {
					fields = append(fields, xlogger.Err(err))
				}
				logger.Error("http request", fields...)
			case status >= 400:
				logger.Warn("http request", fields...)
			default:
				logger.Info("http request", fields...)
			}
			return err
		}
	}
}
