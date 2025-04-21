package kubectl

import (
	"github.com/spf13/cobra"
)

var treeCmd = &cobra.Command{
	Use:   "tree TREE",
	Short: "Display folder tree view",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func newTreeCmd() *cobra.Command {
	return treeCmd
}
