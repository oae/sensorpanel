package theme

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

// PackageManager represents a Node.js package manager.
type PackageManager string

const (
	// NPM is the npm package manager.
	NPM PackageManager = "npm"
	// Yarn is the yarn package manager.
	Yarn PackageManager = "yarn"
	// PNPM is the pnpm package manager.
	PNPM PackageManager = "pnpm"
	// Bun is the bun package manager/runtime.
	Bun PackageManager = "bun"
)

// String returns the package manager name.
func (pm PackageManager) String() string {
	return string(pm)
}

// DetectPackageManager detects the package manager used in a theme directory.
// Priority order:
//  1. bun.lockb → bun
//  2. pnpm-lock.yaml → pnpm
//  3. yarn.lock → yarn
//  4. package-lock.json → npm
//  5. package.json "packageManager" field
//  6. fallback: npm
func DetectPackageManager(themeDir string) PackageManager {
	// Check lockfiles in priority order
	lockfiles := []struct {
		file string
		pm   PackageManager
	}{
		{"bun.lockb", Bun},
		{"pnpm-lock.yaml", PNPM},
		{"yarn.lock", Yarn},
		{"package-lock.json", NPM},
	}

	for _, lf := range lockfiles {
		if fileExists(filepath.Join(themeDir, lf.file)) {
			return lf.pm
		}
	}

	// Check package.json packageManager field
	pkgPath := filepath.Join(themeDir, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			PackageManager string `json:"packageManager"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.PackageManager != "" {
			// Parse "npm@10.0.0" format
			pm := pkg.PackageManager
			for i, c := range pm {
				if c == '@' {
					pm = pm[:i]
					break
				}
			}
			switch pm {
			case "bun":
				return Bun
			case "pnpm":
				return PNPM
			case "yarn":
				return Yarn
			case "npm":
				return NPM
			}
		}
	}

	// Default to npm
	return NPM
}

// InstallCmd returns the command to install dependencies.
func (pm PackageManager) InstallCmd() []string {
	switch pm {
	case Bun:
		return []string{"bun", "install"}
	case PNPM:
		return []string{"pnpm", "install"}
	case Yarn:
		return []string{"yarn", "install"}
	default:
		return []string{"npm", "install"}
	}
}

// DevCmd returns the command to start the dev server.
func (pm PackageManager) DevCmd() []string {
	switch pm {
	case Bun:
		return []string{"bun", "run", "dev"}
	case PNPM:
		return []string{"pnpm", "run", "dev"}
	case Yarn:
		return []string{"yarn", "dev"}
	default:
		return []string{"npm", "run", "dev"}
	}
}

// BuildCmd returns the command to build the theme.
func (pm PackageManager) BuildCmd() []string {
	switch pm {
	case Bun:
		return []string{"bun", "run", "build"}
	case PNPM:
		return []string{"pnpm", "run", "build"}
	case Yarn:
		return []string{"yarn", "build"}
	default:
		return []string{"npm", "run", "build"}
	}
}

// Executable returns the package manager executable name.
func (pm PackageManager) Executable() string {
	return string(pm)
}

// IsInstalled checks if the package manager is installed.
func (pm PackageManager) IsInstalled() bool {
	_, err := exec.LookPath(pm.Executable())
	return err == nil
}

// HasNodeModules checks if node_modules exists in the theme directory.
func HasNodeModules(themeDir string) bool {
	return dirExists(filepath.Join(themeDir, "node_modules"))
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists checks if a directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
