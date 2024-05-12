package mergerfs

import (
	"fmt"
	"os/exec"
	"strings"

	mountutils "k8s.io/mount-utils"

	merger "github.com/on2e/union-csi-driver/gogomergerfs/pkg/merger"
)

// mergerfs wraps the mergerfs union filesystem (https://github.com/trapexit/mergerfs),
// implementing the pkg/merger.Merger interface.
type mergerfs struct {
	binaryPath string
	mounter    mountutils.Interface
}

var _ merger.Merger = &mergerfs{}

func NewMergerfs() *mergerfs {
	return &mergerfs{
		binaryPath: "mergerfs",
		mounter:    mountutils.New(""),
	}
}

func (m *mergerfs) SetBinaryPath(path string) {
	m.binaryPath = path
}

func (m *mergerfs) Merge(branches []string, target string, options []string) error {
	mergeCmd := m.binaryPath
	mergeArgs := []string{
		strings.Join(branches, ":"),
		target,
	}
	if len(options) > 0 {
		mergeArgs = append(mergeArgs, "-o", strings.Join(options, ","))
	}
	cmd := exec.Command(mergeCmd, mergeArgs...)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		fmt.Print(output)
	}
	if err != nil {
		// See k/k issue #103753
		if err.Error() == "wait: no child processes" {
			if cmd.ProcessState.Success() {
				return nil
			}
			err = &exec.ExitError{ProcessState: cmd.ProcessState}
		}
	}
	return err
}

func (m *mergerfs) Unmerge(target string) error {
	err := m.mounter.Unmount(target)
	// Ignore "not mounted" errors
	if err != nil && strings.Contains(err.Error(), "not mounted") {
		err = nil
	}
	return err
}
