package i18n

import (
	"fmt"
	"sync"
)

const (
	English = "en"
	Turkish = "tr"
)

type languageInfo struct {
	code    string
	display string
}

var supported = []languageInfo{
	{code: English, display: "English"},
	{code: Turkish, display: "Türkçe"},
}

var catalogs = map[string]map[string]string{
	English: en,
	Turkish: tr,
}

var (
	mu        sync.RWMutex
	current   = English
	observers []func(string)
)

func Names() []string {
	out := make([]string, 0, len(supported))
	for _, l := range supported {
		out = append(out, l.code)
	}
	return out
}

func DisplayName(code string) string {
	for _, l := range supported {
		if l.code == code {
			return l.display
		}
	}
	return code
}

func Current() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

func Set(code string) bool {
	if _, ok := catalogs[code]; !ok {
		return false
	}
	mu.Lock()
	changed := current != code
	current = code
	listeners := observers
	mu.Unlock()
	if changed {
		for _, fn := range listeners {
			fn(code)
		}
	}
	return true
}

func T(key string) string {
	code := Current()
	if table, ok := catalogs[code]; ok {
		if v, ok := table[key]; ok {
			return v
		}
	}
	if code != English {
		if v, ok := en[key]; ok {
			return v
		}
	}
	return key
}

func Tf(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

func OnChange(fn func(string)) {
	mu.Lock()
	observers = append(observers, fn)
	mu.Unlock()
}
