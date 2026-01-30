package theme

import (
	"os"
	"path/filepath"
	"testing"
)

// Package Manager Tests

func TestPackageManager_String(t *testing.T) {
	tests := []struct {
		pm   PackageManager
		want string
	}{
		{NPM, "npm"},
		{Yarn, "yarn"},
		{PNPM, "pnpm"},
		{Bun, "bun"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.pm.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPackageManager_Executable(t *testing.T) {
	tests := []struct {
		pm   PackageManager
		want string
	}{
		{NPM, "npm"},
		{Yarn, "yarn"},
		{PNPM, "pnpm"},
		{Bun, "bun"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.pm.Executable(); got != tt.want {
				t.Errorf("Executable() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPackageManager_InstallCmd(t *testing.T) {
	tests := []struct {
		pm   PackageManager
		want []string
	}{
		{NPM, []string{"npm", "install"}},
		{Yarn, []string{"yarn", "install"}},
		{PNPM, []string{"pnpm", "install"}},
		{Bun, []string{"bun", "install"}},
	}

	for _, tt := range tests {
		t.Run(tt.pm.String(), func(t *testing.T) {
			got := tt.pm.InstallCmd()
			if len(got) != len(tt.want) {
				t.Errorf("InstallCmd() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("InstallCmd()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPackageManager_DevCmd(t *testing.T) {
	tests := []struct {
		pm   PackageManager
		want []string
	}{
		{NPM, []string{"npm", "run", "dev"}},
		{Yarn, []string{"yarn", "dev"}},
		{PNPM, []string{"pnpm", "run", "dev"}},
		{Bun, []string{"bun", "run", "dev"}},
	}

	for _, tt := range tests {
		t.Run(tt.pm.String(), func(t *testing.T) {
			got := tt.pm.DevCmd()
			if len(got) != len(tt.want) {
				t.Errorf("DevCmd() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("DevCmd()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPackageManager_BuildCmd(t *testing.T) {
	tests := []struct {
		pm   PackageManager
		want []string
	}{
		{NPM, []string{"npm", "run", "build"}},
		{Yarn, []string{"yarn", "build"}},
		{PNPM, []string{"pnpm", "run", "build"}},
		{Bun, []string{"bun", "run", "build"}},
	}

	for _, tt := range tests {
		t.Run(tt.pm.String(), func(t *testing.T) {
			got := tt.pm.BuildCmd()
			if len(got) != len(tt.want) {
				t.Errorf("BuildCmd() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("BuildCmd()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDetectPackageManager_Lockfiles(t *testing.T) {
	tests := []struct {
		name     string
		lockfile string
		want     PackageManager
	}{
		{"bun.lockb", "bun.lockb", Bun},
		{"pnpm-lock.yaml", "pnpm-lock.yaml", PNPM},
		{"yarn.lock", "yarn.lock", Yarn},
		{"package-lock.json", "package-lock.json", NPM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			// Create lockfile
			os.WriteFile(filepath.Join(tmpDir, tt.lockfile), []byte(""), 0644)

			got := DetectPackageManager(tmpDir)
			if got != tt.want {
				t.Errorf("DetectPackageManager() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectPackageManager_PackageJsonField(t *testing.T) {
	tests := []struct {
		name           string
		packageManager string
		want           PackageManager
	}{
		{"npm with version", "npm@10.0.0", NPM},
		{"yarn with version", "yarn@4.0.0", Yarn},
		{"pnpm with version", "pnpm@8.0.0", PNPM},
		{"bun with version", "bun@1.0.0", Bun},
		{"npm bare", "npm", NPM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			pkgJSON := `{"packageManager": "` + tt.packageManager + `"}`
			os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644)

			got := DetectPackageManager(tmpDir)
			if got != tt.want {
				t.Errorf("DetectPackageManager() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectPackageManager_Priority(t *testing.T) {
	// bun.lockb should take priority over other lockfiles
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "bun.lockb"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "yarn.lock"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "package-lock.json"), []byte(""), 0644)

	got := DetectPackageManager(tmpDir)
	if got != Bun {
		t.Errorf("DetectPackageManager() = %v, want Bun (priority)", got)
	}
}

func TestDetectPackageManager_Default(t *testing.T) {
	tmpDir := t.TempDir()
	// No lockfiles, no package.json

	got := DetectPackageManager(tmpDir)
	if got != NPM {
		t.Errorf("DetectPackageManager() = %v, want NPM (default)", got)
	}
}

func TestHasNodeModules(t *testing.T) {
	t.Run("with node_modules", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, "node_modules"), 0755)

		if !HasNodeModules(tmpDir) {
			t.Error("HasNodeModules() = false, want true")
		}
	})

	t.Run("without node_modules", func(t *testing.T) {
		tmpDir := t.TempDir()

		if HasNodeModules(tmpDir) {
			t.Error("HasNodeModules() = true, want false")
		}
	})

	t.Run("node_modules is a file", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "node_modules"), []byte(""), 0644)

		if HasNodeModules(tmpDir) {
			t.Error("HasNodeModules() = true for file, want false")
		}
	})
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("test"), 0644)

	if !fileExists(filePath) {
		t.Error("fileExists() = false for existing file")
	}

	// Directory should return false
	if fileExists(tmpDir) {
		t.Error("fileExists() = true for directory")
	}

	// Non-existent file
	if fileExists(filepath.Join(tmpDir, "nonexistent.txt")) {
		t.Error("fileExists() = true for non-existent file")
	}
}

func TestDirExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subDir, 0755)

	if !dirExists(subDir) {
		t.Error("dirExists() = false for existing directory")
	}

	// File should return false
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("test"), 0644)
	if dirExists(filePath) {
		t.Error("dirExists() = true for file")
	}

	// Non-existent directory
	if dirExists(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("dirExists() = true for non-existent directory")
	}
}

// Theme Tests

func TestDefaultMetadata(t *testing.T) {
	meta := DefaultMetadata("my-theme")

	if meta.Name != "my-theme" {
		t.Errorf("Name = %q, want my-theme", meta.Name)
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", meta.Version)
	}
	if meta.Width != 480 {
		t.Errorf("Width = %d, want 480", meta.Width)
	}
	if meta.Height != 320 {
		t.Errorf("Height = %d, want 320", meta.Height)
	}
}

func TestTheme_IndexPath(t *testing.T) {
	theme := &Theme{
		Path: "/home/user/themes/my-theme",
	}

	expected := "/home/user/themes/my-theme/dist/index.html"
	if got := theme.IndexPath(); got != expected {
		t.Errorf("IndexPath() = %q, want %q", got, expected)
	}
}

func TestTheme_DistDir(t *testing.T) {
	theme := &Theme{
		Path: "/home/user/themes/my-theme",
	}

	expected := "/home/user/themes/my-theme/dist"
	if got := theme.DistDir(); got != expected {
		t.Errorf("DistDir() = %q, want %q", got, expected)
	}
}

func TestList_EmptyThemesDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Themes directory doesn't exist yet
	themes, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(themes) != 0 {
		t.Errorf("List() returned %d themes, want 0", len(themes))
	}
}

func TestList_WithThemes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create themes directory with a theme
	themesDir := filepath.Join(tmpDir, "sensorpanel", "themes")
	themeDir := filepath.Join(themesDir, "test-theme")
	os.MkdirAll(themeDir, 0755)

	// Create package.json
	pkgJSON := `{"name": "test-theme", "version": "1.0.0"}`
	os.WriteFile(filepath.Join(themeDir, "package.json"), []byte(pkgJSON), 0644)

	themes, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(themes) != 1 {
		t.Errorf("List() returned %d themes, want 1", len(themes))
	}
	if themes[0].Name != "test-theme" {
		t.Errorf("Theme name = %q, want test-theme", themes[0].Name)
	}
}

func TestLoad_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	_, err := Load("nonexistent-theme")
	if err != ErrThemeNotFound {
		t.Errorf("Load() error = %v, want ErrThemeNotFound", err)
	}
}

func TestLoad_Success(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create theme directory structure
	themesDir := filepath.Join(tmpDir, "sensorpanel", "themes")
	themeDir := filepath.Join(themesDir, "test-theme")
	os.MkdirAll(filepath.Join(themeDir, "src"), 0755)
	os.MkdirAll(filepath.Join(themeDir, "dist"), 0755)

	// Create package.json with custom dimensions
	pkgJSON := `{"name": "test-theme", "version": "2.0.0", "width": 800, "height": 480}`
	os.WriteFile(filepath.Join(themeDir, "package.json"), []byte(pkgJSON), 0644)

	// Create dist/index.html
	os.WriteFile(filepath.Join(themeDir, "dist", "index.html"), []byte("<html></html>"), 0644)

	theme, err := Load("test-theme")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if theme.Name != "test-theme" {
		t.Errorf("Name = %q, want test-theme", theme.Name)
	}
	if theme.Metadata.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", theme.Metadata.Version)
	}
	if theme.Metadata.Width != 800 {
		t.Errorf("Width = %d, want 800", theme.Metadata.Width)
	}
	if theme.Metadata.Height != 480 {
		t.Errorf("Height = %d, want 480", theme.Metadata.Height)
	}
	if !theme.HasDist {
		t.Error("HasDist = false, want true")
	}
	if !theme.HasSrc {
		t.Error("HasSrc = false, want true")
	}
}

func TestLoad_DefaultDimensions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create theme without dimensions in package.json
	themesDir := filepath.Join(tmpDir, "sensorpanel", "themes")
	themeDir := filepath.Join(themesDir, "test-theme")
	os.MkdirAll(themeDir, 0755)

	pkgJSON := `{"name": "test-theme"}`
	os.WriteFile(filepath.Join(themeDir, "package.json"), []byte(pkgJSON), 0644)

	theme, err := Load("test-theme")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should use defaults
	if theme.Metadata.Width != 480 {
		t.Errorf("Width = %d, want 480 (default)", theme.Metadata.Width)
	}
	if theme.Metadata.Height != 320 {
		t.Errorf("Height = %d, want 320 (default)", theme.Metadata.Height)
	}
}

func TestLoad_InvalidPackageJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	themesDir := filepath.Join(tmpDir, "sensorpanel", "themes")
	themeDir := filepath.Join(themesDir, "test-theme")
	os.MkdirAll(themeDir, 0755)

	// Invalid JSON
	os.WriteFile(filepath.Join(themeDir, "package.json"), []byte("{invalid json}"), 0644)

	theme, err := Load("test-theme")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should use defaults
	if theme.Metadata.Name != "test-theme" {
		t.Errorf("Name = %q, want test-theme (default)", theme.Metadata.Name)
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create a theme
	themesDir := filepath.Join(tmpDir, "sensorpanel", "themes")
	themeDir := filepath.Join(themesDir, "existing-theme")
	os.MkdirAll(themeDir, 0755)
	os.WriteFile(filepath.Join(themeDir, "package.json"), []byte("{}"), 0644)

	if !Exists("existing-theme") {
		t.Error("Exists() = false for existing theme")
	}
	if Exists("nonexistent-theme") {
		t.Error("Exists() = true for non-existent theme")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create a theme
	themesDir := filepath.Join(tmpDir, "sensorpanel", "themes")
	themeDir := filepath.Join(themesDir, "delete-me")
	os.MkdirAll(themeDir, 0755)
	os.WriteFile(filepath.Join(themeDir, "package.json"), []byte("{}"), 0644)

	// Delete it
	err := Delete("delete-me")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Should be gone
	if Exists("delete-me") {
		t.Error("Theme still exists after Delete()")
	}
}

func TestDelete_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	err := Delete("nonexistent")
	if err != ErrThemeNotFound {
		t.Errorf("Delete() error = %v, want ErrThemeNotFound", err)
	}
}

func TestCreate(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	theme, err := Create("new-theme")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if theme.Name != "new-theme" {
		t.Errorf("Name = %q, want new-theme", theme.Name)
	}

	// Check that files were created
	themeDir := theme.Path
	if _, err := os.Stat(filepath.Join(themeDir, "package.json")); err != nil {
		t.Error("package.json not created")
	}
	if _, err := os.Stat(filepath.Join(themeDir, "src")); err != nil {
		t.Error("src/ not created")
	}
}

func TestCreate_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create first theme
	Create("existing-theme")

	// Try to create again
	_, err := Create("existing-theme")
	if err != ErrThemeExists {
		t.Errorf("Create() error = %v, want ErrThemeExists", err)
	}
}

func TestTheme_WalkDistFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create theme with dist files
	themesDir := filepath.Join(tmpDir, "sensorpanel", "themes")
	themeDir := filepath.Join(themesDir, "test-theme")
	distDir := filepath.Join(themeDir, "dist")
	os.MkdirAll(distDir, 0755)

	// Create some files
	os.WriteFile(filepath.Join(distDir, "index.html"), []byte(""), 0644)
	os.WriteFile(filepath.Join(distDir, "style.css"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(distDir, "assets"), 0755)
	os.WriteFile(filepath.Join(distDir, "assets", "main.js"), []byte(""), 0644)

	os.WriteFile(filepath.Join(themeDir, "package.json"), []byte("{}"), 0644)

	theme, _ := Load("test-theme")

	var files []string
	err := theme.WalkDistFiles(func(path string, d os.DirEntry) error {
		files = append(files, filepath.Base(path))
		return nil
	})

	if err != nil {
		t.Fatalf("WalkDistFiles() error = %v", err)
	}

	if len(files) != 3 {
		t.Errorf("Found %d files, want 3", len(files))
	}
}
