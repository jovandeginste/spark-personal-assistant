package app

import (
	"errors"
	"log/slog"
	"reflect"
	"testing"

	"github.com/awterman/monkey"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNewApp(t *testing.T) {
	app := NewApp()

	require.NotNil(t, app, "NewApp should return a non-nil App pointer")
	assert.Equal(t, "", app.ConfigFile, "Initially, ConfigFile should be an empty string")
	assert.Zero(t, app.Config, "Initially, Config struct should be zero-valued")
	// logger and db are zero-valued pointers/structs initially
	assert.Zero(t, app.logger, "Initially, logger should be a zero-valued struct")
	assert.Nil(t, app.db, "Initially, db should be a nil pointer")
}

func TestApp_Logger(t *testing.T) {
	app := &App{}
	// Manually set the logger field for testing purposes
	app.logger = *slog.Default() // Use default logger as a concrete instance

	logger := app.Logger()

	require.NotNil(t, logger, "Logger() should return a non-nil pointer")
	// Using reflect.DeepEqual or comparing pointers is better than comparing logger instances directly
	assert.Equal(t, reflect.ValueOf(&app.logger).Pointer(), reflect.ValueOf(logger).Pointer(), "Logger() should return a pointer to the internal logger instance")
}

func TestApp_Initialize(t *testing.T) {
	mockReadConfigErr := errors.New("mock read config error")
	mockInitDbErr := errors.New("mock init db error")

	tests := []struct {
		name                 string
		readConfigErr        error
		initLoggerShouldFail bool // initializeLogger currently doesn't return error, used for flow testing
		initDatabaseErr      error
		expectError          bool
		expectedErr          error
		expectReadConfigCall bool
		expectInitLoggerCall bool
		expectInitDbCall     bool
	}{
		{
			name:                 "Successful initialization",
			readConfigErr:        nil,
			initLoggerShouldFail: false,
			initDatabaseErr:      nil,
			expectError:          false,
			expectReadConfigCall: true,
			expectInitLoggerCall: true,
			expectInitDbCall:     true,
		},
		{
			name:                 "ReadConfig fails",
			readConfigErr:        mockReadConfigErr,
			initLoggerShouldFail: false, // Subsequent steps should not be called
			initDatabaseErr:      nil,
			expectError:          true,
			expectedErr:          mockReadConfigErr,
			expectReadConfigCall: true,
			expectInitLoggerCall: false, // Should skip subsequent steps
			expectInitDbCall:     false, // Should skip subsequent steps
		},
		// initializeLogger doesn't return an error based on current code,
		// so we only test the flow. If it were to return an error,
		// the test case structure would be similar to ReadConfig fails.
		// {
		// 	name:                 "InitializeLogger fails",
		// 	readConfigErr:        nil,
		// 	initLoggerShouldFail: true, // Simulate failure
		// 	initDatabaseErr:      nil, // Subsequent steps should not be called
		// 	expectError:          true,
		// 	expectedErr:          mockInitLoggerErr, // Assuming it returns mockInitLoggerErr
		// 	expectReadConfigCall: true,
		// 	expectInitLoggerCall: true,
		// 	expectInitDbCall:     false, // Should skip subsequent steps
		// },
		{
			name:                 "InitializeDatabase fails",
			readConfigErr:        nil,
			initLoggerShouldFail: false,
			initDatabaseErr:      mockInitDbErr,
			expectError:          true,
			expectedErr:          mockInitDbErr,
			expectReadConfigCall: true,
			expectInitLoggerCall: true,
			expectInitDbCall:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{}

			// Mock internal methods
			readConfigCalled := false
			patchReadConfig := monkey.Method(nil, app, app.ReadConfig, func() error {
				readConfigCalled = true
				return tt.readConfigErr
			})
			defer patchReadConfig.Reset()

			initLoggerCalled := false
			patchInitLogger := monkey.Method(nil, app, app.initializeLogger, func() {
				initLoggerCalled = true
				// If simulate failure, could panic or return a hardcoded error if method sig changes
				// For now, it's void, so no error to simulate directly.
			})
			defer patchInitLogger.Reset()

			initDbCalled := false
			patchInitDb := monkey.Method(nil, app, app.initializeDatabase, func() error {
				initDbCalled = true
				return tt.initDatabaseErr
			})
			defer patchInitDb.Reset()

			// Initialize a mock Mailer struct in Config
			app.Config.Mailer = Mailer{} // Mailer struct

			// Call the function under test
			err := app.Initialize()

			// Assert error expectations
			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err, "Error mismatch")
				}
			} else {
				assert.NoError(t, err, "Did not expect an error")
			}

			// Assert method calls based on expected flow
			assert.Equal(t, tt.expectReadConfigCall, readConfigCalled, "ReadConfig call expectation mismatch")
			assert.Equal(t, tt.expectInitLoggerCall, initLoggerCalled, "initializeLogger call expectation mismatch")
			assert.Equal(t, tt.expectInitDbCall, initDbCalled, "initializeDatabase call expectation mismatch")

			// Assert Mailer.app field is set if initialization proceeded past ReadConfig
			if tt.expectReadConfigCall && tt.readConfigErr == nil {
				assert.NotNil(t, app.Config.Mailer.app, "Mailer.app should be set after successful ReadConfig")
				// Use pointer comparison to ensure it's the same app instance
				assert.Equal(t, reflect.ValueOf(app).Pointer(), reflect.ValueOf(app.Config.Mailer.app).Pointer(), "Mailer.app should point back to the app instance")
			} else {
				assert.Nil(t, app.Config.Mailer.app, "Mailer.app should not be set if ReadConfig failed")
			}
		})
	}
}

func TestApp_initializeLogger(t *testing.T) {
	app := &App{}

	// Mock slog.Default() if needed to assert the specific logger instance.
	// However, simply checking that a.logger is non-zero after calling is sufficient
	// for testing that it was initialized.
	// Original code uses slog.Default(), which returns a global instance.
	// Testing equality with slog.Default() directly is fine.

	patchSlogDefault := monkey.Func(nil, slog.Default, func() *slog.Logger {
		// Return a distinct logger instance for the test
		// In real code, this would be the default one.
		// We can return a known mock or just verify it's non-nil.
		return slog.New(slog.NewTextHandler(nil, nil)) // Return a new logger
	})
	defer patchSlogDefault.Reset()

	// Ensure logger is zero-valued before call
	assert.Zero(t, app.logger, "logger should be zero-valued before initialization")

	// Call the function under test
	app.initializeLogger()

	// Assert logger is initialized
	assert.NotZero(t, app.logger, "logger should be initialized after calling initializeLogger")

	// You can further assert properties if needed, e.g., handler type, etc.,
	// but this is tightly coupled to slog implementation details.
	// Checking non-zero is usually enough.
}

func TestApp_initializeDatabase(t *testing.T) {
	mockSqliteOpenErr := errors.New("mock sqlite open error")
	mockMigrateErr := errors.New("mock migrate error")

	// Mock DB instance - we only need methods that initializeDatabase calls (AutoMigrate)
	mockDB := &gorm.DB{} // Minimal mock

	tests := []struct {
		name                 string
		sqliteOpenErr        error
		migrateErr           error
		expectError          bool
		expectedErr          error
		expectSqliteOpenCall bool
		expectMigrateCall    bool
		expectedDBSet        bool
	}{
		{
			name:                 "Successful database initialization and migration",
			sqliteOpenErr:        nil,
			migrateErr:           nil,
			expectError:          false,
			expectSqliteOpenCall: true,
			expectMigrateCall:    true,
			expectedDBSet:        true,
		},
		{
			name:                 "sqlite.Open fails",
			sqliteOpenErr:        mockSqliteOpenErr,
			migrateErr:           nil, // Should not be called
			expectError:          true,
			expectedErr:          mockSqliteOpenErr,
			expectSqliteOpenCall: true,
			expectMigrateCall:    false, // Should skip migration
			expectedDBSet:        false, // db should not be set
		},
		{
			name:                 "Migrate fails",
			sqliteOpenErr:        nil,
			migrateErr:           mockMigrateErr,
			expectError:          true,
			expectedErr:          mockMigrateErr,
			expectSqliteOpenCall: true,
			expectMigrateCall:    true, // Migrate should be called after successful Open
			expectedDBSet:        true, // db should be set by successful Open
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{}
			// Need Config and Config.Database.File for sqlite.Open call
			app.Config.Database.File = "/path/to/mock.db"

			// Mock sqlite.Open
			sqliteOpenCalled := false
			patchSqliteOpen := monkey.Func(nil, sqlite.Open, func(dsn string) gorm.Dialector {
				sqliteOpenCalled = true
				assert.Equal(t, app.Config.Database.File, dsn, "sqlite.Open called with incorrect DSN")
				// In a real mock, you'd return a mock Dialector that produces the mockDB.
				// For this test, we just need to control the error.
				// We can bypass the Dialector interface and directly patch gorm.Open
				// which is called after sqlite.Open returns the Dialector.
				return nil // Return nil Dialector, gorm.Open mock handles the rest
			})
			defer patchSqliteOpen.Reset()

			// Mock gorm.Open
			gormOpenCalled := false
			patchGormOpen := monkey.Func(nil, gorm.Open, func(dialector gorm.Dialector, opts ...gorm.Option) (*gorm.DB, error) {
				gormOpenCalled = true
				// Check opts if necessary, but the main check is the error return
				if tt.sqliteOpenErr != nil {
					// If sqlite.Open was meant to fail, the error would come from here in the real code flow
					return nil, tt.sqliteOpenErr
				}
				return mockDB, nil // Return mock DB on success
			})
			defer patchGormOpen.Reset()

			// Mock initializeLogger to ensure app.logger is set before sloggorm is called
			// In the real flow, Initialize calls initializeLogger before initializeDatabase
			// Although initializeDatabase itself doesn't *need* the logger to be initialized
			// (sloggorm uses slog.Default().Handler()), it's good practice for flow tests.
			// We can simply patch the method if necessary or ensure app.logger is non-zero.
			app.logger = *slog.Default() // Ensure logger is initialized for sloggorm mock

			// Mock app.Migrate
			migrateCalled := false
			patchMigrate := monkey.Method(nil, app, app.Migrate, func() error {
				migrateCalled = true
				return tt.migrateErr
			})
			defer patchMigrate.Reset()

			// Call the function under test
			err := app.initializeDatabase()

			// Assert error expectations
			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				if tt.expectedErr != nil {
					assert.Equal(t, tt.expectedErr, err, "Error mismatch")
				}
			} else {
				assert.NoError(t, err, "Did not expect an error")
			}

			// Assert method calls based on expected flow
			assert.Equal(t, tt.expectSqliteOpenCall, sqliteOpenCalled, "sqlite.Open call expectation mismatch")
			assert.Equal(t, tt.expectSqliteOpenCall, gormOpenCalled, "gorm.Open call expectation mismatch (should be called after sqlite.Open Dialector)")
			assert.Equal(t, tt.expectMigrateCall, migrateCalled, "Migrate call expectation mismatch")

			// Assert app.db field is set
			if tt.expectedDBSet {
				assert.NotNil(t, app.db, "app.db should be set after successful initialization")
				// If gorm.Open succeeded, app.db should point to the mockDB instance
				assert.Equal(t, mockDB, app.db, "app.db should be set to the mock DB instance")
			} else {
				assert.Nil(t, app.db, "app.db should be nil on error")
			}
		})
	}
}
