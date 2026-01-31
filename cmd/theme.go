// cmd/theme.go - Theme management commands
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/oae/sensorpanel/pkg/browser"
	"github.com/oae/sensorpanel/pkg/config"
	"github.com/oae/sensorpanel/pkg/paths"
	"github.com/oae/sensorpanel/pkg/theme"
	"github.com/spf13/cobra"
)

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage display themes",
	Long: `Commands for creating, listing, and selecting themes.

Themes are web-based (React/HTML/CSS) and rendered using a headless browser.
Theme files are stored in ~/.local/share/sensorpanel/themes/`,
}

var themeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed themes",
	RunE: func(cmd *cobra.Command, args []string) error {
		themes, err := theme.List()
		if err != nil {
			return fmt.Errorf("failed to list themes: %w", err)
		}

		// Get current theme from config
		currentTheme, _ := config.GetTheme()

		if len(themes) == 0 {
			fmt.Println("No themes installed.")
			fmt.Println("Use 'sensorpanel theme create <name>' to create one.")
			return nil
		}

		fmt.Println("=== Installed Themes ===")
		for _, t := range themes {
			marker := "  "
			if t.Name == currentTheme {
				marker = "* "
			}
			status := ""
			if !t.HasDist {
				status = " (not built)"
			}
			fmt.Printf("%s%s - %s%s\n", marker, t.Name, t.Metadata.Description, status)
		}
		fmt.Println()
		fmt.Println("* = active theme")

		return nil
	},
}

var themeCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new theme from template",
	Long: `Create a new theme with React template.

This creates a new theme directory with:
  - package.json  - Theme metadata and dependencies
  - src/          - React source files
  - dist/         - Pre-built output (works immediately)
  - vite.config.js - Build configuration

You can customize the theme by editing files in src/ and running 'npm run build'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		fmt.Printf("Creating theme '%s'...\n", name)
		t, err := theme.Create(name)
		if err != nil {
			if err == theme.ErrThemeExists {
				return fmt.Errorf("theme '%s' already exists", name)
			}
			return err
		}

		fmt.Printf("Theme created at: %s\n", t.Path)
		fmt.Println()
		fmt.Println("The theme is ready to use with the pre-built template.")
		fmt.Println()
		fmt.Println("To customize:")
		fmt.Printf("  cd %s\n", t.Path)
		fmt.Println("  npm install      # Install dependencies")
		fmt.Println("  npm run dev      # Start development server")
		fmt.Println("  npm run build    # Build for production")
		fmt.Println()
		fmt.Printf("To use this theme: sensorpanel theme select %s\n", name)

		return nil
	},
}

var themeSelectCmd = &cobra.Command{
	Use:   "select <name>",
	Short: "Select a theme to use",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Verify theme exists
		t, err := theme.Load(name)
		if err != nil {
			if err == theme.ErrThemeNotFound {
				return fmt.Errorf("theme '%s' not found (use 'theme list' to see available themes)", name)
			}
			return err
		}

		if !t.HasDist {
			fmt.Printf("Warning: Theme '%s' has not been built.\n", name)
			fmt.Printf("Run 'cd %s && npm run build' first.\n", t.Path)
		}

		if err := config.SetTheme(name); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Selected theme: %s\n", name)
		return nil
	},
}

var themePreviewCmd = &cobra.Command{
	Use:   "preview [name]",
	Short: "Preview a theme in the browser",
	Long: `Open a theme in the default web browser for preview.

If no name is provided, previews the currently selected theme.
The preview shows mock sensor data that updates in real-time.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var themeName string
		if len(args) > 0 {
			themeName = args[0]
		} else {
			var err error
			themeName, err = config.GetTheme()
			if err != nil || themeName == "" {
				return fmt.Errorf("no theme specified and no theme selected in config")
			}
		}

		t, err := theme.Load(themeName)
		if err != nil {
			return fmt.Errorf("failed to load theme: %w", err)
		}

		if !t.HasDist {
			return fmt.Errorf("theme '%s' not built - run 'npm run build' in theme directory", themeName)
		}

		// Open in browser
		indexPath := t.IndexPath()
		url := "file://" + indexPath

		fmt.Printf("Opening theme preview: %s\n", url)
		return openBrowser(url)
	},
}

var themeDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a theme",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Check if it's the active theme
		currentTheme, _ := config.GetTheme()
		if currentTheme == name {
			fmt.Println("Warning: Deleting the currently active theme.")
			fmt.Println("Run 'sensorpanel theme select <other>' to select another theme.")
		}

		if err := theme.Delete(name); err != nil {
			return err
		}

		// Clear from config if it was active
		if currentTheme == name {
			config.SetTheme("")
		}

		fmt.Printf("Deleted theme: %s\n", name)
		return nil
	},
}

var themePathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show theme directory path",
	RunE: func(cmd *cobra.Command, args []string) error {
		themesDir, err := paths.ThemesDir()
		if err != nil {
			return err
		}
		fmt.Println(themesDir)
		return nil
	},
}

var (
	themeDevNoBrowser bool
	themeDevInterval  float64
	themeDevOpts      []string
)

var themeDevCmd = &cobra.Command{
	Use:   "dev [name]",
	Short: "Start development server with live sensor data",
	Long: `Start a complete theme development environment with one command.

This automatically:
  1. Detects your package manager (npm/yarn/pnpm/bun)
  2. Installs dependencies if needed
  3. Starts a WebSocket server for live sensor data
  4. Starts the Vite dev server with HMR
  5. Opens your browser to the theme

The sensor data will be broadcast via WebSocket and your theme will update in real-time.
Press Ctrl+C to stop all servers.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var themeName string
		if len(args) > 0 {
			themeName = args[0]
		} else {
			// Use current theme from config
			themeName, _ = config.GetTheme()
		}

		if themeName == "" {
			return fmt.Errorf("no theme specified and no theme selected in config\nUse: sensorpanel theme dev <name>")
		}

		// Load theme to get path
		t, err := theme.Load(themeName)
		if err != nil {
			if err == theme.ErrThemeNotFound {
				return fmt.Errorf("theme '%s' not found (use 'theme list' to see available themes)", themeName)
			}
			return fmt.Errorf("failed to load theme '%s': %w", themeName, err)
		}

		fmt.Printf("Starting development server for theme: %s\n", themeName)
		fmt.Printf("Theme path: %s\n\n", t.Path)

		// Load sensor options from config file first
		var sensorOptions map[string]interface{}
		if cfg, err := config.Load(); err == nil && cfg.SensorOptions != nil {
			sensorOptions = make(map[string]interface{})
			for k, v := range cfg.SensorOptions {
				sensorOptions[k] = v
			}
		}

		// Override with --opt flags if provided
		if len(themeDevOpts) > 0 {
			if sensorOptions == nil {
				sensorOptions = make(map[string]interface{})
			}
			for _, opt := range themeDevOpts {
				key, value, ok := strings.Cut(opt, "=")
				if !ok {
					return fmt.Errorf("invalid option format %q (expected key=value)", opt)
				}
				// If value contains commas, treat it as a string slice
				if strings.Contains(value, ",") {
					sensorOptions[key] = strings.Split(value, ",")
				} else {
					sensorOptions[key] = value
				}
			}
		}

		// Create dev server
		devServer := theme.NewDevServer(t.Path)
		devServer.NoBrowser = themeDevNoBrowser
		devServer.SensorOptions = sensorOptions
		if themeDevInterval > 0 {
			devServer.Interval = themeDevInterval
		}

		// Setup signal handling
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			fmt.Println("\nShutting down...")
			cancel()
		}()

		// Start the dev server
		if err := devServer.Start(ctx); err != nil {
			return err
		}
		defer devServer.Stop()

		// Wait for shutdown
		devServer.Wait()
		return nil
	},
}

var themeBuildCmd = &cobra.Command{
	Use:   "build [name]",
	Short: "Build a theme for production",
	Long: `Build a theme for production use.

This runs the theme's build script (usually via Vite) to produce
optimized static files in the dist/ directory.

If no name is provided, builds the currently selected theme.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var themeName string
		if len(args) > 0 {
			themeName = args[0]
		} else {
			themeName, _ = config.GetTheme()
		}

		if themeName == "" {
			return fmt.Errorf("no theme specified and no theme selected in config\nUse: sensorpanel theme build <name>")
		}

		// Load theme to get path
		t, err := theme.Load(themeName)
		if err != nil {
			if err == theme.ErrThemeNotFound {
				return fmt.Errorf("theme '%s' not found (use 'theme list' to see available themes)", themeName)
			}
			return fmt.Errorf("failed to load theme '%s': %w", themeName, err)
		}

		fmt.Printf("Building theme: %s\n", themeName)
		fmt.Printf("Theme path: %s\n\n", t.Path)

		if err := theme.Build(t.Path); err != nil {
			return err
		}

		fmt.Printf("\nTheme '%s' built successfully!\n", themeName)
		fmt.Printf("Output: %s/dist/\n", t.Path)
		return nil
	},
}

var themeBrowserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Manage headless browser for theme rendering",
}

var themeBrowserInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Download headless Chrome for theme rendering",
	Long: `Download Chrome for Testing to render themes.

This downloads a portable Chrome binary to ~/.cache/sensorpanel/browser/
The browser is used to render web-based themes to images for the display.

If you have Chrome/Chromium already installed, this is optional.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if browser.IsInstalled() {
			path, _ := browser.BrowserPath()
			fmt.Printf("Chrome already installed at: %s\n", path)
			fmt.Println("Use 'theme browser remove' to remove it first.")
			return nil
		}

		fmt.Printf("Downloading Chrome for Testing v%s...\n", browser.Version())
		fmt.Println("This may take a few minutes.")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		lastPercent := -1
		err := browser.Download(ctx, func(downloaded, total int64) {
			if total > 0 {
				percent := int(downloaded * 100 / total)
				if percent != lastPercent && percent%10 == 0 {
					fmt.Printf("  %d%% (%d / %d MB)\n", percent, downloaded/1024/1024, total/1024/1024)
					lastPercent = percent
				}
			}
		})
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		path, _ := browser.BrowserPath()
		fmt.Printf("Chrome installed at: %s\n", path)
		return nil
	},
}

var themeBrowserStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check headless browser status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Browser Status ===")

		// Check cached browser
		if path, err := browser.BrowserPath(); err == nil {
			fmt.Printf("Cached:  %s (v%s)\n", path, browser.Version())
		} else {
			fmt.Println("Cached:  Not installed")
		}

		// Check system browser
		if path, err := browser.SystemChromePath(); err == nil {
			fmt.Printf("System:  %s\n", path)
		} else {
			fmt.Println("System:  Not found")
		}

		// Which will be used
		if path, err := browser.GetChromePath(); err == nil {
			fmt.Printf("\nWill use: %s\n", path)
		} else {
			fmt.Println("\nNo browser available!")
			fmt.Println("Run 'sensorpanel theme browser install' to download one.")
		}

		return nil
	},
}

var themeBrowserRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove cached headless browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := browser.Remove(); err != nil {
			return err
		}
		fmt.Println("Cached browser removed.")
		return nil
	},
}

var themeSDKCmd = &cobra.Command{
	Use:   "sdk",
	Short: "Manage theme SDK",
}

var themeSDKUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update theme SDK to latest version",
	Long: `Update the sensorpanel SDK files in a theme to the latest version.

This updates the lib/sensorpanel/ directory with the latest SDK files
from sensorpanel. Use this after upgrading sensorpanel to get bug fixes
and new features in themes.

If no name is provided, updates the currently selected theme.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var themeName string
		if len(args) > 0 {
			themeName = args[0]
		} else {
			themeName, _ = config.GetTheme()
		}

		if themeName == "" {
			return fmt.Errorf("no theme specified and no theme selected in config\nUse: sensorpanel theme sdk update <name>")
		}

		// Verify theme exists
		t, err := theme.Load(themeName)
		if err != nil {
			if err == theme.ErrThemeNotFound {
				return fmt.Errorf("theme '%s' not found (use 'theme list' to see available themes)", themeName)
			}
			return err
		}

		fmt.Printf("Updating SDK for theme: %s\n", themeName)
		fmt.Printf("Theme path: %s\n\n", t.Path)

		if err := theme.UpdateSDK(themeName); err != nil {
			return fmt.Errorf("failed to update SDK: %w", err)
		}

		fmt.Println("SDK updated successfully!")
		fmt.Println()
		fmt.Println("Files updated:")
		fmt.Println("  lib/sensorpanel/index.ts")
		fmt.Println("  lib/sensorpanel/types.ts")
		fmt.Println("  lib/sensorpanel/client.ts")
		fmt.Println("  lib/sensorpanel/hooks.ts")
		fmt.Println()
		fmt.Printf("Run 'sensorpanel theme build %s' to rebuild the theme.\n", themeName)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(themeCmd)

	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeCreateCmd)
	themeCmd.AddCommand(themeSelectCmd)
	themeCmd.AddCommand(themePreviewCmd)
	themeCmd.AddCommand(themeDeleteCmd)
	themeCmd.AddCommand(themePathCmd)
	themeCmd.AddCommand(themeDevCmd)
	themeCmd.AddCommand(themeBuildCmd)
	themeCmd.AddCommand(themeBrowserCmd)
	themeCmd.AddCommand(themeSDKCmd)

	// Flags for theme dev
	themeDevCmd.Flags().BoolVar(&themeDevNoBrowser, "no-browser", false, "Don't open browser automatically")
	themeDevCmd.Flags().Float64VarP(&themeDevInterval, "interval", "i", 1.0, "Sensor update interval in seconds")
	themeDevCmd.Flags().StringSliceVarP(&themeDevOpts, "opt", "o", nil, "Sensor options in key=value format (e.g., disk.mounts=/,/home)")

	themeBrowserCmd.AddCommand(themeBrowserInstallCmd)
	themeBrowserCmd.AddCommand(themeBrowserStatusCmd)
	themeBrowserCmd.AddCommand(themeBrowserRemoveCmd)

	themeSDKCmd.AddCommand(themeSDKUpdateCmd)
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	// Don't wait for browser to close
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
