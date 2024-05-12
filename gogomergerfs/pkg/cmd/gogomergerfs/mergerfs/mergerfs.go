package mergerfs

import (
	cobra "github.com/spf13/cobra"

	merger "github.com/on2e/union-csi-driver/gogomergerfs/pkg/merger"
	mergerfs "github.com/on2e/union-csi-driver/gogomergerfs/pkg/merger/mergerfs"
	signal "github.com/on2e/union-csi-driver/gogomergerfs/pkg/signal"
)

type flags struct {
	Branches []string
	Target   string
	Options  []string
	Block    bool
}

func NewCommand() *cobra.Command {
	flags := &flags{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "mergerfs",
		Short: "Use mergerfs",
		Long:  "A featureful FUSE based union filesystem (https://github.com/trapexit/mergerfs)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, flags)
		},
	}
	cmd.Flags().StringSliceVar(
		&flags.Branches,
		"branches",
		[]string{},
		"Comma-separated list of paths to merge together",
	)
	cmd.Flags().StringVar(
		&flags.Target,
		"target",
		"",
		"The union mount point",
	)
	cmd.Flags().StringSliceVarP(
		&flags.Options,
		"options",
		"o",
		[]string{},
		"Comma-separated list of mount options",
	)
	cmd.Flags().BoolVar(
		&flags.Block,
		"block",
		false,
		"Execute mergerfs, block for SIGINT or SIGTERM, then unmount. If set to false, execute mergerfs as if executing directly the command",
	)
	cmd.Flags().SortFlags = false
	return cmd
}

func runCommand(cmd *cobra.Command, flags *flags) error {
	var mfs merger.Merger = mergerfs.NewMergerfs()

	if !flags.Block {
		return mfs.Merge(flags.Branches, flags.Target, flags.Options)
	}

	bm := merger.NewBlockingMerger(mfs, flags.Branches, flags.Target, flags.Options)
	if err := bm.Run(signal.SetupSignalHandler()); err != nil {
		return err
	}
	if err := bm.CleanUp(); err != nil {
		return err
	}

	return nil
}
