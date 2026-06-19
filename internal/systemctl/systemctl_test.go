package systemctl

import (
	"strings"
	"testing"
)

func TestRenderMihomoService(t *testing.T) {
	s := RenderMihomoService("/usr/bin/mihomo", "/home/user/.config/mihomo")

	if !strings.Contains(s, "ExecStart=/usr/bin/mihomo -d /home/user/.config/mihomo") {
		t.Error("missing ExecStart")
	}
	if !strings.Contains(s, "WantedBy=multi-user.target") {
		t.Error("should use multi-user.target")
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

func TestRenderSubTimer(t *testing.T) {
	s := RenderSubTimer()

	if !strings.Contains(s, "OnCalendar=*-*-* 02:00:00") {
		t.Error("missing OnCalendar")
	}
	if !strings.Contains(s, "timers.target") {
		t.Error("missing timers.target")
	}
}

func TestRenderSubTimerService(t *testing.T) {
	s := RenderSubTimerService("/usr/local/bin/mihoro")

	if !strings.Contains(s, "/usr/local/bin/mihoro sub update --all") {
		t.Error("missing mihoro sub update --all")
	}
	if !strings.Contains(s, "Type=oneshot") {
		t.Error("missing Type=oneshot")
	}
}

func TestRenderUpdateTimer(t *testing.T) {
	s := RenderUpdateTimer()

	if !strings.Contains(s, "OnCalendar=Mon *-*-* 01:00:00") {
		t.Error("missing OnCalendar")
	}
}

func TestRenderUpdateTimerService(t *testing.T) {
	s := RenderUpdateTimerService("/usr/local/bin/mihoro", "https://ghfast.top")

	if !strings.Contains(s, "/usr/local/bin/mihoro update --mirror https://ghfast.top") {
		t.Error("missing mihoro update --mirror")
	}
}

func TestRenderUpdateTimerServiceNoMirror(t *testing.T) {
	s := RenderUpdateTimerService("/usr/local/bin/mihoro", "")

	if !strings.Contains(s, "/usr/local/bin/mihoro update") {
		t.Error("missing mihoro update")
	}
	if strings.Contains(s, "--mirror") {
		t.Error("should not contain --mirror")
	}
}
