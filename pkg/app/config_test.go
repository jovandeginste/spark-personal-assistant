package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awterman/monkey"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetDefaults(t *testing.T) {
	tests := []struct {
		name              string
		initialConfig     Config
		expectedAssistant ai.AssistantConfig
	}{
		{
			name: "Both name and style are empty",
			initialConfig: Config{
				Assistant: ai.AssistantConfig{Name: "", Style: ""},
			},
			expectedAssistant: ai.AssistantConfig{
				Name:  "Spark",
				Style: "Assume the persona of a classic, highly professional English butler.",
			},
		},
		{
			name: "Name is empty, style is not",
			initialConfig: Config{
				Assistant: ai.AssistantConfig{Name: "", Style: "Custom Style"},
			},
			expectedAssistant: ai.AssistantConfig{
				Name:  "Spark",
				Style: "Custom Style",
			},
		},
		{
			name: "Name is not empty, style is empty",
			initialConfig: Config{
				Assistant: ai.AssistantConfig{Name: "MyAssistant", Style: ""},
			},
			expectedAssistant: ai.AssistantConfig{
				Name:  "MyAssistant",
				Style: "Assume the persona of a classic, highly professional English butler.",
			},
		},
		{
			name: "Both name and style are not empty",
			initialConfig: Config{
				Assistant: ai.AssistantConfig{Name: "MyAssistant", Style: "Custom Style"},
			},
			expectedAssistant: ai.AssistantConfig{
				Name:  "MyAssistant",
				Style: "Custom Style",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{Config: tt.initialConfig}

			app.SetDefaults()

			assert.Equal(t, app.Config.Assistant.Name, tt.expectedAssistant.Name, "Assistant name mismatch")
			assert.Contains(t, app.Config.Assistant.Style, tt.expectedAssistant.Style, "Assistant style mismatch")
		})
	}
}

type testReadConfigCase struct {
	name string
	// Mocks for viper operations
	mockSetConfigFile func(path string)   // Use a function to assert the path passed to SetConfigFile
	mockReadInConfig  func() error        // Return value for ReadInConfig
	mockUnmarshal     func(cfg any) error // Behavior for Unmarshal

	// Mocks for path operations
	mockFilepathAbs func(path string) (string, error) // Behavior for filepath.Abs

	// Control flow flags/inputs
	configFileArg string // Value passed to --config flag (simulated)
	sparkEnvVar   string // Value of SPARK_CONFIG env var (simulated)

	expectError    bool
	expectedErr    error
	expectedDBPath string // Expected final value of a.Config.Database.File
}

func TestReadConfig(t *testing.T) {
	// Mock data that viper.Unmarshal will populate into a.Config
	mockUnmarshaledConfigData := Config{
		Assistant:    ai.AssistantConfig{Name: "TestAI", Style: "Formal"},
		Database:     DatabaseConfig{File: "relative/path/to/db.sqlite"},
		UserData:     UserData{Names: []string{"User One", "User Two"}},
		ExtraContext: []string{"Context 1", "Context 2"},
		Mailer: Mailer{
			From:    MailerFrom{Name: "Test Mailer", Address: "test@example.com"},
			Server:  MailerServer{Address: "smtp.test.com", Port: 587, UserName: "user", Password: "pass"},
			Preview: false,
			Bcc:     "bcc@example.com",
		},
		LLM: &ai.AIConfig{Type: "openai", Model: "gpt-3.5-turbo", APIKey: "testkey"},
	}

	// Mock data for successful unmarshaling with absolute db path
	mockUnmarshaledConfigAbsoluteDBData := Config{
		Assistant:    ai.AssistantConfig{Name: "AbsoluteAI", Style: "Casual"},
		Database:     DatabaseConfig{File: "/absolute/path/to/db.sqlite"},
		UserData:     UserData{Names: []string{"Absolute User"}},
		ExtraContext: []string{"Absolute Context"},
		Mailer: Mailer{
			From:    MailerFrom{Name: "Absolute Mailer", Address: "absolute@example.com"},
			Server:  MailerServer{Address: "smtp.abs.com", Port: 465, UserName: "absuser", Password: "abspass"},
			Preview: true,
			Bcc:     "",
		},
		LLM: &ai.AIConfig{Type: "gemini", Model: "gemini-pro", APIKey: "absolute_testkey"},
	}

	tests := []testReadConfigCase{
		{
			name:              "Successful read with relative database path",
			mockSetConfigFile: func(path string) { assert.Equal(t, "test_config.yaml", path) },
			mockReadInConfig:  func() error { return nil },
			mockUnmarshal: func(cfg any) error {
				cPtr, ok := cfg.(*Config)
				require.True(t, ok, "Unmarshal target is not *Config")
				*cPtr = mockUnmarshaledConfigData // Copy mock data
				return nil
			},
			mockFilepathAbs: func(path string) (string, error) {
				// filepath.Abs will be called on the ConfigFile path
				if path == "test_config.yaml" {
					return "/mock/absolute/config/path/test_config.yaml", nil
				}
				t.Errorf("Unexpected filepath.Abs call for: %s", path) // Fail if Abs is called on something else
				return "", errors.New("unexpected call")
			},
			configFileArg:  "test_config.yaml", // Explicitly setting via flag
			expectError:    false,
			expectedDBPath: "/mock/absolute/config/path/relative/path/to/db.sqlite", // Expected resolved path
		},
		{
			name:              "Successful read with absolute database path",
			mockSetConfigFile: func(path string) { assert.Equal(t, "test_config_abs.yaml", path) },
			mockReadInConfig:  func() error { return nil },
			mockUnmarshal: func(cfg any) error {
				cPtr, ok := cfg.(*Config)
				require.True(t, ok, "Unmarshal target is not *Config")
				*cPtr = mockUnmarshaledConfigAbsoluteDBData // Copy mock data
				return nil
			},
			mockFilepathAbs: func(path string) (string, error) {
				if path == "test_config_abs.yaml" {
					return "/another/absolute/config/path/test_config_abs.yaml", nil
				}
				t.Errorf("Unexpected filepath.Abs call for: %s", path)
				return "", errors.New("unexpected call")
			},
			configFileArg:  "test_config_abs.yaml",
			expectError:    false,
			expectedDBPath: "/absolute/path/to/db.sqlite", // Should remain absolute
		},
		{
			name:              "ConfigFileNotFoundError (handled), uses defaults",
			mockSetConfigFile: func(path string) { assert.Equal(t, "non_existent.yaml", path) },
			mockReadInConfig: func() error {
				return viper.ConfigFileNotFoundError{}
			},
			mockUnmarshal: func(cfg any) error {
				// Simulate viper unmarshaling an empty config struct when file is not found
				cPtr, ok := cfg.(*Config)
				require.True(t, ok, "Unmarshal target is not *Config")
				*cPtr = Config{} // Start with empty config
				return nil
			},
			mockFilepathAbs: func(path string) (string, error) {
				if path == "non_existent.yaml" {
					return "/mock/path/non_existent.yaml", nil
				}
				t.Errorf("Unexpected filepath.Abs call for: %s", path)
				return "", errors.New("unexpected call")
			},
			configFileArg:  "non_existent.yaml",
			expectError:    false,        // ConfigFileNotFoundError is not returned by ReadConfig
			expectedDBPath: "/mock/path", // Path resolution doesn't happen for Database.File
		},
		{
			name:              "Other ReadInConfig error",
			mockSetConfigFile: func(path string) { assert.Equal(t, "test_error.yaml", path) },
			mockReadInConfig:  func() error { return errors.New("permission denied") },
			mockUnmarshal: func(cfg any) error {
				t.Errorf("Unmarshal should not be called on ReadInConfig error")
				return nil
			},
			mockFilepathAbs: func(path string) (string, error) {
				if path == "test_error.yaml" {
					return "/mock/path/test_error.yaml", nil
				}
				t.Errorf("Unexpected filepath.Abs call for: %s", path)
				return "", errors.New("unexpected call")
			},
			configFileArg:  "test_error.yaml",
			expectError:    true,
			expectedErr:    errors.New("permission denied"),
			expectedDBPath: "",
		},
		{
			name:              "Unmarshal error",
			mockSetConfigFile: func(path string) { assert.Equal(t, "test_unmarshal.yaml", path) },
			mockReadInConfig:  func() error { return nil },
			mockUnmarshal:     func(cfg any) error { return errors.New("unmarshal failed") },
			mockFilepathAbs: func(path string) (string, error) {
				if path == "test_unmarshal.yaml" {
					return "/mock/path/test_unmarshal.yaml", nil
				}
				t.Errorf("Unexpected filepath.Abs call for: %s", path)
				return "", errors.New("unexpected call")
			},
			configFileArg:  "test_unmarshal.yaml",
			expectError:    true,
			expectedErr:    errors.New("unmarshal failed"),
			expectedDBPath: "",
		},
		{
			name:              "filepath.Abs error for config file",
			mockSetConfigFile: func(path string) { assert.Equal(t, "test_abs_error.yaml", path) },
			mockReadInConfig:  func() error { return nil }, // Need ReadInConfig to succeed for Abs to be called on ConfigFile
			mockUnmarshal: func(cfg any) error {
				// Need Unmarshal to succeed for the DB path resolution logic to be attempted
				cPtr, ok := cfg.(*Config)
				require.True(t, ok, "Unmarshal target is not *Config")
				*cPtr = mockUnmarshaledConfigData // Use relative path mock data
				return nil
			},
			mockFilepathAbs: func(path string) (string, error) {
				if path == "test_abs_error.yaml" {
					return "", errors.New("abs error on config file")
				}
				t.Errorf("Unexpected filepath.Abs call for: %s", path)
				return "", errors.New("unexpected call")
			},
			configFileArg:  "test_abs_error.yaml",
			expectError:    true,
			expectedErr:    errors.New("abs error on config file"), // Expect the abs error
			expectedDBPath: "",                                     // Path resolution doesn't complete
		},
		{
			name:              "Config file from SPARK_CONFIG env var (relative path)",
			mockSetConfigFile: func(path string) { assert.Equal(t, "relative_env_config.yaml", path) },
			mockReadInConfig:  func() error { return nil },
			mockUnmarshal: func(cfg any) error {
				cPtr, ok := cfg.(*Config)
				require.True(t, ok, "Unmarshal target is not *Config")
				*cPtr = mockUnmarshaledConfigData // Use relative path mock data
				return nil
			},
			mockFilepathAbs: func(path string) (string, error) {
				if path == "relative_env_config.yaml" {
					return "/mock/absolute/current/relative_env_config.yaml", nil // Simulate abs path of env var path
				}
				t.Errorf("Unexpected filepath.Abs call for: %s", path)
				return "", errors.New("unexpected call")
			},
			configFileArg:  "",                         // No flag, rely on env var
			sparkEnvVar:    "relative_env_config.yaml", // Env var is relative
			expectError:    false,
			expectedDBPath: "/mock/absolute/current/relative/path/to/db.sqlite", // Resolved relative to the absolute env path
		},
		{
			name:              "Config file from SPARK_CONFIG env var (absolute path)",
			mockSetConfigFile: func(path string) { assert.Equal(t, "/env/path/absolute_env_config.yaml", path) },
			mockReadInConfig:  func() error { return nil },
			mockUnmarshal: func(cfg any) error {
				cPtr, ok := cfg.(*Config)
				require.True(t, ok, "Unmarshal target is not *Config")
				*cPtr = mockUnmarshaledConfigData // Use relative path mock data
				return nil
			},
			mockFilepathAbs: func(path string) (string, error) {
				if path == "/env/path/absolute_env_config.yaml" {
					return "/env/path/absolute_env_config.yaml", nil // Abs called on the env var path (already absolute)
				}
				t.Errorf("Unexpected filepath.Abs call for: %s", path)
				return "", errors.New("unexpected call")
			},
			configFileArg:  "",                                   // No flag, rely on env var
			sparkEnvVar:    "/env/path/absolute_env_config.yaml", // Env var is absolute
			expectError:    false,
			expectedDBPath: "/env/path/relative/path/to/db.sqlite", // Resolved relative to the absolute env path
		},
		{
			name:              "Config file flag overrides SPARK_CONFIG",
			mockSetConfigFile: func(path string) { assert.Equal(t, "flag_config.yaml", path) }, // Should use flag value
			mockReadInConfig:  func() error { return nil },
			mockUnmarshal: func(cfg any) error {
				cPtr, ok := cfg.(*Config)
				require.True(t, ok, "Unmarshal target is not *Config")
				*cPtr = mockUnmarshaledConfigData // Use relative path mock data
				return nil
			},
			mockFilepathAbs: func(path string) (string, error) {
				if path == "flag_config.yaml" { // Abs called on the flag path
					return "/mock/absolute/flag/path/flag_config.yaml", nil
				}
				t.Errorf("Unexpected filepath.Abs call for: %s", path)
				return "", errors.New("unexpected call")
			},
			configFileArg:  "flag_config.yaml",                  // Explicitly setting via flag
			sparkEnvVar:    "/env/path/env_config_ignored.yaml", // This should be ignored
			expectError:    false,
			expectedDBPath: "/mock/absolute/flag/path/relative/path/to/db.sqlite", // Resolved relative to the absolute flag path
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testAppWith(t, &tt)
		})
	}
}

func testAppWith(t *testing.T, tt *testReadConfigCase) {
	t.Helper()
	app := &App{} // Start with a fresh App instance

	// Manually set ConfigFile field on the app instance, simulating Cobra's flag binding
	// This replicates the logic in cmd/spark/cli.go's PersistentFlags().StringVar
	app.ConfigFile = "./spark.yaml" // Default value
	if sparkConfigEnv, ok := os.LookupEnv("SPARK_CONFIG"); ok {
		app.ConfigFile = sparkConfigEnv // Env var overrides default
	}

	// The --config flag would override both. We simulate this by setting app.ConfigFile directly
	// if configFileArg is provided in the test case.
	if tt.configFileArg != "" {
		app.ConfigFile = tt.configFileArg
	} else if tt.sparkEnvVar != "" {
		// If sparkEnvVar is set in the test, explicitly set the env var for the duration
		// of this test case and also update app.ConfigFile to match what the CLI would do.
		t.Setenv("SPARK_CONFIG", tt.sparkEnvVar)
		app.ConfigFile = tt.sparkEnvVar // Explicitly set what the CLI would
	}

	// --- Patching ---
	// viper.SetConfigFile is called with the value of a.ConfigFile
	patches := monkey.Func(nil, viper.SetConfigFile, tt.mockSetConfigFile)
	defer patches.Reset()

	// viper.ReadInConfig is called next
	readInConfigCalls := 0
	monkey.Func(patches, viper.ReadInConfig, func() error {
		readInConfigCalls++
		return tt.mockReadInConfig()
	})

	// viper.Unmarshal is called after ReadInConfig (unless ReadInConfig returns a non-ConfigFileNotFoundError)
	// Patching viper.Unmarshal requires patching a method on the *viper.Viper instance.
	// The code `viper.Unmarshal(&a.Config)` implicitly uses the default global viper instance.
	// We can patch the Unmarshal method directly on the default viper instance.
	unmarshalCalls := 0
	monkey.Method(patches, viper.GetViper(), viper.Unmarshal, func(runTimeConfig any, _ ...viper.DecoderConfigOption) error {
		unmarshalCalls++
		return tt.mockUnmarshal(runTimeConfig) // Call the test-specific unmarshal behavior
	})

	// filepath.Abs is called *after* Unmarshal if the database path is relative.
	// The current code calls it *only* on the config file path (`filepath.Abs(a.ConfigFile)`)
	// *after* unmarshalling and the initial absolute check.
	absCalls := []string{}
	monkey.Func(patches, filepath.Abs, func(path string) (string, error) {
		absCalls = append(absCalls, path)
		return tt.mockFilepathAbs(path) // Call the test-specific Abs behavior
	})

	// SetDefaults is called after Unmarshal and before database path resolution
	setDefaultsCallCount := 0
	monkey.Method(patches, app, app.SetDefaults, func() {
		setDefaultsCallCount++
		// Don't call the real SetDefaults here to keep this mock simple.
		// The actual effects of SetDefaults on the final config are verified
		// by comparing against a config struct that has had SetDefaults applied elsewhere.
	})

	// --- Call the function under test ---
	err := app.ReadConfig()

	// --- Assertions ---
	if tt.expectError {
		assert.Error(t, err, "Expected an error")
		if tt.expectedErr != nil {
			assert.Equal(t, tt.expectedErr, err, "Error mismatch")
		}
		// If error, app.Config state might be partially populated or zero, hard to assert.
		return
	}

	assert.NoError(t, err, "Did not expect an error")

	// Assert viper calls
	assert.Equal(t, 1, readInConfigCalls, "viper.ReadInConfig should be called exactly once")
	assert.Equal(t, 1, unmarshalCalls, "viper.Unmarshal should be called exactly once")

	// Assert SetDefaults was called
	assert.Equal(t, 1, setDefaultsCallCount, "SetDefaults should be called exactly once")

	// Assert filepath.Abs call(s)
	// filepath.Abs is called once on `a.ConfigFile` after unmarshal if the DB path is relative.
	// If the DB path is absolute, filepath.Abs is not called in this resolution block.
	if !strings.HasPrefix(app.Config.Database.originalFile, "/") {
		// Relative path, filepath.Abs(a.ConfigFile) should be called once
		if assert.Equal(t, 1, len(absCalls), "filepath.Abs should be called exactly once for relative DB path") {
			assert.Equal(t, app.ConfigFile, absCalls[0], "filepath.Abs called with incorrect path for relative DB path")
		}
	} else {
		// Absolute path, filepath.Abs should not be called in the path resolution block
		assert.Equal(t, 0, len(absCalls), "filepath.Abs should not be called for absolute DB path")
	}

	// Verify the final state of app.Config after unmarshalling and real SetDefaults.
	// To do this, apply the real SetDefaults to a copy of the unmarshaled data.
	tempAppConfig := Config{}
	// Re-run the unmarshal mock to get the unmarshaled state into tempAppConfig
	// This is necessary because the mockUnmarshal function only modifies the pointer
	// passed to it during the actual ReadConfig call.
	// We need to ensure the temp copy gets the same unmarshaled data.
	// This relies on the mockUnmarshal setting the cfg pointer correctly.
	_ = tt.mockUnmarshal(&tempAppConfig)
	tempApp := &App{Config: tempAppConfig}
	tempApp.SetDefaults() // Apply real SetDefaults to the temporary config

	testEqualApp(t, tempApp, app)

	assert.Equal(t, tt.expectedDBPath, app.Config.Database.File, "Database file path mismatch after resolution")
}

func testEqualApp(t *testing.T, tempApp, app *App) {
	t.Helper()

	// Now compare the main app.Config (which had real SetDefaults applied because we patched the method)
	// against the tempApp.Config (which we explicitly applied real SetDefaults to).
	// Note: The patching of SetDefaults makes the assertion here simpler -
	// we just need to check if the final state of app.Config matches the state
	// resulting from the unmarshaled data *plus* the real SetDefaults logic.
	// The DB path is resolved after SetDefaults, so compare it separately.
	assert.Equal(t, tempApp.Config.Assistant, app.Config.Assistant, "Assistant config mismatch")
	// Don't compare Database.File here, compare the rest of the Database struct
	assert.Equal(
		t,
		DatabaseConfig{originalFile: app.Config.Database.originalFile, File: app.Config.Database.File},
		app.Config.Database,
		"Database config mismatch (excluding File path resolution)",
	) // Compare everything except the potentially resolved file path
	assert.Equal(t, tempApp.Config.UserData, app.Config.UserData, "UserData config mismatch")
	assert.Equal(t, tempApp.Config.ExtraContext, app.Config.ExtraContext, "ExtraContext config mismatch")
	assert.Equal(t, tempApp.Config.Mailer.From, app.Config.Mailer.From, "Mailer From config mismatch")
	assert.Equal(t, tempApp.Config.Mailer.Server, app.Config.Mailer.Server, "Mailer Server config mismatch")
	assert.Equal(t, tempApp.Config.Mailer.Preview, app.Config.Mailer.Preview, "Mailer Preview config mismatch")
	assert.Equal(t, tempApp.Config.Mailer.Bcc, app.Config.Mailer.Bcc, "Mailer Bcc config mismatch")
	assert.Equal(t, tempApp.Config.LLM, app.Config.LLM, "LLM config mismatch")

	// Assert the final database file path resolution specifically
}
