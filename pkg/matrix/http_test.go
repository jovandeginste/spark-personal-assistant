package matrix

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMatrixConfig(t *testing.T) *MatrixConfig {
	t.Helper()

	a := app.NewApp()
	a.ConfigFile = filepath.Join(t.TempDir(), "spark.yaml")
	require.NoError(t, a.Initialize())

	return &MatrixConfig{App: a}
}

func TestSwitchPersonaHandler(t *testing.T) {
	mc := newTestMatrixConfig(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/persona?persona=spock", nil)
	rec := httptest.NewRecorder()

	require.NoError(t, mc.switchPersona(e.NewContext(req, rec)))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Switched to persona: Spock")
}

func TestSwitchPersonaHandlerRequiresPersona(t *testing.T) {
	mc := newTestMatrixConfig(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/persona", nil)
	rec := httptest.NewRecorder()

	require.NoError(t, mc.switchPersona(e.NewContext(req, rec)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSwitchPersonaHandlerUnknownPersona(t *testing.T) {
	mc := newTestMatrixConfig(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/persona?persona=does-not-exist", nil)
	rec := httptest.NewRecorder()

	require.NoError(t, mc.switchPersona(e.NewContext(req, rec)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
