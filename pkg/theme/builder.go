package theme

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Build builds a theme for production.
func Build(themeDir string) error {
	pm := DetectPackageManager(themeDir)

	// Check if package manager is installed
	if !pm.IsInstalled() {
		return fmt.Errorf("package manager %s not found - please install it first", pm)
	}

	// Install dependencies if needed
	if !HasNodeModules(themeDir) {
		fmt.Printf("Installing dependencies with %s...\n", pm)
		cmdArgs := pm.InstallCmd()
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = themeDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}
	}

	// Run build command
	fmt.Printf("Building theme with %s...\n", pm)
	cmdArgs := pm.BuildCmd()
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = themeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Verify dist/index.html exists
	distIndex := filepath.Join(themeDir, "dist", "index.html")
	if _, err := os.Stat(distIndex); os.IsNotExist(err) {
		return fmt.Errorf("build did not produce dist/index.html")
	}

	fmt.Println("Build complete!")
	return nil
}

// IsOutdated checks if the theme's dist is older than its source files.
// Returns true if src/ has files newer than dist/index.html.
func IsOutdated(themeDir string) bool {
	distIndex := filepath.Join(themeDir, "dist", "index.html")
	distInfo, err := os.Stat(distIndex)
	if os.IsNotExist(err) {
		// No dist = outdated (needs build)
		return true
	}
	if err != nil {
		return false
	}

	distMtime := distInfo.ModTime()

	// Check src/ directory for any file newer than dist
	srcDir := filepath.Join(themeDir, "src")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return false // No src = can't be outdated
	}

	outdated := false
	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if info.ModTime().After(distMtime) {
			outdated = true
			return filepath.SkipAll
		}
		return nil
	})

	// Also check lib/ directory for SDK changes
	libDir := filepath.Join(themeDir, "lib")
	if _, err := os.Stat(libDir); err == nil {
		filepath.Walk(libDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if info.ModTime().After(distMtime) {
				outdated = true
				return filepath.SkipAll
			}
			return nil
		})
	}

	return outdated
}

// NeedsBuild checks if the theme needs to be built.
// Returns true if dist/index.html doesn't exist or is outdated.
func NeedsBuild(themeDir string) bool {
	distIndex := filepath.Join(themeDir, "dist", "index.html")
	if _, err := os.Stat(distIndex); os.IsNotExist(err) {
		return true
	}
	return IsOutdated(themeDir)
}
