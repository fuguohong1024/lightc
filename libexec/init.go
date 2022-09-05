package libexec

import (
	"github.com/fuguohong1024/lightc/libexec/internal/process"
	"golang.org/x/xerrors"
)

func InitProcess() error {
	if err := process.InitProcess(); err != nil {
		return xerrors.Errorf("init container process failed: %w", err)
	}

	return nil
}
