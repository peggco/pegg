package vfs

import (
	"context"

	"github.com/peggco/pegg/internal/core/hook"
)

type TransformContext struct {
	Path    string
	Content string
}

type FileDelete struct {
	Path string
}

type MutationOp string

const (
	MutationAdd    MutationOp = "add"
	MutationModify MutationOp = "modify"
	MutationDelete MutationOp = "delete"
)

type FileMutation struct {
	Ctx        context.Context
	Op         MutationOp
	Path       string
	BeforeHash string
	AfterHash  string
	Before     []byte
	After      []byte
	BeforeSize int64
	AfterSize  int64
	Truncated  bool
}

type vfsHooks struct {
	beforeRead     *hook.Hook[TransformContext]
	afterWrite     *hook.Hook[TransformContext]
	onDelete       *hook.Hook[FileDelete]
	beforeMutation *hook.Hook[FileMutation]
	afterMutation  *hook.Hook[FileMutation]
	access         *hook.Hook[accessCheck]
}

func newHooks() vfsHooks {
	return vfsHooks{
		beforeRead:     hook.NewHook[TransformContext](),
		afterWrite:     hook.NewHook[TransformContext](),
		onDelete:       hook.NewHook[FileDelete](),
		beforeMutation: hook.NewHook[FileMutation](),
		afterMutation:  hook.NewHook[FileMutation](),
		access:         hook.NewHook[accessCheck](),
	}
}

func (v *VFS) OnBeforeMutation(fn func(FileMutation)) {
	if fn == nil {
		return
	}
	v.hooks.beforeMutation.Add(func(ev FileMutation) FileMutation {
		fn(ev)
		return ev
	})
}

func (v *VFS) OnAfterMutation(fn func(FileMutation)) {
	if fn == nil {
		return
	}
	v.hooks.afterMutation.Add(func(ev FileMutation) FileMutation {
		fn(ev)
		return ev
	})
}

func (v *VFS) wantsMutations() bool {
	return v.hooks.beforeMutation.Has() || v.hooks.afterMutation.Has()
}

func (v *VFS) OnBeforeRead(fn func(path, content string) string) {
	v.hooks.beforeRead.Add(func(tc TransformContext) TransformContext {
		tc.Content = fn(tc.Path, tc.Content)
		return tc
	})
}

func (v *VFS) OnAfterWrite(fn func(path, content string) string) {
	v.hooks.afterWrite.Add(func(tc TransformContext) TransformContext {
		tc.Content = fn(tc.Path, tc.Content)
		return tc
	})
}

func (v *VFS) OnFileDelete(fn func(FileDelete) FileDelete) {
	v.hooks.onDelete.Add(fn)
}
