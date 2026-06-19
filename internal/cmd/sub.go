package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"mihoro-go/internal/config"
	"mihoro-go/internal/mihoro"
	"mihoro-go/internal/systemctl"
	"mihoro-go/internal/utils"

	"github.com/spf13/cobra"
)

var subCmd = &cobra.Command{
	Use:   "sub",
	Short: "Manage subscriptions",
	Long:  "Manage proxy subscriptions: add, list, update, switch, and remove.",
}

var subAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a subscription interactively",
	RunE:  runSubAdd,
}

var subListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all subscriptions",
	RunE:    runSubList,
}

var subInfoCmd = &cobra.Command{
	Use:   "info [name]",
	Short: "Show subscription details",
	RunE:  runSubInfo,
}

var subRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a subscription",
	Args:    cobra.ExactArgs(1),
	RunE:    runSubRemove,
}

var (
	subUpdateAll   bool
	subUpdateForce bool
	subPurge       bool
)

var subUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Download subscription updates",
	RunE:  runSubUpdate,
}

var subUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a subscription",
	Args:  cobra.ExactArgs(1),
	RunE:  runSubUse,
}

var subCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show active subscription",
	RunE:  runSubCurrent,
}

func init() {
	subCmd.AddCommand(subAddCmd)
	subCmd.AddCommand(subListCmd)
	subCmd.AddCommand(subInfoCmd)
	subCmd.AddCommand(subRemoveCmd)
	subCmd.AddCommand(subUpdateCmd)
	subCmd.AddCommand(subUseCmd)
	subCmd.AddCommand(subCurrentCmd)

	subRemoveCmd.Flags().BoolVar(&subPurge, "purge", false, "Also delete downloaded files")
	subUpdateCmd.Flags().BoolVarP(&subUpdateAll, "all", "a", false, "Update all subscriptions")
	subUpdateCmd.Flags().BoolVarP(&subUpdateForce, "force", "f", false, "Force download even if cached")
}

func loadSubConfig() (*config.SubscriptionsFile, string, error) {
	dir := configPath
	sf, err := config.LoadSubscriptions(dir)
	if err != nil {
		return nil, dir, err
	}
	return sf, dir, nil
}

func runSubAdd(cmd *cobra.Command, args []string) error {
	sf, dir, err := loadSubConfig()
	if err != nil {
		return err
	}

	ctx := CliCtx()
	sub := config.Subscription{}
	sub.UserAgent = "clash/mihoro-go"

	for {
		sub.Name = prompt("Subscription name (e.g. my-vps)", "")
		if sub.Name == "" {
			fmt.Println("  Name is required.")
			continue
		}
		if len(sub.Name) > 20 {
			fmt.Printf("  Name too long (%d chars, max 20).\n", len(sub.Name))
			continue
		}
		if strings.ContainsAny(sub.Name, " \t") {
			fmt.Println("  Name must not contain spaces.")
			continue
		}
		if strings.Contains(sub.Name, "..") || strings.Contains(sub.Name, "/") {
			fmt.Println("  Name must not contain path separators.")
			continue
		}
		if sf.Find(sub.Name) != nil {
			fmt.Printf("  Subscription %q already exists.\n", sub.Name)
			continue
		}
		break
	}

	for {
		sub.URL = prompt("Subscription URL", "")
		if sub.URL == "" {
			fmt.Println("  URL is required.")
			continue
		}
		break
	}

	// Download immediately to validate
	destPath := config.SubDownloadPath(dir, sub.Name)
	var size int64
	fmt.Printf("  Downloading... ")
	size, err = utils.Download(ctx, nil, utils.DownloadOptions{
		URL:       sub.URL,
		DestPath:  destPath,
		UserAgent: sub.UserAgent,
		Timeout:   10 * time.Second,
	})
	if err != nil {
		fmt.Printf("  FAILED (%v)\n", err)

		// Ask proxy/headers for retry — only retry if user provides something
		sub.Proxy = prompt("Proxy URL (leave empty to skip)", "")
		if sub.Proxy == "" {
			fmt.Print("Add custom headers? [y/N]: ")
			hs := bufio.NewScanner(os.Stdin)
			hs.Scan()
			if strings.EqualFold(strings.TrimSpace(hs.Text()), "y") {
				fmt.Println("Headers (key=value, empty line to finish):")
				sub.Headers = make(map[string]string)
				for {
					fmt.Print("  ")
					if !hs.Scan() {
						break
					}
					line := strings.TrimSpace(hs.Text())
					if line == "" {
						break
					}
					if k, v, ok := parseKeyValue(line); ok {
						sub.Headers[k] = v
					} else {
						fmt.Println("  Invalid format, use key=value")
					}
				}
			}
		}

		if sub.Proxy != "" || len(sub.Headers) > 0 {
			fmt.Printf("  Retrying... ")
			size, err = utils.Download(ctx, nil, utils.DownloadOptions{
				URL:       sub.URL,
				DestPath:  destPath,
				UserAgent: sub.UserAgent,
				Headers:   sub.Headers,
				ProxyURL:  sub.Proxy,
			})
			if err != nil {
				fmt.Println("FAILED")
				return fmt.Errorf("subscription download failed: %w", err)
			}
		} else {
			return fmt.Errorf("subscription download failed (use proxy or custom headers to retry)")
		}
	}
	fmt.Printf("OK (%dKB)\n", size/1024)

	if err := utils.TryDecodeBase64InPlace(destPath); err != nil {
		fmt.Printf("  warning: base64 decode: %v\n", err)
	}

	// Auto-update
	fmt.Print("Enable auto-update? [Y/n]: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	autoStr := strings.TrimSpace(scanner.Text())
	sub.AutoUpdate = autoStr == "" || strings.EqualFold(autoStr, "y") || strings.EqualFold(autoStr, "yes")

	// Save
	if err := sf.Add(sub); err != nil {
		return err
	}
	now := time.Now()
	for i := range sf.Subscriptions {
		if sf.Subscriptions[i].Name == sub.Name {
			sf.Subscriptions[i].LastUpdate = now.Format(time.RFC3339)
			sf.Subscriptions[i].LastStatus = "success"
			if info, err := os.Stat(destPath); err == nil {
				sf.Subscriptions[i].LastSize = info.Size()
			}
			break
		}
	}
	if err := sf.Save(); err != nil {
		fmt.Printf("  warning: save subscription: %v\n", err)
	}

	if sf.ActiveSubscription == "" {
		sf.ActiveSubscription = sub.Name
		if err := sf.Save(); err != nil {
			fmt.Printf("  warning: set active: %v\n", err)
		}
	}

	fmt.Printf("Added %q\n", sub.Name)
	return nil
}

func runSubList(cmd *cobra.Command, args []string) error {
	sf, _, err := loadSubConfig()
	if err != nil {
		return err
	}

	if len(sf.Subscriptions) == 0 {
		fmt.Println("No subscriptions. Use 'mihoro sub add' to add one.")
		return nil
	}

	fmt.Printf("%-18s %-10s %-18s %-14s\n",
		"NAME", "UPDATE", "LAST-UPDATE", "STATUS")

	for _, s := range sf.Subscriptions {
		update := "off"
		if s.AutoUpdate {
			update = "daily"
		}

		lastUpdate := "-"
		if s.LastUpdate != "" {
			lastUpdate = formatShortTime(s.LastUpdate)
		}

		status := statusColored(s.LastStatus, s.LastSize)

		name := s.Name
		if s.Name == sf.ActiveSubscription {
			name = name + " *"
		}

		fmt.Printf("%-18s %-10s %-18s %-14s\n",
			name, update, lastUpdate, status)
	}
	return nil
}

func runSubInfo(cmd *cobra.Command, args []string) error {
	sf, _, err := loadSubConfig()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		s := sf.Find(args[0])
		if s == nil {
			return fmt.Errorf("subscription %q not found", args[0])
		}
		printSubInfo(sf, s)
		return nil
	}

	if len(sf.Subscriptions) == 0 {
		fmt.Println("No subscriptions configured.")
		return nil
	}

	for i, s := range sf.Subscriptions {
		if i > 0 {
			fmt.Println("---")
		}
		printSubInfo(sf, &s)
	}
	return nil
}

func printSubInfo(sf *config.SubscriptionsFile, s *config.Subscription) {
	auto := "no"
	if s.AutoUpdate {
		auto = "yes"
	}

	lastUpdate := "-"
	if s.LastUpdate != "" {
		lastUpdate = formatShortTime(s.LastUpdate)
	}

	status := statusColored(s.LastStatus, s.LastSize)
	if s.LastError != "" {
		status += " (" + s.LastError + ")"
	}

	created := "-"
	if s.CreatedAt != "" {
		created = formatShortTime(s.CreatedAt)
	}

	active := "no"
	if s.Name == sf.ActiveSubscription {
		active = "yes"
	}

	fmt.Printf("Name:      %s\n", s.Name)
	fmt.Printf("URL:       %s\n", s.URL)
	fmt.Printf("UA:        %s\n", s.UserAgent)
	fmt.Printf("Update:    %s\n", auto)
	fmt.Printf("Last:      %s\n", lastUpdate)
	fmt.Printf("Status:    %s\n", status)
	fmt.Printf("Created:   %s\n", created)
	fmt.Printf("Active:    %s\n", active)

	if len(s.Headers) > 0 {
		fmt.Println("Headers:")
		for k, v := range s.Headers {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
	if s.Proxy != "" {
		fmt.Printf("Proxy:     %s\n", s.Proxy)
	}
}

func runSubRemove(cmd *cobra.Command, args []string) error {
	sf, dir, err := loadSubConfig()
	if err != nil {
		return err
	}

	name := args[0]
	if name == sf.ActiveSubscription {
		return fmt.Errorf("cannot remove active subscription %q. Switch first with 'mihoro sub use <name>'", name)
	}

	s := sf.Find(name)
	if s == nil {
		return fmt.Errorf("subscription %q not found", name)
	}

	if err := sf.Remove(name); err != nil {
		return err
	}
	if err := sf.Save(); err != nil {
		return err
	}

	if subPurge {
		_ = os.Remove(config.SubDownloadPath(dir, name))
		fmt.Printf("Removed %q (purged)\n", name)
	} else {
		fmt.Printf("Removed %q\n", name)
	}
	return nil
}

func runSubUpdate(cmd *cobra.Command, args []string) error {
	sf, dir, err := loadSubConfig()
	if err != nil {
		return err
	}

	if !subUpdateAll && len(args) == 0 {
		return fmt.Errorf("specify a subscription name or use --all")
	}
	if subUpdateAll && len(args) > 0 {
		return fmt.Errorf("cannot specify both --all and a subscription name")
	}

	var targets []*config.Subscription
	if subUpdateAll {
		for i := range sf.Subscriptions {
			targets = append(targets, &sf.Subscriptions[i])
		}
		if len(targets) == 0 {
			fmt.Println("No subscriptions configured.")
			return nil
		}
	} else {
		s := sf.Find(args[0])
		if s == nil {
			return fmt.Errorf("subscription %q not found", args[0])
		}
		targets = append(targets, s)
	}

	ctx := CliCtx()
	var hasFailures bool
	for _, s := range targets {
		destPath := config.SubDownloadPath(dir, s.Name)

		if !subUpdateForce {
			if _, err := os.Stat(destPath); err == nil {
				fmt.Printf("  %-12s unchanged\n", s.Name)
				continue
			}
		}

		ua := s.UserAgent
		if ua == "" {
			ua = "clash/mihoro-go"
		}

		fmt.Printf("  %-12s ", s.Name)
		size, err := utils.Download(ctx, nil, utils.DownloadOptions{
			URL:       s.URL,
			DestPath:  destPath,
			UserAgent: ua,
			Headers:   s.Headers,
			ProxyURL:  s.Proxy,
		})
		if err != nil {
			fmt.Println("FAILED")
			s.LastUpdate = time.Now().Format(time.RFC3339)
			s.LastStatus = "failed"
			s.LastError = err.Error()
			_ = sf.Save()
			hasFailures = true
			continue
		}

		if err := utils.TryDecodeBase64InPlace(destPath); err != nil {
			fmt.Printf("  warning: base64 decode: %v\n", err)
		}

		s.LastUpdate = time.Now().Format(time.RFC3339)
		s.LastStatus = "success"
		s.LastSize = size
		_ = sf.Save()

		fmt.Printf("OK (%dKB)\n", size/1024)

		if s.Name == sf.ActiveSubscription {
			m, err := mihoro.New(configPath)
			if err != nil {
				fmt.Printf("  warning: load config: %v\n", err)
			} else {
				if err := config.CopyAfterOverride(destPath, m.MihomoCfg, &m.Config.MihomoConfig); err != nil {
					fmt.Printf("  warning: apply: %v\n", err)
				}
				if err := systemctl.Restart(systemctl.MihomoService); err != nil {
					fmt.Printf("  warning: restart: %v\n", err)
				}
			}
		}
	}
	if hasFailures {
		return fmt.Errorf("some subscriptions failed to download")
	}
	return nil
}

func runSubUse(cmd *cobra.Command, args []string) error {
	sf, dir, err := loadSubConfig()
	if err != nil {
		return err
	}

	ctx := CliCtx()
	name := args[0]
	s := sf.Find(name)
	if s == nil {
		return fmt.Errorf("subscription %q not found", name)
	}

	subYaml := config.SubDownloadPath(dir, name)

	if _, err := os.Stat(subYaml); os.IsNotExist(err) {
		ua := s.UserAgent
		if ua == "" {
			ua = "clash/mihoro-go"
		}
		fmt.Printf("  Downloading... ")
		size, err := utils.Download(ctx, nil, utils.DownloadOptions{
			URL:       s.URL,
			DestPath:  subYaml,
			UserAgent: ua,
			Headers:   s.Headers,
			ProxyURL:  s.Proxy,
		})
		if err != nil {
			return fmt.Errorf("download: %w", err)
		}
		if err := utils.TryDecodeBase64InPlace(subYaml); err != nil {
			fmt.Printf("  warning: base64 decode: %v\n", err)
		}
		s.LastSize = size
		fmt.Printf("OK (%dKB)\n", size/1024)
	}

	m, err := mihoro.New(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := config.CopyAfterOverride(subYaml, m.MihomoCfg, &m.Config.MihomoConfig); err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	if err := systemctl.Restart(systemctl.MihomoService); err != nil {
		fmt.Printf("  warning: restart mihomo: %v\n", err)
	}

	s.LastUpdate = time.Now().Format(time.RFC3339)
	s.LastStatus = "success"
	sf.ActiveSubscription = name
	if err := sf.Save(); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	fmt.Printf("Switched to %q\n", name)
	return nil
}

func runSubCurrent(cmd *cobra.Command, args []string) error {
	sf, _, err := loadSubConfig()
	if err != nil {
		return err
	}

	s := sf.Active()
	if s == nil {
		fmt.Println("No active subscription. Use 'mihoro sub use <name>' to select one.")
		return nil
	}

	fmt.Printf("Active subscription: %s (%s)\n", s.Name, s.URL)
	return nil
}

func prompt(label string, current string) string {
	if current != "" {
		fmt.Printf("%s [%s]: ", label, current)
	} else {
		fmt.Printf("%s: ", label)
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		os.Exit(1)
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return current
	}
	return input
}

func parseKeyValue(s string) (key, value string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

func formatShortTime(t string) string {
	if len(t) >= 16 {
		return t[:10] + " " + t[11:16]
	}
	return t
}

func statusColored(lastStatus string, lastSize int64) string {
	switch lastStatus {
	case "success":
		if lastSize < 1024 {
			return fmt.Sprintf("\033[32mOK (%dB)\033[0m", lastSize)
		}
		return fmt.Sprintf("\033[32mOK (%dKB)\033[0m", lastSize/1024)
	case "failed":
		return "\033[31mFAILED\033[0m"
	default:
		return "-"
	}
}
