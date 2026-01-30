package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/alperen/sensorpanel/pkg/paths"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove all configs, data, and cache",
	Long: `Remove sensorpanel config and cache from your system.

This will delete:
  - Config:  ~/.config/sensorpanel/  (device selection, settings)
  - Cache:   ~/.cache/sensorpanel/   (browser binary)

Themes are kept by default. Use --all to also remove themes:
  - Data:    ~/.local/share/sensorpanel/ (themes)

Use --dry-run to see what would be deleted without removing anything.`,
	RunE: runPrune,
}

var (
	pruneDryRun  bool
	pruneAllData bool
)

func init() {
	rootCmd.AddCommand(pruneCmd)
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "Show what would be deleted without removing")
	pruneCmd.Flags().BoolVar(&pruneAllData, "all", false, "Also remove themes (by default themes are kept)")
}

func runPrune(cmd *cobra.Command, args []string) error {
	configDir, err := paths.ConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config dir: %w", err)
	}

	dataDir, err := paths.DataDir()
	if err != nil {
		return fmt.Errorf("failed to get data dir: %w", err)
	}

	cacheDir, err := paths.CacheDir()
	if err != nil {
		return fmt.Errorf("failed to get cache dir: %w", err)
	}

	// Build list of directories to remove
	type dirInfo struct {
		path        string
		description string
	}

	dirs := []dirInfo{
		{configDir, "Config (device selection, settings)"},
		{cacheDir, "Cache (browser binary)"},
	}

	if pruneAllData {
		dirs = append([]dirInfo{{dataDir, "Data (themes)"}}, dirs...)
	}

	// Check what exists
	var toRemove []dirInfo
	for _, d := range dirs {
		if _, err := os.Stat(d.path); err == nil {
			toRemove = append(toRemove, d)
		}
	}

	if len(toRemove) == 0 {
		fmt.Println("Nothing to remove - no sensorpanel directories found.")
		return nil
	}

	// Show what will be removed
	if pruneDryRun {
		fmt.Println("Would remove:")
	} else {
		fmt.Println("Removing:")
	}

	for _, d := range toRemove {
		size, _ := dirSize(d.path)
		fmt.Printf("  %s\n", d.path)
		fmt.Printf("    %s (%s)\n", d.description, formatSize(size))
	}

	if pruneDryRun {
		fmt.Println("\nRun without --dry-run to remove these directories.")
		return nil
	}

	// Actually remove
	var errors []error
	for _, d := range toRemove {
		if err := os.RemoveAll(d.path); err != nil {
			errors = append(errors, fmt.Errorf("failed to remove %s: %w", d.path, err))
		}
	}

	if len(errors) > 0 {
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "Error: %v\n", e)
		}
		return fmt.Errorf("some directories could not be removed")
	}

	fmt.Println("\nDone. All sensorpanel data has been removed.")
	return nil
}

// dirSize calculates the total size of a directory recursively.
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// formatSize formats bytes as human-readable string.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
