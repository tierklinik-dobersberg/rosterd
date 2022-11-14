package middleware

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/labstack/echo/v4"
)

var LoggerContextKey = struct{ S string }{"logger"}

// L returns the logger assigned to ctx. If no logger is available
// then hclog.Default() is returned.
func L(ctx context.Context) hclog.Logger {
	l := ctx.Value(LoggerContextKey)
	if l == nil {
		return hclog.Default()
	}

	logger, ok := l.(hclog.Logger)
	if !ok {
		return hclog.Default()
	}

	return logger
}

func AddLogFields(c echo.Context, keyval ...any) {
	req := c.Request()
	logger := L(req.Context())
	logger = logger.With(keyval...)

	c.SetRequest(
		req.WithContext(
			context.WithValue(req.Context(), LoggerContextKey, logger),
		),
	)
}

func RequestLogger(logger hclog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()

			logger = logger.With(
				"uri", req.URL.String(),
				"user-agent", req.UserAgent(),
				"method", req.Method,
			)

			c.SetRequest(
				req.WithContext(
					context.WithValue(req.Context(), LoggerContextKey, logger),
				),
			)

			start := time.Now()
			err := next(c)
			stop := time.Now()

			res := c.Response()

			// get the updated logger from the request
			logger = L(c.Request().Context()).With(
				"response", res.Status,
				"duration", stop.Sub(start).String(),
			)

			if err != nil {
				logger.Error("failed to handle request", "error", err.Error())
			} else {
				logger.Info("request handled")
			}

			return err
		}
	}
}
