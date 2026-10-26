package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

type savedTab struct {
	Title   string `yaml:"title"`
	Content string `yaml:"content"`
}

type savedTabs struct {
	Active int        `yaml:"active"`
	Tabs   []savedTab `yaml:"tabs"`
}

type tabStateFile struct {
	Connections map[string]savedTabs `yaml:"connections"`
}

var tabStateMu sync.Mutex

func tabStatePath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "dbrix", "editor_tabs.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "dbrix", "editor_tabs.yaml")
	}
	return filepath.Join(home, ".local", "state", "dbrix", "editor_tabs.yaml")
}

func loadTabStates() (map[string]savedTabs, error) {
	data, err := os.ReadFile(tabStatePath())
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]savedTabs{}, nil
	}
	if err != nil {
		return map[string]savedTabs{}, err
	}
	var file tabStateFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return map[string]savedTabs{}, err
	}
	if file.Connections == nil {
		file.Connections = map[string]savedTabs{}
	}
	return file.Connections, nil
}

func writeTabStates(states map[string]savedTabs) error {
	tabStateMu.Lock()
	defer tabStateMu.Unlock()

	path := tabStatePath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(tabStateFile{Connections: states})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".editor-tabs-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (m *Model) rememberCurrentTabs() {
	if m.tabConnName == "" || len(m.tabs) == 0 {
		return
	}
	if m.tabStates == nil {
		m.tabStates = map[string]savedTabs{}
	}
	m.syncActiveTab()
	m.tabStates[m.tabConnName] = savedTabsFrom(m.tabs, m.activeTab)
}

func (m *Model) collectTabStates() map[string]savedTabs {
	out := make(map[string]savedTabs, len(m.tabStates)+len(m.sessions)+1)
	for name, state := range m.tabStates {
		out[name] = state
	}
	for name, saved := range m.sessions {
		if len(saved.tabs) > 0 {
			out[name] = savedTabsFrom(saved.tabs, saved.activeTab)
		}
	}
	if m.tabConnName != "" && len(m.tabs) > 0 {
		m.syncActiveTab()
		out[m.tabConnName] = savedTabsFrom(m.tabs, m.activeTab)
	}
	return out
}

func (m *Model) saveTabStatesNow() error {
	states := m.collectTabStates()
	m.tabStates = states
	return writeTabStates(states)
}
