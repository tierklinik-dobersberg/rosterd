package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/tierklinik-dobersberg/cis/pkg/jwt"
)

var (
	ClaimsContextKey = struct{ s string }{"claims"}
	RawJWTContextKey = struct{ s string }{"rawjwt"}
)

func ClaimsFromContext(ctx context.Context) *jwt.Claims {
	c := ctx.Value(ClaimsContextKey)
	if c == nil {
		return nil
	}
	claims, _ := c.(*jwt.Claims)

	return claims
}

func JWTFromContext(ctx context.Context) string {
	c := ctx.Value(RawJWTContextKey)
	if c == nil {
		return ""
	}
	token, _ := c.(string)

	return token
}

func JWTAuth(cookieName string, secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {

			var jwtValue string

			cookie, err := c.Cookie(cookieName)
			if err == nil {
				jwtValue = cookie.Value
			} else {
				jwtValue = c.Request().Header.Get("Authorization")
				if jwtValue == "" {
					return c.NoContent(http.StatusUnauthorized)
				}

				if strings.HasPrefix(jwtValue, "Bearer ") {
					jwtValue = strings.TrimPrefix(jwtValue, "Bearer ")
				} else {
					L(c.Request().Context()).Info("invalid authorization header", "header", jwtValue)
					return c.NoContent(http.StatusForbidden)
				}
			}

			claims, err := jwt.ParseAndVerify([]byte(secret), jwtValue)
			if err != nil {
				if os.Getenv("ROSTERD_DEBUG") == "" || claims == nil {
					L(c.Request().Context()).Info("invalid authorization header", "error", err)
					return c.NoContent(http.StatusForbidden)
				}
			}

			AddLogFields(c,
				"jwt:subject", claims.Subject,
				"jwt:id", claims.ID,
			)

			ctx := context.WithValue(c.Request().Context(), ClaimsContextKey, claims)
			ctx = context.WithValue(ctx, RawJWTContextKey, jwtValue)

			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}
