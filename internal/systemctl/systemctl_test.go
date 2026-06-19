package systemctl

import (
	"strings"
	"testing"
)

func TestRenderServiceStringUserScope(t *testing.T) {
	s := RenderServiceString("/usr/bin/mihomo", "/home/user/.config/mihomo", UserScope)

	if !strings.Contains(s, "ExecStart=/usr/bin/mihomo -d /home/user/.config/mihomo") {
		t.Error("missing ExecStart")
	}
	if !strings.Contains(s, "WantedBy=default.target") {
		t.Error("user scope should use default.target")
	}
	if !strings.Contains(s, "[Unit]") {
		t.Error("missing [Unit] section")
	}
	if !strings.Contains(s, "[Service]") {
		t.Error("missing [Service] section")
	}
	if !strings.Contains(s, "[Install]") {
		t.Error("missing [Install] section")
	}
}

func TestRenderServiceStringSystemScope(t *testing.T) {
	s := RenderServiceString("/usr/bin/mihomo", "/etc/mihomo", SystemScope)

	if !strings.Contains(s, "ExecStart=/usr/bin/mihomo -d /etc/mihomo") {
		t.Error("missing ExecStart")
	}
	if !strings.Contains(s, "WantedBy=multi-user.target") {
		t.Error("system scope should use multi-user.target")
	}
}

// mockServiceManager is a test double for ServiceManager.
type mockServiceManager struct {
	startCalled bool
	stopCalled  bool
	activeVal   bool
	enabledVal  bool
}

func (m *mockServiceManager) Start(s string) error    { m.startCalled = true; return nil }
func (m *mockServiceManager) Stop(s string) error     { m.stopCalled = true; return nil }
func (m *mockServiceManager) Restart(s string) error  { return nil }
func (m *mockServiceManager) Enable(s string) error   { return nil }
func (m *mockServiceManager) Disable(s string) error  { return nil }
func (m *mockServiceManager) Status(s string) error   { return nil }
func (m *mockServiceManager) DaemonReload() error     { return nil }
func (m *mockServiceManager) IsActive(s string) bool  { return m.activeVal }
func (m *mockServiceManager) IsEnabled(s string) bool { return m.enabledVal }

func TestMockServiceManager(t *testing.T) {
	m := &mockServiceManager{activeVal: true, enabledVal: false}

	if !m.IsActive("mihomo.service") {
		t.Error("expected active")
	}
	if m.IsEnabled("mihomo.service") {
		t.Error("expected not enabled")
	}

	_ = m.Start("mihomo.service")
	if !m.startCalled {
		t.Error("Start should have been called")
	}
}
