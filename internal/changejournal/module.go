package changejournal

import (
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/vfs"
)

func ChangeJournalModule(cfg config.ChangeJournalConfig, fs *vfs.VFS, sessions *session.Manager, bus event.Bus, log *logger.Logger) (*Manager, error) {
	return New(Deps{
		Config:   cfg,
		FS:       fs,
		Sessions: sessions,
		Bus:      bus,
		Log:      log,
	})
}
