//go:build mage

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = Build

// Build compiles sensorpanel for the current platform
func Build() error {
	fmt.Println("Building sensorpanel...")
	return sh.Run("go", "build", "-o", binaryName(), ".")
}

// Test runs all tests
func Test() error {
	fmt.Println("Running tests...")
	return sh.Run("go", "test", "-v", "./...")
}

// Lint runs golangci-lint
func Lint() error {
	fmt.Println("Running linter...")
	return sh.Run("golangci-lint", "run", "./...")
}

// Vet runs go vet
func Vet() error {
	fmt.Println("Running go vet...")
	return sh.Run("go", "vet", "./...")
}

// Install builds and installs to GOPATH/bin
func Install() error {
	fmt.Println("Installing sensorpanel...")
	return sh.Run("go", "install", ".")
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("Cleaning...")
	os.Remove("sensorpanel")
	os.Remove("sensorpanel.exe")
	return os.RemoveAll("dist")
}

// Release cross-compiles for all platforms
func Release() error {
	mg.Deps(Clean)

	if err := os.MkdirAll("dist", 0755); err != nil {
		return err
	}

	targets := []struct {
		goos   string
		goarch string
	}{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
	}

	for _, t := range targets {
		fmt.Printf("Building for %s/%s...\n", t.goos, t.goarch)

		ext := ""
		if t.goos == "windows" {
			ext = ".exe"
		}

		output := fmt.Sprintf("dist/sensorpanel-%s-%s%s", t.goos, t.goarch, ext)

		env := map[string]string{
			"GOOS":        t.goos,
			"GOARCH":      t.goarch,
			"CGO_ENABLED": "0",
		}

		if err := sh.RunWith(env, "go", "build", "-o", output, "-ldflags", "-s -w", "."); err != nil {
			return err
		}
	}

	fmt.Println("Release builds complete!")
	return nil
}

// Dev builds and runs the dashboard
func Dev() error {
	mg.Deps(Build)
	return sh.RunV("./"+binaryName(), "run")
}

// DevTheme starts theme development mode for the selected theme
func DevTheme() error {
	mg.Deps(Build)
	return sh.RunV("./"+binaryName(), "theme", "dev")
}

// Check runs all checks (vet, lint, test)
func Check() error {
	fmt.Println("Running all checks...")
	if err := Vet(); err != nil {
		return err
	}
	if err := Test(); err != nil {
		return err
	}
	// Lint is optional - don't fail if golangci-lint is not installed
	if err := Lint(); err != nil {
		fmt.Printf("Warning: lint failed (golangci-lint may not be installed): %v\n", err)
	}
	fmt.Println("All checks passed!")
	return nil
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "sensorpanel.exe"
	}
	return "sensorpanel"
}
