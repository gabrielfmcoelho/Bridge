package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	configPath string
	staticFS   embed.FS
)

func SetStaticFS(fs embed.FS) {
	staticFS = fs
}

var rootCmd = &cobra.Command{
	Use:   "sshcm",
	Short: "SSHCM — IT Asset & SSH Config Management Platform",
}

func init() {
	// Load .env file if present (does not override existing env vars).
	_ = godotenv.Load()

	defaultConfig := filepath.Join(homeDir(), ".ssh", "config")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", defaultConfig, "path to SSH config file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return os.Getenv("HOME")
	}
	return h
}

func sshcmConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "sshcm")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sshcm")
}
