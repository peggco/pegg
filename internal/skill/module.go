package skill

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/prompt"
	"github.com/peggco/pegg/internal/builtin/skills"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
)

const agentsSkillsDir = ".agents/skills"

func SkillModule(log *logger.Logger) error {
	if err := loadAll(log, ""); err != nil {
		return err
	}
	agent.OnMessageInput(ExpandMessage)
	return nil
}

func Reload() error {
	return loadAll(nil, "")
}

func ReloadForProject(projectDir string) error {
	return loadAll(nil, projectDir)
}

func loadAll(log *logger.Logger, projectDir string) error {
	peggSkills, err := config.GetConfigPath("skills")
	if err != nil {
		return err
	}
	if err := MaterializeTo(peggSkills); err != nil {
		warn(log, "skill: failed to materialize built-in skills: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("skill: home dir: %w", err)
	}
	dirs := []string{filepath.Join(home, agentsSkillsDir), peggSkills}
	if projectDir != "" {
		dirs = append(dirs, filepath.Join(projectDir, ".pegg", "skills"))
	} else {
		dirs = append(dirs, config.ProjectReadPaths("skills")...)
	}
	resetRegistry()
	for _, dir := range dirs {
		if err := LoadDir(dir, dir); err != nil {
			warn(log, "skill: skip %s: %v", dir, err)
		}
	}
	return nil
}

func resetRegistry() {
	reg.mu.Lock()
	reg.skills = make(map[string]*Skill)
	reg.mu.Unlock()
}

func warn(log *logger.Logger, format string, args ...any) {
	if log == nil {
		return
	}
	log.Fwarn(format, args...)
}

func MaterializeTo(root string) error {
	for name, build := range skills.All() {
		targetDir := filepath.Join(root, name)
		if dirExists(targetDir) {
			continue
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return fmt.Errorf("skill: mkdir %s: %w", targetDir, err)
		}
		skillMD, err := build().Build(prompt.FormatMarkdown)
		if err != nil {
			return fmt.Errorf("skill: render %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, SkillFileName), []byte(skillMD), 0o644); err != nil {
			return fmt.Errorf("skill: write %s: %w", targetDir, err)
		}
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
