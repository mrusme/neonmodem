//go:build !unix && !windows

package shell

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

func Command(context.Context, string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("can't run on %s", runtime.GOOS)
}
