// Package cli wires the cobra command tree for notion-link.
package cli

import (
	"github.com/spf13/cobra"
)

// Version is overridden at build time via -ldflags.
var Version = "0.1.0-dev"

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "notion-link",
		Short:         "Symlink Notion pages from your local offline cache as Markdown files.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}

	root.AddCommand(newLinkCmd())
	root.AddCommand(newSyncCmd())
	root.AddCommand(newUnlinkCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newSpacesCmd())
	root.AddCommand(newVersionCmd())
	return root
}
