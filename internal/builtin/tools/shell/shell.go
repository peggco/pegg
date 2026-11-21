package shell

import (
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/vfs"
)

func ShellTools(fs *vfs.VFS) {
	tool.Default().Register(bashTool(fs))
}
