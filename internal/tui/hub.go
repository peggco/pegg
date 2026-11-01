package tui

import (
	"context"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/skill"
	"github.com/peggco/pegg/internal/skillhub"
	"github.com/peggco/pegg/internal/tui/components"
)

func (a *App) searchHubSkills(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}
	a.hubMu.Lock()
	a.hubSeq++
	seq := a.hubSeq
	a.hubQuery = query
	a.hubMu.Unlock()

	go func() {
		time.Sleep(250 * time.Millisecond)

		a.hubMu.Lock()
		current := a.hubSeq == seq && a.hubQuery == query
		a.hubMu.Unlock()
		if !current {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		items, err := skillhub.NewClient().Search(ctx, query, 8)

		a.hubMu.Lock()
		current = a.hubSeq == seq && a.hubQuery == query
		a.hubMu.Unlock()
		if !current {
			return
		}

		a.chatMu.Lock()
		defer a.chatMu.Unlock()
		if a.home == nil {
			return
		}
		if err != nil {
			a.home.SetHubLoading(false)
			a.requestRedraw()
			return
		}
		list := make([]components.ListItem, 0, len(items))
		for _, item := range items {
			list = append(list, components.ListItem{
				Label:  item.Name,
				Detail: item.Source + " · hub",
				Data:   "hub|" + item.Source + "|" + item.SkillID,
			})
		}
		a.home.SetHubItems(list)
		a.requestRedraw()
	}()
}

func (a *App) installHubSkill(source, skillID, name string) {
	source = strings.TrimSpace(source)
	skillID = strings.TrimSpace(skillID)
	if source == "" || skillID == "" {
		return
	}
	go func() {
		dir, err := config.GetConfigPath("skills")
		if err != nil {
			a.setHubError(err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		installed, err := skillhub.Install(ctx, skillhub.NewClient(), skillhub.Spec{Source: source, SkillID: skillID}, dir)
		if err != nil {
			a.setHubError(err.Error())
			return
		}
		if err := skill.Reload(); err != nil {
			a.setHubError(err.Error())
			return
		}
		installedName := strings.TrimSpace(installed.Name)
		if installedName == "" {
			installedName = name
		}

		a.chatMu.Lock()
		defer a.chatMu.Unlock()
		if a.home == nil {
			return
		}
		items := make([]components.ListItem, 0)
		for _, sk := range skill.List() {
			items = append(items, components.ListItem{Label: sk.Name, Detail: sk.Description})
		}
		a.home.SetSkills(items)
		a.home.Input().ReplaceSkill(installedName)
		a.home.DismissPickers()
		a.errorMsg = ""
		a.requestRedraw()
	}()
}

func (a *App) setHubError(message string) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if a.home != nil {
		a.home.SetHubLoading(false)
	}
	a.errorMsg = message
	a.requestRedraw()
}
