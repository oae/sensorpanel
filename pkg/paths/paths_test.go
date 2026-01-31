//go:build linux

package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDir(t *testing.T) {
	t.Run("with XDG_CONFIG_HOME set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		dir, err := ConfigDir()
		if err != nil {
			t.Fatalf("ConfigDir() error = %v", err)
		}

		expected := filepath.Join(tmpDir, "sensorpanel")
		if dir != expected {
			t.Errorf("ConfigDir() = %q, want %q", dir, expected)
		}
	})

	t.Run("without XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")

		dir, err := ConfigDir()
		if err != nil {
			t.Fatalf("ConfigDir() error = %v", err)
		}

		// Should end with .config/sensorpanel
		if !strings.HasSuffix(dir, filepath.Join(".config", "sensorpanel")) {
			t.Errorf("ConfigDir() = %q, want suffix %q", dir, filepath.Join(".config", "sensorpanel"))
		}
	})
}

func TestDataDir(t *testing.T) {
	t.Run("with XDG_DATA_HOME set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_DATA_HOME", tmpDir)

		dir, err := DataDir()
		if err != nil {
			t.Fatalf("DataDir() error = %v", err)
		}

		expected := filepath.Join(tmpDir, "sensorpanel")
		if dir != expected {
			t.Errorf("DataDir() = %q, want %q", dir, expected)
		}
	})

	t.Run("without XDG_DATA_HOME", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")

		dir, err := DataDir()
		if err != nil {
			t.Fatalf("DataDir() error = %v", err)
		}

		// Should end with .local/share/sensorpanel
		if !strings.HasSuffix(dir, filepath.Join(".local", "share", "sensorpanel")) {
			t.Errorf("DataDir() = %q, want suffix %q", dir, filepath.Join(".local", "share", "sensorpanel"))
		}
	})
}

func TestCacheDir(t *testing.T) {
	t.Run("with XDG_CACHE_HOME set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_CACHE_HOME", tmpDir)

		dir, err := CacheDir()
		if err != nil {
			t.Fatalf("CacheDir() error = %v", err)
		}

		expected := filepath.Join(tmpDir, "sensorpanel")
		if dir != expected {
			t.Errorf("CacheDir() = %q, want %q", dir, expected)
		}
	})

	t.Run("without XDG_CACHE_HOME", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "")

		dir, err := CacheDir()
		if err != nil {
			t.Fatalf("CacheDir() error = %v", err)
		}

		// Should end with .cache/sensorpanel
		if !strings.HasSuffix(dir, filepath.Join(".cache", "sensorpanel")) {
			t.Errorf("CacheDir() = %q, want suffix %q", dir, filepath.Join(".cache", "sensorpanel"))
		}
	})
}

func TestThemesDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	dir, err := ThemesDir()
	if err != nil {
		t.Fatalf("ThemesDir() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "sensorpanel", "themes")
	if dir != expected {
		t.Errorf("ThemesDir() = %q, want %q", dir, expected)
	}
}

func TestBrowserDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	dir, err := BrowserDir()
	if err != nil {
		t.Fatalf("BrowserDir() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "sensorpanel", "browser")
	if dir != expected {
		t.Errorf("BrowserDir() = %q, want %q", dir, expected)
	}
}

func TestThemeDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	dir, err := ThemeDir("my-theme")
	if err != nil {
		t.Fatalf("ThemeDir() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "sensorpanel", "themes", "my-theme")
	if dir != expected {
		t.Errorf("ThemeDir() = %q, want %q", dir, expected)
	}
}

func TestEnsureDir(t *testing.T) {
	t.Run("creates new directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		newDir := filepath.Join(tmpDir, "new", "nested", "dir")

		err := EnsureDir(newDir)
		if err != nil {
			t.Fatalf("EnsureDir() error = %v", err)
		}

		info, err := os.Stat(newDir)
		if err != nil {
			t.Fatalf("Directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("Path is not a directory")
		}
	})

	t.Run("existing directory succeeds", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := EnsureDir(tmpDir)
		if err != nil {
			t.Errorf("EnsureDir() on existing dir error = %v", err)
		}
	})
}

func TestEnsureThemesDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	dir, err := EnsureThemesDir()
	if err != nil {
		t.Fatalf("EnsureThemesDir() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "sensorpanel", "themes")
	if dir != expected {
		t.Errorf("EnsureThemesDir() = %q, want %q", dir, expected)
	}

	// Verify directory was created
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Path is not a directory")
	}
}

func TestEnsureBrowserDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	dir, err := EnsureBrowserDir()
	if err != nil {
		t.Fatalf("EnsureBrowserDir() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "sensorpanel", "browser")
	if dir != expected {
		t.Errorf("EnsureBrowserDir() = %q, want %q", dir, expected)
	}

	// Verify directory was created
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Path is not a directory")
	}
}

func TestAppName(t *testing.T) {
	// Verify app name is consistent across all functions
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	configDir, _ := ConfigDir()
	dataDir, _ := DataDir()
	cacheDir, _ := CacheDir()

	// All should have sensorpanel as the last component
	if filepath.Base(configDir) != "sensorpanel" {
		t.Errorf("ConfigDir base = %q, want sensorpanel", filepath.Base(configDir))
	}
	if filepath.Base(dataDir) != "sensorpanel" {
		t.Errorf("DataDir base = %q, want sensorpanel", filepath.Base(dataDir))
	}
	if filepath.Base(cacheDir) != "sensorpanel" {
		t.Errorf("CacheDir base = %q, want sensorpanel", filepath.Base(cacheDir))
	}
}
