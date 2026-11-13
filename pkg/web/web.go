package web

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

const authHeader = "X-Auth-Token"

// TokenAuth returns Echo middleware that requires the configured token in
// either the X-Auth-Token header or an Authorization: Bearer header. When the
// token is empty, authentication is disabled.
func TokenAuth(token string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if token == "" {
				return next(c)
			}

			provided := c.Request().Header.Get(authHeader)
			if provided == "" {
				if scheme, value, ok := strings.Cut(c.Request().Header.Get(echo.HeaderAuthorization), " "); ok &&
					strings.EqualFold(scheme, "Bearer") {
					provided = strings.TrimSpace(value)
				}
			}

			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			return next(c)
		}
	}
}
