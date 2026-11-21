package web

import (
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/vfs"
)

func WebTools(fs *vfs.VFS) {
	tool.Default().Register(webfetchTool(fs))
	tool.Default().Register(websearchTool(fs))
}
