package vfs

import (
	"context"
)

type AccessOp string

const (
	OpResolve AccessOp = "resolve"
	OpRead    AccessOp = "read"
	OpWrite   AccessOp = "write"
	OpEdit    AccessOp = "edit"
	OpDelete  AccessOp = "delete"
)

type AccessRequest struct {
	Op   AccessOp
	Path string
	Ctx  context.Context
}

type AccessVerdict struct {
	Allow bool
	Path  string
	Err   error
}

type accessCheck struct {
	Request AccessRequest
	Verdict AccessVerdict
}

func (v *VFS) OnAccessCheck(fn func(AccessRequest, AccessVerdict) AccessVerdict) {
	if fn == nil {
		return
	}
	v.hooks.access.Add(func(ac accessCheck) accessCheck {
		ac.Verdict = fn(ac.Request, ac.Verdict)
		return ac
	})
}

func (v *VFS) checkAccess(req AccessRequest) AccessVerdict {
	seed := AccessVerdict{Allow: false, Path: req.Path, Err: v.outOfBounds(req.Path)}
	if isGlobalWritable(req.Path) || isTempStorePath(req.Path) {
		seed.Allow = true
		seed.Err = nil
	}
	res := v.hooks.access.Apply(accessCheck{Request: req, Verdict: seed})
	verdict := res.Verdict
	if !verdict.Allow && verdict.Err == nil {
		verdict.Err = v.outOfBounds(req.Path)
	}
	return verdict
}

func (v *VFS) denyAccess(req AccessRequest, verdict AccessVerdict) error {
	if !verdict.Allow && verdict.Err != nil {
		return verdict.Err
	}
	return v.outOfBounds(req.Path)
}
