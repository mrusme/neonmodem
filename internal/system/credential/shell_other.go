//go:build !unix && !windows

package credential

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

func shellCommand(context.Context, string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("can't run on %s", runtime.GOOS)
}
