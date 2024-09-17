package app

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"multi-app-relay-service/pkg/ui"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const ManagementPort = 7999

type RunManager struct {
	ActiveApps map[string]*App // Map of running apps by ID
	Limit      int             // Maximum number of apps
	Mutex      sync.Mutex      // Mutex for concurrency control
}

func NewAppRunManager(limit int) *RunManager {
	return &RunManager{
		ActiveApps: make(map[string]*App),
		Limit:      limit,
	}
}

func (am *RunManager) RunApp(app *App) error {
	am.Mutex.Lock()
	defer am.Mutex.Unlock()

	if len(am.ActiveApps) >= am.Limit {
		return fmt.Errorf("app limit reached")
	}
	am.ActiveApps[app.ID] = app
	err := app.Start()
	return err
}

func (am *RunManager) StopApp(app *App) error {
	am.Mutex.Lock()
	defer am.Mutex.Unlock()

	if _, exists := am.ActiveApps[app.ID]; !exists {
		return fmt.Errorf("app not found")
	}
	app.Stop()
	delete(am.ActiveApps, app.ID)
	return nil
}

func (am *RunManager) GetRunningApp(id string) (*App, error) {
	am.Mutex.Lock()
	defer am.Mutex.Unlock()

	app, exists := am.ActiveApps[id]
	if !exists {
		return nil, fmt.Errorf("app not found")
	}
	return app, nil
}

func (am *RunManager) ListRunningApps() []*App {
	am.Mutex.Lock()
	defer am.Mutex.Unlock()

	apps := []*App{}
	for _, app := range am.ActiveApps {
		apps = append(apps, app)
	}
	return apps
}

type Manager struct {
	AllApps       []*App
	RunManager    *RunManager
	AppPorts      map[string]int
	AppsConfig    *AppsConfig
	ManagementApp *App
}

func NewManager() *Manager {
	return &Manager{
		AllApps:       make([]*App, 0),
		AppPorts:      make(map[string]int),
		RunManager:    NewAppRunManager(5),
		AppsConfig:    nil,
		ManagementApp: nil,
	}
}

func (m *Manager) GetApp(id string) (*App, error) {
	for _, app := range m.AllApps {
		if app.ID == id {
			return app, nil
		}
	}
	return nil, fmt.Errorf("app not found")
}

func (m *Manager) GetAppConfig(id string) (*Config, error) {
	for _, app := range m.AppsConfig.Apps {
		if app.Name == id {
			return app, nil
		}
	}
	return nil, fmt.Errorf("app not found")
}

func (m *Manager) GetAppPort(id string) (int, error) {
	port, exists := m.AppPorts[id]
	if !exists {
		return 0, fmt.Errorf("app not found")
	}
	return port, nil
}

func (m *Manager) StageUICode() error {
	return ui.CopyEmbeddedFiles(m.ManagementApp.RootDir)
}

func (m *Manager) StageCode() error {
	for _, repo := range m.AppsConfig.Repos {
		err := repo.Clean()
		if err != nil {
			return err
		}
		err = repo.Clone()
		if err != nil {
			return err
		}
	}
	return nil
}

type AppsConfig struct {
	Apps         []*Config  `yaml:"apps" json:"apps"`
	Version      string     `yaml:"version" json:"version"`
	ManagementUi *Config    `yaml:"ui" json:"ui"`
	Repos        []*GitRepo `yaml:"repos,omitempty" json:"repos,omitempty"`
}

type Config struct {
	Name              string  `yaml:"name" json:"name"`
	Command           string  `yaml:"command" json:"command"`
	RoutePath         *string `yaml:"routePath,omitempty" json:"routePath"`
	CodePath          *string `yaml:"codePath,omitempty" json:"codePath"`
	PassFullProxyPath bool    `yaml:"passFullProxyPath,omitempty" json:"passFullProxyPath,omitempty"`
	Type              Type    `yaml:"type" json:"type"`
	Meta              *Meta   `yaml:"meta" json:"meta"`
}

func (c *Config) ToApp(port int) (*App, error) {
	if c.RoutePath == nil {
		return nil, fmt.Errorf("routePath not found in config")
	}
	if c.CodePath == nil {
		return nil, fmt.Errorf("codePath not found in config")
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	rootDir := *c.CodePath
	if !filepath.IsAbs(rootDir) {
		rootDir = filepath.Join(wd, rootDir)
	}
	c.Command = strings.Replace(c.Command, "${PORT}", fmt.Sprintf("%d", port), -1)
	commands, err := c.ToCommandArray()
	if err != nil {
		return nil, err
	}
	return NewApp(c.Name, rootDir, c.Name, c.Type, commands, port), nil
}

type Meta struct {
	Title       string   `yaml:"title" json:"title"`
	Description string   `yaml:"description" json:"description"`
	ImageUrl    string   `yaml:"imageUrl,omitempty" json:"imageUrl,omitempty"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

func (c *Config) ToCommandArray() ([]string, error) {
	var commands []string
	for _, cmdStr := range strings.Split(c.Command, " ") {
		if cmdStr != "" {
			commands = append(commands, cmdStr)
		}
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("no commands found")
	}
	return commands, nil
}

func NewManagerFromYaml(filename string) (*Manager, error) {
	manager := NewManager()

	yamlFile, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config AppsConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}
	// Load apps from YAML file

	if config.Apps == nil {
		return nil, fmt.Errorf("no apps found in config")
	}
	startingPort := 8001
	for _, appConfig := range config.Apps {
		app, err := appConfig.ToApp(startingPort)
		if err != nil {
			return nil, err
		}
		manager.AllApps = append(manager.AllApps, app)
		manager.AppPorts[app.ID] = startingPort
		startingPort++
	}
	manager.AppsConfig = &config

	managementApp, err := config.ManagementUi.ToApp(ManagementPort)
	if err != nil {
		return nil, err
	}
	manager.AppPorts[managementApp.ID] = ManagementPort
	manager.ManagementApp = managementApp

	return manager, nil
}
