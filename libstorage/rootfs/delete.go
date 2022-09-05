package rootfs

import (
	"os"
	"path/filepath"

	"github.com/fuguohong1024/lightc/libstorage/errors"
	"github.com/fuguohong1024/lightc/paths"
	"golang.org/x/xerrors"
)

func Delete(id string) error {
	if _, err := os.Stat(filepath.Join(paths.RootFSPath, id)); err != nil {
		if os.IsNotExist(err) {
			return errors.RootFSNotExist{ID: id}
		}
		return xerrors.Errorf("get rootfs stat failed: %w", err)
	}

	if err := os.RemoveAll(filepath.Join(paths.RootFSPath, id)); err != nil {
		return xerrors.Errorf("remove rootfs failed: %w", err)
	}

	return nil
}
