// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	envPrefix      = "KARTA"
	configEnvVar   = envPrefix + "_CONFIG"
	configDirName  = ".karta"
	configFileName = "config.yaml"
)

type Config struct {
	Output string `mapstructure:"output" yaml:"output" json:"output"`
}

// config is the single source of truth set by PersistentPreRunE before each RunE.
var config *Config

// loadConfig builds a viper instance holding config file and environment values.
func loadConfig(flagPath string) (*viper.Viper, error) {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	path, explicit := configFilePath(flagPath)
	if path == "" {
		return v, nil
	}
	v.SetConfigFile(path)

	err := v.ReadInConfig()
	switch {
	case err == nil:
		return v, nil
	case errors.Is(err, os.ErrNotExist) && !explicit:
		// The default config file is optional; environment values still apply.
		return v, nil
	default:
		return nil, fmt.Errorf("config file %s: %w", path, err)
	}
}

// configFilePath returns the config file path and whether it was named explicitly.
func configFilePath(flagPath string) (path string, explicit bool) {
	if flagPath != "" {
		return flagPath, true
	}
	if env := os.Getenv(configEnvVar); env != "" {
		return env, true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(home, configDirName, configFileName), false
}

// buildConfig loads config and env, binds pflags, and unmarshals into Config.
func buildConfig(cmd *cobra.Command) (*Config, error) {
	configPath, err := cmd.Root().PersistentFlags().GetString(flagConfig)
	if err != nil {
		return nil, err
	}
	v, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	if f := cmd.Root().PersistentFlags().Lookup(flagOutput); f != nil {
		if err := v.BindPFlag(flagOutput, f); err != nil {
			return nil, err
		}
	}

	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if f := cmd.Root().PersistentFlags().Lookup(flagOutput); f != nil {
		// Reached for a value from config or the environment; the same value
		// given as a flag is rejected by pflag before this runs.
		if err := f.Value.Set(cfg.Output); err != nil {
			return nil, usageError(cmd, fmt.Errorf("output: %w", err))
		}
	}
	return &cfg, nil
}
