package commands

import (
	"os"

	"audiod/internal/branding"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: "Generate shell completion script for " + branding.AppName + " CLI.\n\n" +
		`To load completions:

Bash:
  $ source <(audiod completion bash)
  # To load completions for each session, execute once:
  $ audiod completion bash > /etc/bash_completion.d/audiod

Zsh:
  $ source <(audiod completion zsh)
  # To load completions for each session, execute once:
  $ audiod completion zsh > "${fpath[1]}/_audiod"

Fish:
  $ audiod completion fish | source
  # To load completions for each session, execute once:
  $ audiod completion fish > ~/.config/fish/completions/audiod.fish

PowerShell:
  PS> audiod completion powershell | Out-String | Invoke-Expression
  # To load completions for each session, execute once:
  PS> audiod completion powershell > audiod.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}
