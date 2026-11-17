package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"mihoro-go/internal/bin"
	"mihoro-go/internal/mihoro"
	"mihoro-go/internal/utils"

	"github.com/spf13/cobra"
)

var (
	updateCoreFlag    bool
	updateGeodataFlag bool
	updateUIFlag      bool
	updateAllFlag     bool
	updateArch        string
	updateMirror      string
	updateProxy       string
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update mihomo components (core, geodata, ui)",
	Long:  "Update mihomo components. By default, core + geodata + ui are updated.",
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateCoreFlag, "core", false, "Update mihomo core binary")
	updateCmd.Flags().BoolVar(&updateGeodataFlag, "geodata", false, "Update geodata")
	updateCmd.Flags().BoolVar(&updateUIFlag, "ui", false, "Update external UI assets")
	updateCmd.Flags().BoolVar(&updateAllFlag, "all", false, "Update everything: geodata, ui, and core")
	updateCmd.Flags().StringVar(&updateArch, "arch", "", "Override architecture detection")
	updateCmd.Flags().StringVar(&updateMirror, "mirror", "", "GitHub mirror base URL (e.g. https://ghfast.top)")
	updateCmd.Flags().StringVar(&updateProxy, "proxy", "",
		"HTTP/SOCKS proxy for downloads (e.g. http://127.0.0.1:7890)")
	updateCmd.MarkFlagsMutuallyExclusive("all", "core")
	updateCmd.MarkFlagsMutuallyExclusive("all", "geodata")
	updateCmd.MarkFlagsMutuallyExclusive("all", "ui")
}

// updateStage describes one component: how to prepare it for installation.
type updateStage struct {
	name    string
	prepare func(ctx context.Context, client *http.Client) (mihoro.PreparedUpdate, string, error)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := CliCtx()

	if updateArch != "" {
		if _, err := bin.ValidateArch(updateArch); err != nil {
			return fmt.Errorf("--arch: %w", err)
		}
	}

	dir := configPath
	m, err := mihoro.New(dir)
	if err != nil {
		return err
	}

	// Mirror: the flag wins, config.toml is the persistent fallback. The
	// normalized value is exported so all downloads in this run use it.
	rawMirror := updateMirror
	if rawMirror == "" {
		rawMirror = m.Config.GitHubMirror
	}
	if _, err := utils.SetMirrorEnv(rawMirror); err != nil {
		return fmt.Errorf("--mirror: %w", err)
	}

	// Proxy: explicit only. The flag wins over config.toml.
	proxy, err := utils.NormalizeProxy(updateProxy)
	if err != nil {
		return fmt.Errorf("--proxy: %w", err)
	}
	if proxy == "" {
		proxy, err = utils.NormalizeProxy(m.Config.Proxy)
		if err != nil {
			return fmt.Errorf("config proxy: %w", err)
		}
	}
	if proxy != "" {
		fmt.Printf("Using proxy %s\n", utils.RedactProxy(proxy))
	}
	client := newHTTPClient(proxy)

	stages := updateStages(m, updateArch)
	report := mihoro.NewStageReport()

	// Phase 1: prepare (download and verify) every component.
	type pendingUpdate struct {
		name string
		plan mihoro.PreparedUpdate
	}
	var pending []pendingUpdate
	prepareFailed := false

	for _, stage := range stages {
		plan, reason, err := stage.prepare(ctx, client)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			report.Failed(stage.name, err)
			prepareFailed = true
			break
		}
		if plan == nil {
			report.Skipped(stage.name, reason)
			continue
		}
		pending = append(pending, pendingUpdate{name: stage.name, plan: plan})
	}

	if prepareFailed {
		for _, p := range pending {
			p.plan.Discard()
		}
		report.Print("update summary")
		return fmt.Errorf("update failed during download - nothing was changed")
	}

	// Phase 2: apply. Core is last in every stage list, so the service is
	// stopped only after all downloads succeeded.
	applyFailed := false
	for _, p := range pending {
		if err := p.plan.Apply(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			report.Failed(p.name, err)
			applyFailed = true
			break
		}
		p.plan.Discard()
		report.Installed(p.name)
	}
	for _, p := range pending {
		p.plan.Discard()
	}

	// Restart once, after everything is in place. UI assets are static and
	// do not need a restart; a skipped-only run restarts nothing.
	if !applyFailed && (report.HasInstalled("core") || report.HasInstalled("geodata")) {
		runStage(report, "service restart", func() (mihoro.StageStatus, error) {
			if err := m.RestartService(); err != nil {
				return mihoro.StageFailed, err
			}
			return mihoro.StageInstalled, nil
		})
	}

	report.Print("update summary")
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if report.HasFailures() {
		return fmt.Errorf("one or more update stages failed - see summary above")
	}
	return nil
}

// updateStages returns the components selected by the command flags. Core is
// always last so the service stops as late as possible.
func updateStages(m *mihoro.Mihoro, arch string) []updateStage {
	core := updateStage{
		name: "core",
		prepare: func(ctx context.Context, client *http.Client) (mihoro.PreparedUpdate, string, error) {
			return m.PrepareCore(ctx, client, arch)
		},
	}
	geodata := updateStage{
		name: "geodata",
		prepare: func(ctx context.Context, client *http.Client) (mihoro.PreparedUpdate, string, error) {
			return m.PrepareGeodata(ctx, client)
		},
	}
	ui := updateStage{
		name: "ui",
		prepare: func(ctx context.Context, client *http.Client) (mihoro.PreparedUpdate, string, error) {
			return m.PrepareUI(ctx, client)
		},
	}

	switch {
	case updateAllFlag:
		return []updateStage{geodata, ui, core}
	case updateCoreFlag:
		return []updateStage{core}
	case updateUIFlag:
		return []updateStage{ui}
	case updateGeodataFlag:
		return []updateStage{geodata}
	default:
		return []updateStage{geodata, ui, core}
	}
}

func runStage(report *mihoro.StageReport, name string, fn func() (mihoro.StageStatus, error)) {
	status, err := fn()
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	switch status {
	case mihoro.StageInstalled:
		report.Installed(name)
	case mihoro.StageSkipped:
		report.Skipped(name, "")
	case mihoro.StageFailed:
		report.Failed(name, err)
	}
}
