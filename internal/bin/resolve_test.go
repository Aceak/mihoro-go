package bin

import (
	"testing"

	"mihoro-go/internal/config"
)

func TestDetectArch(t *testing.T) {
	arch, err := DetectArch()
	if err != nil {
		t.Fatalf("DetectArch() = error %v", err)
	}

	// Verify the detected arch is in the supported list
	found := false
	for _, a := range supportedArchs {
		if a == arch {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("detected arch %q is not in supported list", arch)
	}
}

func TestValidateArch(t *testing.T) {
	valid := []string{"amd64", "amd64-compatible", "amd64-v3", "arm64", "armv7", "riscv64", "loong64-abi2"}
	for _, a := range valid {
		if _, err := ValidateArch(a); err != nil {
			t.Errorf("ValidateArch(%q) should be valid, got: %v", a, err)
		}
	}

	invalid := []string{"invalid", "x86_64", "aarch64"}
	for _, a := range invalid {
		if _, err := ValidateArch(a); err == nil {
			t.Errorf("ValidateArch(%q) should fail", a)
		}
	}
}

func TestValidateArchSuggestions(t *testing.T) {
	_, err := ValidateArch("amd")
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if !containsStr(errStr, "Did you mean") {
		t.Error("expected suggestion in error message")
	}
}

func TestBuildDownloadURL(t *testing.T) {
	url := BuildDownloadURL("v1.19.0", "amd64", config.ChannelStable)
	expected := "https://github.com/MetaCubeX/mihomo/releases/latest/download/mihomo-linux-amd64-v1.19.0.gz"
	if url != expected {
		t.Errorf("got  %s\nwant %s", url, expected)
	}

	urlAlpha := BuildDownloadURL("alpha-abc123", "arm64", config.ChannelAlpha)
	expectedAlpha := "https://github.com/MetaCubeX/mihomo/releases/download/Prerelease-Alpha/mihomo-linux-arm64-alpha-abc123.gz"
	if urlAlpha != expectedAlpha {
		t.Errorf("got  %s\nwant %s", urlAlpha, expectedAlpha)
	}

	urlCompat := BuildDownloadURL("v1.19.0", "amd64-compatible", config.ChannelStable)
	expectedCompat := "https://github.com/MetaCubeX/mihomo/releases/latest/download/mihomo-linux-amd64-compatible-v1.19.0.gz"
	if urlCompat != expectedCompat {
		t.Errorf("got  %s\nwant %s", urlCompat, expectedCompat)
	}
}

func TestValidateArchAllSupported(t *testing.T) {
	// Verify all supported archs pass validation
	for _, a := range supportedArchs {
		if _, err := ValidateArch(a); err != nil {
			t.Errorf("ValidateArch(%q) should be valid: %v", a, err)
		}
	}
}

func TestValidateArchEmptyString(t *testing.T) {
	_, err := ValidateArch("")
	if err == nil {
		t.Error("empty string should be invalid")
	}
}

func TestBuildDownloadURLAllChannels(t *testing.T) {
	url := BuildDownloadURL("v1.0.0", "arm64", config.ChannelStable)
	if !containsStr(url, "latest/download") {
		t.Error("stable should use latest/download")
	}

	url = BuildDownloadURL("alpha-xxx", "arm64", config.ChannelAlpha)
	if !containsStr(url, "Prerelease-Alpha") {
		t.Error("alpha should use Prerelease-Alpha")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
