package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestTokenAuth(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		header     string
		value      string
		wantStatus int
	}{
		{name: "disabled", wantStatus: http.StatusOK},
		{name: "missing token", token: "secret", wantStatus: http.StatusUnauthorized},
		{name: "wrong token", token: "secret", header: authHeader, value: "wrong", wantStatus: http.StatusUnauthorized},
		{name: "custom header", token: "secret", header: authHeader, value: "secret", wantStatus: http.StatusOK},
		{
			name:       "bearer token",
			token:      "secret",
			header:     echo.HeaderAuthorization,
			value:      "Bearer secret",
			wantStatus: http.StatusOK,
		},
		{
			name:       "lowercase bearer token",
			token:      "secret",
			header:     echo.HeaderAuthorization,
			value:      "bearer secret",
			wantStatus: http.StatusOK,
		},
		{
			name:       "wrong bearer token",
			token:      "secret",
			header:     echo.HeaderAuthorization,
			value:      "Bearer wrong",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.Use(TokenAuth(tt.token))
			e.GET("/", func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set(tt.header, tt.value)
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
