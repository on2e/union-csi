package cmd

import (
	"fmt"
	"os"

	cobra "github.com/spf13/cobra"

	mergerfs "github.com/on2e/union-csi-driver/gogomergerfs/pkg/cmd/gogomergerfs/mergerfs"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "gogomergerfs",
		Short: "gogomergerfs is a Go wrapper for the MergerFS union filesystem",
		Long:  "gogomergerfs is a Go wrapper for the MergerFS union filesystem",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.AddCommand(mergerfs.NewCommand())
	return cmd
}

func Execute() {
	cmd := NewCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
