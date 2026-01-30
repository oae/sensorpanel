package device

import (
	"strings"
	"testing"
)

func TestDeviceSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		spec    DeviceSpec
		wantErr string
	}{
		{
			name:    "empty ID",
			spec:    DeviceSpec{},
			wantErr: "ID is required",
		},
		{
			name: "invalid ID with spaces",
			spec: DeviceSpec{
				ID: "my device",
			},
			wantErr: "ID must be a valid Go identifier",
		},
		{
			name: "invalid ID with uppercase",
			spec: DeviceSpec{
				ID: "MyDevice",
			},
			wantErr: "ID must be a valid Go identifier",
		},
		{
			name: "invalid ID starting with number",
			spec: DeviceSpec{
				ID: "3device",
			},
			wantErr: "ID must be a valid Go identifier",
		},
		{
			name: "empty Name",
			spec: DeviceSpec{
				ID: "mydevice",
			},
			wantErr: "Name is required",
		},
		{
			name: "zero VendorID",
			spec: DeviceSpec{
				ID:   "mydevice",
				Name: "My Device",
			},
			wantErr: "VendorID is required",
		},
		{
			name: "zero ProductID",
			spec: DeviceSpec{
				ID:       "mydevice",
				Name:     "My Device",
				VendorID: 0x1234,
			},
			wantErr: "ProductID is required",
		},
		{
			name: "zero Width",
			spec: DeviceSpec{
				ID:        "mydevice",
				Name:      "My Device",
				VendorID:  0x1234,
				ProductID: 0x5678,
			},
			wantErr: "Width must be positive",
		},
		{
			name: "negative Width",
			spec: DeviceSpec{
				ID:        "mydevice",
				Name:      "My Device",
				VendorID:  0x1234,
				ProductID: 0x5678,
				Width:     -100,
			},
			wantErr: "Width must be positive",
		},
		{
			name: "zero Height",
			spec: DeviceSpec{
				ID:        "mydevice",
				Name:      "My Device",
				VendorID:  0x1234,
				ProductID: 0x5678,
				Width:     480,
			},
			wantErr: "Height must be positive",
		},
		{
			name: "negative Height",
			spec: DeviceSpec{
				ID:        "mydevice",
				Name:      "My Device",
				VendorID:  0x1234,
				ProductID: 0x5678,
				Width:     480,
				Height:    -100,
			},
			wantErr: "Height must be positive",
		},
		{
			name: "valid spec",
			spec: DeviceSpec{
				ID:        "mydevice",
				Name:      "My Device",
				VendorID:  0x1234,
				ProductID: 0x5678,
				Width:     480,
				Height:    320,
			},
			wantErr: "",
		},
		{
			name: "valid spec with underscore",
			spec: DeviceSpec{
				ID:        "my_device_v2",
				Name:      "My Device V2",
				VendorID:  0x1234,
				ProductID: 0x5678,
				Width:     800,
				Height:    480,
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.wantErr)
				}
			}
		})
	}
}

func TestDeviceSpec_StructName(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"mydevice", "MydeviceProfile"},
		{"my_device", "MyDeviceProfile"},
		{"my_cool_device", "MyCoolDeviceProfile"},
		{"qtkeji", "QtkejiProfile"},
		{"device_v2", "DeviceV2Profile"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			spec := DeviceSpec{ID: tt.id}
			if got := spec.StructName(); got != tt.want {
				t.Errorf("StructName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceSpec_ColorFormatStr(t *testing.T) {
	tests := []struct {
		format ColorFormat
		want   string
	}{
		{RGB565, "RGB565"},
		{RGB888, "RGB888"},
		{ColorFormat(99), "RGB565"}, // Unknown defaults to RGB565
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			spec := DeviceSpec{ColorFormat: tt.format}
			if got := spec.ColorFormatStr(); got != tt.want {
				t.Errorf("ColorFormatStr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceSpec_ByteOrderStr(t *testing.T) {
	tests := []struct {
		order ByteOrder
		want  string
	}{
		{BigEndian, "BigEndian"},
		{LittleEndian, "LittleEndian"},
		{ByteOrder(99), "BigEndian"}, // Unknown defaults to BigEndian
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			spec := DeviceSpec{ByteOrder: tt.order}
			if got := spec.ByteOrderStr(); got != tt.want {
				t.Errorf("ByteOrderStr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceSpec_ProtocolTypeStr(t *testing.T) {
	tests := []struct {
		proto ProtocolType
		want  string
	}{
		{ProtocolSCSI, "ProtocolSCSI"},
		{ProtocolBulk, "ProtocolBulk"},
		{ProtocolType(99), "ProtocolSCSI"}, // Unknown defaults to ProtocolSCSI
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			spec := DeviceSpec{ProtocolType: tt.proto}
			if got := spec.ProtocolTypeStr(); got != tt.want {
				t.Errorf("ProtocolTypeStr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceSpec_BufferSize(t *testing.T) {
	tests := []struct {
		name string
		spec DeviceSpec
		want int
	}{
		{
			name: "RGB565 480x320",
			spec: DeviceSpec{
				Width:       480,
				Height:      320,
				ColorFormat: RGB565,
			},
			want: 480 * 320 * 2, // 307200
		},
		{
			name: "RGB888 800x480",
			spec: DeviceSpec{
				Width:       800,
				Height:      480,
				ColorFormat: RGB888,
			},
			want: 800 * 480 * 3, // 1152000
		},
		{
			name: "RGB565 1920x1080",
			spec: DeviceSpec{
				Width:       1920,
				Height:      1080,
				ColorFormat: RGB565,
			},
			want: 1920 * 1080 * 2, // 4147200
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.spec.BufferSize(); got != tt.want {
				t.Errorf("BufferSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"", false},
		{"mydevice", true},
		{"my_device", true},
		{"_private", true},
		{"device123", true},
		{"MyDevice", false},  // uppercase
		{"my-device", false}, // hyphen (not valid Go identifier)
		{"3device", false},   // starts with number
		{"my device", false}, // space
		{"my.device", false}, // dot
		{"_", true},          // single underscore is valid
		{"__init__", true},   // double underscores
		{"a", true},          // single letter
		{"A", false},         // uppercase single letter
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			if got := isValidIdentifier(tt.s); got != tt.want {
				t.Errorf("isValidIdentifier(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		s    string
		want string
	}{
		{"mydevice", "Mydevice"},
		{"my_device", "MyDevice"},
		{"my-device", "MyDevice"},
		{"my_cool_device", "MyCoolDevice"},
		{"already", "Already"},
		{"_leading", "Leading"},
		{"trailing_", "Trailing"},
		{"multi__underscore", "MultiUnderscore"},
		{"mixed-and_case", "MixedAndCase"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			if got := toPascalCase(tt.s); got != tt.want {
				t.Errorf("toPascalCase(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestGenerateProfile(t *testing.T) {
	t.Run("valid spec generates code", func(t *testing.T) {
		spec := DeviceSpec{
			ID:            "test_device",
			Name:          "Test Device",
			Description:   "A test device for unit testing",
			VendorID:      0x1234,
			ProductID:     0x5678,
			Width:         480,
			Height:        320,
			ColorFormat:   RGB565,
			ByteOrder:     BigEndian,
			MaxBrightness: 255,
			ProtocolType:  ProtocolSCSI,
		}

		code, err := GenerateProfile(spec)
		if err != nil {
			t.Fatalf("GenerateProfile() error = %v", err)
		}

		// Check that the generated code contains expected elements
		expectedStrings := []string{
			"package device",
			"type TestDeviceProfile struct{}",
			"func (p *TestDeviceProfile) ID() string",
			`return "test_device"`,
			`return "Test Device"`,
			`return "A test device for unit testing"`,
			"vendorID == 0x1234 && productID == 0x5678",
			"return 480",
			"return 320",
			"return RGB565",
			"return BigEndian",
			"return 307200", // BufferSize
			"return 255",    // MaxBrightness
			"return ProtocolSCSI",
			"return panel.ImageToRGB565BufferBE(img)",
		}

		for _, expected := range expectedStrings {
			if !strings.Contains(code, expected) {
				t.Errorf("Generated code missing expected string: %q", expected)
			}
		}
	})

	t.Run("RGB565 LittleEndian generates correct converter", func(t *testing.T) {
		spec := DeviceSpec{
			ID:            "le_device",
			Name:          "LE Device",
			VendorID:      0x1234,
			ProductID:     0x5678,
			Width:         480,
			Height:        320,
			ColorFormat:   RGB565,
			ByteOrder:     LittleEndian,
			MaxBrightness: 100,
			ProtocolType:  ProtocolSCSI,
		}

		code, err := GenerateProfile(spec)
		if err != nil {
			t.Fatalf("GenerateProfile() error = %v", err)
		}

		if !strings.Contains(code, "return panel.ImageToRGB565BufferLE(img)") {
			t.Error("Expected RGB565 LittleEndian to use ImageToRGB565BufferLE")
		}
	})

	t.Run("RGB888 generates correct converter", func(t *testing.T) {
		spec := DeviceSpec{
			ID:            "rgb888_device",
			Name:          "RGB888 Device",
			VendorID:      0x1234,
			ProductID:     0x5678,
			Width:         800,
			Height:        480,
			ColorFormat:   RGB888,
			ByteOrder:     BigEndian,
			MaxBrightness: 100,
			ProtocolType:  ProtocolBulk,
		}

		code, err := GenerateProfile(spec)
		if err != nil {
			t.Fatalf("GenerateProfile() error = %v", err)
		}

		if !strings.Contains(code, "return panel.ImageToRGB888Buffer(img)") {
			t.Error("Expected RGB888 to use ImageToRGB888Buffer")
		}
		if !strings.Contains(code, "return ProtocolBulk") {
			t.Error("Expected ProtocolBulk in generated code")
		}
	})

	t.Run("invalid spec returns error", func(t *testing.T) {
		spec := DeviceSpec{} // Empty spec is invalid

		_, err := GenerateProfile(spec)
		if err == nil {
			t.Error("GenerateProfile() expected error for invalid spec, got nil")
		}
		if !strings.Contains(err.Error(), "invalid spec") {
			t.Errorf("Expected error to contain 'invalid spec', got: %v", err)
		}
	})

	t.Run("generated code contains correct VID/PID format", func(t *testing.T) {
		spec := DeviceSpec{
			ID:            "vidpid_test",
			Name:          "VID/PID Test",
			VendorID:      0xABCD,
			ProductID:     0xEF01,
			Width:         320,
			Height:        240,
			ColorFormat:   RGB565,
			ByteOrder:     BigEndian,
			MaxBrightness: 50,
			ProtocolType:  ProtocolSCSI,
		}

		code, err := GenerateProfile(spec)
		if err != nil {
			t.Fatalf("GenerateProfile() error = %v", err)
		}

		// Check VID/PID are properly formatted in hex
		if !strings.Contains(code, "0xABCD") {
			t.Error("Expected VendorID to be formatted as 0xABCD")
		}
		if !strings.Contains(code, "0xEF01") {
			t.Error("Expected ProductID to be formatted as 0xEF01")
		}
	})
}
