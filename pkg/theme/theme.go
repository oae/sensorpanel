// Package theme manages theme discovery, creation, and serving.
package theme

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/oae/sensorpanel/pkg/paths"
)

// ErrThemeNotFound is returned when a theme doesn't exist.
var ErrThemeNotFound = errors.New("theme not found")

// ErrThemeExists is returned when trying to create a theme that already exists.
var ErrThemeExists = errors.New("theme already exists")

// Metadata represents theme metadata from package.json.
type Metadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	Width       int    `json:"width,omitempty"`  // Display width (default 480)
	Height      int    `json:"height,omitempty"` // Display height (default 320)
}

// Theme represents an installed theme.
type Theme struct {
	Name     string   // Theme directory name
	Path     string   // Full path to theme directory
	Metadata Metadata // Parsed metadata from package.json
	HasDist  bool     // Whether dist/index.html exists (built)
	HasSrc   bool     // Whether src/ directory exists
}

// DefaultMetadata returns default theme metadata.
func DefaultMetadata(name string) Metadata {
	return Metadata{
		Name:        name,
		Version:     "1.0.0",
		Description: "A sensorpanel theme",
		Width:       480,
		Height:      320,
	}
}

// List returns all installed themes.
func List() ([]Theme, error) {
	themesDir, err := paths.ThemesDir()
	if err != nil {
		return nil, err
	}

	// If themes directory doesn't exist, return empty list
	if _, err := os.Stat(themesDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read themes directory: %w", err)
	}

	var themes []Theme
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		theme, err := Load(entry.Name())
		if err != nil {
			// Skip invalid themes
			continue
		}
		themes = append(themes, *theme)
	}

	return themes, nil
}

// Load loads a theme by name.
func Load(name string) (*Theme, error) {
	themeDir, err := paths.ThemeDir(name)
	if err != nil {
		return nil, err
	}

	// Check if theme directory exists
	info, err := os.Stat(themeDir)
	if os.IsNotExist(err) {
		return nil, ErrThemeNotFound
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrThemeNotFound
	}

	theme := &Theme{
		Name: name,
		Path: themeDir,
	}

	// Try to load package.json
	pkgPath := filepath.Join(themeDir, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		if err := json.Unmarshal(data, &theme.Metadata); err != nil {
			// Invalid package.json, use defaults
			theme.Metadata = DefaultMetadata(name)
		}
	} else {
		theme.Metadata = DefaultMetadata(name)
	}

	// Set defaults for dimensions if not specified
	if theme.Metadata.Width == 0 {
		theme.Metadata.Width = 480
	}
	if theme.Metadata.Height == 0 {
		theme.Metadata.Height = 320
	}

	// Check for dist/index.html
	distIndex := filepath.Join(themeDir, "dist", "index.html")
	if _, err := os.Stat(distIndex); err == nil {
		theme.HasDist = true
	}

	// Check for src/
	srcDir := filepath.Join(themeDir, "src")
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		theme.HasSrc = true
	}

	return theme, nil
}

// Exists checks if a theme exists.
func Exists(name string) bool {
	_, err := Load(name)
	return err == nil
}

// IndexPath returns the path to the theme's index.html (dist/index.html).
func (t *Theme) IndexPath() string {
	return filepath.Join(t.Path, "dist", "index.html")
}

// DistDir returns the path to the theme's dist directory.
func (t *Theme) DistDir() string {
	return filepath.Join(t.Path, "dist")
}

// Delete removes a theme.
func Delete(name string) error {
	themeDir, err := paths.ThemeDir(name)
	if err != nil {
		return err
	}

	if _, err := os.Stat(themeDir); os.IsNotExist(err) {
		return ErrThemeNotFound
	}

	return os.RemoveAll(themeDir)
}

// Create creates a new theme from the embedded template.
func Create(name string) (*Theme, error) {
	if Exists(name) {
		return nil, ErrThemeExists
	}

	themeDir, err := paths.ThemeDir(name)
	if err != nil {
		return nil, err
	}

	// Create theme directory
	if err := os.MkdirAll(themeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create theme directory: %w", err)
	}

	// Write template files
	if err := writeTemplateFiles(themeDir, name); err != nil {
		// Clean up on failure
		os.RemoveAll(themeDir)
		return nil, fmt.Errorf("failed to write template files: %w", err)
	}

	return Load(name)
}

// writeTemplateFiles writes the React template files to the theme directory.
func writeTemplateFiles(themeDir, themeName string) error {
	// Create directories
	dirs := []string{"src", "dist", "public"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(themeDir, dir), 0755); err != nil {
			return err
		}
	}

	// Write files
	files := getTemplateFiles(themeName)
	for path, content := range files {
		fullPath := filepath.Join(themeDir, path)

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return err
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// WalkDistFiles walks all files in the theme's dist directory.
func (t *Theme) WalkDistFiles(fn func(path string, d fs.DirEntry) error) error {
	distDir := t.DistDir()
	return filepath.WalkDir(distDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		return fn(path, d)
	})
}

// UpdateSDK updates the sensorpanel SDK files in a theme to the latest version.
func UpdateSDK(name string) error {
	theme, err := Load(name)
	if err != nil {
		return err
	}

	// SDK files to update
	sdkFiles := map[string]string{
		"lib/sensorpanel/index.ts":  sdkIndexTS(),
		"lib/sensorpanel/types.ts":  sdkTypesTS(),
		"lib/sensorpanel/client.ts": sdkClientTS(),
		"lib/sensorpanel/hooks.ts":  sdkHooksTS(),
	}

	// Ensure lib/sensorpanel directory exists
	sdkDir := filepath.Join(theme.Path, "lib", "sensorpanel")
	if err := os.MkdirAll(sdkDir, 0755); err != nil {
		return fmt.Errorf("failed to create SDK directory: %w", err)
	}

	// Write each SDK file
	for relPath, content := range sdkFiles {
		fullPath := filepath.Join(theme.Path, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", relPath, err)
		}
	}

	return nil
}
