package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"mihoro-go/internal/bin"
	"mihoro-go/internal/mihoro"

	"github.com/spf13/cobra"
)

var (
	updateCoreFlag    bool
	updateGeodataFlag bool
	updateUIFlag      bool
	updateAllFlag     bool
	updateArch        string
	updateMirror      string
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
	updateCmd.MarkFlagsMutuallyExclusive("all", "core")
	updateCmd.MarkFlagsMutuallyExclusive("all", "geodata")
	updateCmd.MarkFlagsMutuallyExclusive("all", "ui")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := CliCtx()

	mirror := updateMirror

	if updateMirror != "" {
		if !strings.Contains(updateMirror, "://") || !strings.Contains(updateMirror[strings.Index(updateMirror, "://")+3:], ".") {
			return fmt.Errorf("--mirror requires a valid URL (e.g. --mirror \"https://ghfast.top\")")
		}
		_ = os.Setenv("MIHORO_GITHUB_MIRROR", updateMirror)
	}
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

	if mirror == "" {
		mirror = m.Config.GitHubMirror
	}

	report := mihoro.NewStageReport()

	if updateAllFlag {
		runStage(report, "geodata", func() (mihoro.StageStatus, error) {
			return m.UpdateGeodata(ctx, newHTTPClient(), mirror)
		})
		runStage(report, "ui", func() (mihoro.StageStatus, error) {
			return m.UpdateUI(ctx, newHTTPClient(), mirror)
		})
		runStage(report, "core", func() (mihoro.StageStatus, error) {
			return m.UpdateCore(ctx, newHTTPClient(), updateArch, mirror)
		})
		if !report.HasFailures() {
			runStage(report, "service restart", func() (mihoro.StageStatus, error) {
				if err := m.RestartService(); err != nil {
					return mihoro.StageFailed, err
				}
				return mihoro.StageInstalled, nil
			})
		}
	} else if updateCoreFlag {
		runStage(report, "core", func() (mihoro.StageStatus, error) {
			return m.UpdateCore(ctx, newHTTPClient(), updateArch, mirror)
		})
		if !report.HasFailures() && report.HasInstalled("core") {
			runStage(report, "service restart", func() (mihoro.StageStatus, error) {
				if err := m.RestartService(); err != nil {
					return mihoro.StageFailed, err
				}
				return mihoro.StageInstalled, nil
			})
		}
	} else if updateUIFlag {
		runStage(report, "ui", func() (mihoro.StageStatus, error) {
			return m.UpdateUI(ctx, newHTTPClient(), mirror)
		})
	} else if updateGeodataFlag {
		runStage(report, "geodata", func() (mihoro.StageStatus, error) {
			return m.UpdateGeodata(ctx, newHTTPClient(), mirror)
		})
	} else {
		// Default: update core, geodata, and ui
		runStage(report, "geodata", func() (mihoro.StageStatus, error) {
			return m.UpdateGeodata(ctx, newHTTPClient(), mirror)
		})
		runStage(report, "ui", func() (mihoro.StageStatus, error) {
			return m.UpdateUI(ctx, newHTTPClient(), mirror)
		})
		runStage(report, "core", func() (mihoro.StageStatus, error) {
			return m.UpdateCore(ctx, newHTTPClient(), updateArch, mirror)
		})
		if !report.HasFailures() {
			runStage(report, "service restart", func() (mihoro.StageStatus, error) {
				if err := m.RestartService(); err != nil {
					return mihoro.StageFailed, err
				}
				return mihoro.StageInstalled, nil
			})
		}
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
