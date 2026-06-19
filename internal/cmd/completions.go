package cmd

import "github.com/spf13/cobra"

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completions for mihoro",
	Long: `To load completions:

  Bash:
    source <(mihoro completion bash)

  Zsh:
    source <(mihoro completion zsh)

  Fish:
    mihoro completion fish | source`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			_ = cmd.Help()
			return nil
		}
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		default:
			_ = cmd.Help()
			return nil
		}
	},
}
