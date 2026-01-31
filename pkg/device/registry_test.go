package device

import (
	"testing"
)

func TestRegister(t *testing.T) {
	// Note: We can't easily reset the sync.Once, so these tests need to be careful
	// The registry is already initialized when tests run

	// Get initial count
	initial := len(All())

	// Register a mock profile
	mock := &mockProfile{}
	Register(mock)

	// Should have one more
	after := len(All())
	if after != initial+1 {
		t.Errorf("Register: expected %d profiles, got %d", initial+1, after)
	}
}

func TestAll(t *testing.T) {
	profiles := All()

	// Should have at least QTKeJi profile
	found := false
	for _, p := range profiles {
		if p.ID() == "qtkeji" {
			found = true
			break
		}
	}
	if !found {
		t.Error("All() should include qtkeji profile")
	}
}

func TestFindByVIDPID(t *testing.T) {
	tests := []struct {
		vid, pid uint16
		wantID   string
	}{
		{0x1908, 0x0102, "qtkeji"},
		{0x1908, 0x0103, "qtkeji"},
		{0x0000, 0x0000, ""}, // Not found
		{0x1234, 0x5678, ""}, // Not found (unless mock was registered)
	}

	for _, tt := range tests {
		p := FindByVIDPID(tt.vid, tt.pid)
		if tt.wantID == "" {
			// We expect not to find QTKeJi, but might find mock if registered
			if p != nil && p.ID() == "qtkeji" && !(tt.vid == 0x1908) {
				t.Errorf("FindByVIDPID(%04x, %04x) = %v, want nil", tt.vid, tt.pid, p.ID())
			}
		} else {
			if p == nil {
				t.Errorf("FindByVIDPID(%04x, %04x) = nil, want %s", tt.vid, tt.pid, tt.wantID)
			} else if p.ID() != tt.wantID {
				t.Errorf("FindByVIDPID(%04x, %04x) = %s, want %s", tt.vid, tt.pid, p.ID(), tt.wantID)
			}
		}
	}
}

func TestFindByID(t *testing.T) {
	tests := []struct {
		id     string
		wantOK bool
	}{
		{"qtkeji", true},
		{"nonexistent", false},
		{"", false},
	}

	for _, tt := range tests {
		p := FindByID(tt.id)
		if tt.wantOK {
			if p == nil {
				t.Errorf("FindByID(%q) = nil, want profile", tt.id)
			} else if p.ID() != tt.id {
				t.Errorf("FindByID(%q).ID() = %q", tt.id, p.ID())
			}
		} else {
			if p != nil {
				t.Errorf("FindByID(%q) = %v, want nil", tt.id, p.ID())
			}
		}
	}
}

func TestMustFindByVIDPID(t *testing.T) {
	// Known device
	p := MustFindByVIDPID(0x1908, 0x0102)
	if p == nil {
		t.Fatal("MustFindByVIDPID returned nil for known device")
	}
	if p.ID() != "qtkeji" {
		t.Errorf("MustFindByVIDPID(0x1908, 0x0102).ID() = %s, want qtkeji", p.ID())
	}

	// Unknown device - should return generic
	p = MustFindByVIDPID(0xFFFF, 0xFFFF)
	if p == nil {
		t.Fatal("MustFindByVIDPID returned nil for unknown device")
	}
	if p.ID() != "generic" {
		t.Errorf("MustFindByVIDPID(0xFFFF, 0xFFFF).ID() = %s, want generic", p.ID())
	}
}

func TestListProfiles(t *testing.T) {
	profiles := ListProfiles()

	if len(profiles) == 0 {
		t.Error("ListProfiles() returned empty list")
	}

	foundQTKeJi := false
	for _, info := range profiles {
		if info.ID == "qtkeji" {
			foundQTKeJi = true
			if info.Width != 480 || info.Height != 320 {
				t.Errorf("qtkeji profile dimensions: %dx%d, want 480x320", info.Width, info.Height)
			}
		}
	}

	if !foundQTKeJi {
		t.Error("ListProfiles() should include qtkeji")
	}
}

func TestIsKnownDevice(t *testing.T) {
	tests := []struct {
		vid, pid uint16
		want     bool
	}{
		{0x1908, 0x0102, true},
		{0x1908, 0x0103, true},
		{0x0000, 0x0000, false},
		{0xFFFF, 0xFFFF, false},
	}

	for _, tt := range tests {
		got := IsKnownDevice(tt.vid, tt.pid)
		if got != tt.want {
			t.Errorf("IsKnownDevice(%04x, %04x) = %v, want %v", tt.vid, tt.pid, got, tt.want)
		}
	}
}

func TestKnownVIDPIDs(t *testing.T) {
	pairs := KnownVIDPIDs()

	if len(pairs) == 0 {
		t.Error("KnownVIDPIDs() returned empty list")
	}

	// Should include QTKeJi pairs
	foundQTKeJi := false
	for _, pair := range pairs {
		if pair[0] == 0x1908 && (pair[1] == 0x0102 || pair[1] == 0x0103) {
			foundQTKeJi = true
			break
		}
	}

	if !foundQTKeJi {
		t.Error("KnownVIDPIDs() should include QTKeJi VID/PID pairs")
	}
}

func TestFormatVIDPID(t *testing.T) {
	tests := []struct {
		vid, pid uint16
		want     string
	}{
		{0x1908, 0x0102, "1908:0102"},
		{0x0000, 0x0000, "0000:0000"},
		{0xFFFF, 0xFFFF, "ffff:ffff"},
		{0x1234, 0x5678, "1234:5678"},
	}

	for _, tt := range tests {
		got := FormatVIDPID(tt.vid, tt.pid)
		if got != tt.want {
			t.Errorf("FormatVIDPID(%04x, %04x) = %q, want %q", tt.vid, tt.pid, got, tt.want)
		}
	}
}
