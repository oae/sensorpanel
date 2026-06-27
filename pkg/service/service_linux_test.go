//go:build linux

package service

import (
	"strings"
	"testing"
)

func TestGenerateServiceFileIncludesRunArguments(t *testing.T) {
	content, err := generateServiceFile([]string{
		"--music",
		"--interval", "0.5",
		"--opt", "disk.mounts=/,/home",
	})
	if err != nil {
		t.Fatalf("generateServiceFile() error = %v", err)
	}

	for _, expected := range []string{
		` run "--music"`,
		` "--interval" "0.5"`,
		` "--opt" "disk.mounts=/,/home"`,
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("service file does not contain %q:\n%s", expected, content)
		}
	}
}
