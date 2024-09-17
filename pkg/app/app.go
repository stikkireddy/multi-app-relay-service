package app

import (
	"bytes"
	"errors"
	"fmt"
	cmd "github.com/ShinyTrinkets/overseer"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type App struct {
	ID            string           // Unique identifier for the app
	Name          string           // Human-readable name
	Type          Type             // Type of app, e.g., "python" or "nodejs"
	RootDir       string           // Root directory of the app
	Command       []string         // Command(s) to start the app
	Supervisor    *cmd.Overseer    // Pointer to the Supervisor struct
	LogChan       chan *cmd.LogMsg // Channel to receive log messages
	Status        Status           // Status of the app, e.g., "running", "stopped"
	LogLines      *LineBuffer      // Buffer to store log lines
	PreferredPort int              // Preferred port for the app

	mutex sync.Mutex // Mutex for concurrency control
}

type PythonVenv struct {
	VenvDir       string
	PythonBinPath string
	PythonPath    string
	ActivatePath  string
}

// DummyLogger doesn't do anything
// The default Overseer logger is: https://github.com/ShinyTrinkets/meta-logger/blob/master/default.go
// A good production logger is: https://github.com/azer/logger/blob/master/logger.go
type DummyLogger struct {
	Name string
}

func (l *DummyLogger) Info(msg string, v ...interface{}) {
	fmt.Printf("INFO: %v; %v\n", msg, v)
}
func (l *DummyLogger) Error(msg string, v ...interface{}) {
	fmt.Printf("ERROR: %v; %v\n", msg, v)
}

// LineBuffer holds the buffer and manages the lines.
type LineBuffer struct {
	buffer   bytes.Buffer
	lines    int
	maxLines int
}

// NewLineBuffer initializes a LineBuffer with a specified max number of lines.
func NewLineBuffer(maxLines int) *LineBuffer {
	return &LineBuffer{
		maxLines: maxLines,
	}
}

// Append adds a new string to the buffer.
func (lb *LineBuffer) Append(s string) {
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if lb.lines >= lb.maxLines {
			lb.trimOldestLine()
		}
		lb.buffer.WriteString(line + "\n")
		lb.lines++
	}
}

// trimOldestLine removes the oldest line from the buffer.
func (lb *LineBuffer) trimOldestLine() {
	index := strings.IndexByte(lb.buffer.String(), '\n')
	if index != -1 {
		lb.buffer.Next(index + 1)
		lb.lines--
	}
}

// String returns the content of the buffer as a string.
func (lb *LineBuffer) String() string {
	return lb.buffer.String()
}

func NewApp(id, rootDir, name string,
	appType Type,
	command []string,
	preferredPort int) *App {
	cmd.SetupLogBuilder(func(name string) cmd.Logger {
		return &DummyLogger{
			Name: name,
		}
	})

	supervisor := cmd.NewOverseer()
	logFeed := make(chan *cmd.LogMsg)
	logLines := NewLineBuffer(1000)
	supervisor.WatchLogs(logFeed)

	go func() {
		for log := range logFeed {
			//fmt.Printf("LOG: %v: %v\n", log.Type, log.Text)
			logLines.Append(log.Text)
		}
	}()

	return &App{
		ID:            id,
		Name:          name,
		Type:          appType,
		RootDir:       rootDir,
		Command:       command,
		Supervisor:    supervisor,
		Status:        StatusTerminated,
		LogChan:       logFeed,
		LogLines:      logLines,
		PreferredPort: preferredPort,
	}
}

func (a *App) newSuperVisor() {
	a.Supervisor = cmd.NewOverseer()
	a.Supervisor.WatchLogs(a.LogChan)
}

func (a *App) isPython() bool {
	return a.Type == TypePython
}

func (a *App) UpdateStatus(status Status) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.Status = status
}

func (a *App) pythonVenvPath() *PythonVenv {
	venvDir := filepath.Join(a.RootDir, ".venv")
	pythonBinPath := filepath.Join(venvDir, "bin")

	return &PythonVenv{
		VenvDir:       venvDir,
		PythonBinPath: pythonBinPath,
		ActivatePath:  filepath.Join(pythonBinPath, "activate"),
	}
}

func (a *App) pythonCmdOptions() cmd.Options {
	path := os.Getenv("PATH")
	return cmd.Options{
		Buffered:  false,
		Streaming: true,
		Dir:       a.RootDir,
		Env: []string{
			"PATH=" + a.pythonVenvPath().PythonBinPath + ":" + path,
			"MULTI_APP_PORT=" + fmt.Sprintf("%d", a.PreferredPort),
			"PORT=" + fmt.Sprintf("%d", a.PreferredPort),
			"GRADIO_SERVER_PORT=" + fmt.Sprintf("%d", a.PreferredPort),
			"STREAMLIT_SERVER_PORT=" + fmt.Sprintf("%d", a.PreferredPort),
		},
	}
}

func (a *App) makeVenv() error {
	if !a.isPython() {
		return nil
	}
	fmt.Println("Setting up Python venv")
	venv := a.pythonVenvPath()

	cmdOptions := a.pythonCmdOptions()
	fmt.Println("Command options", cmdOptions)
	a.Supervisor.Add("setupVenv", "python", []string{"-m", "venv", venv.VenvDir},
		cmdOptions)
	a.Supervisor.SuperviseAll()
	a.Supervisor.Remove("setupVenv")
	return nil
}

func (a *App) installRequirementsTxt() error {
	if !a.isPython() {
		return nil
	}
	fmt.Println("Installing requirements.txt")
	cmdOptions := a.pythonCmdOptions()
	commandStr := fmt.Sprintf("pip install -r %s/requirements.txt", a.RootDir)
	sourcedCmd := fmt.Sprintf("source %s && %s", a.pythonVenvPath().ActivatePath, commandStr)
	a.Supervisor.Add("installRequirements", "/bin/bash", []string{"-c", sourcedCmd},
		cmdOptions)
	a.Supervisor.SuperviseAll()
	a.Supervisor.Remove("installRequirements")
	return nil
}

func (a *App) showPythonExeLocation() error {
	if !a.isPython() {
		return nil
	}
	fmt.Println("Displaying python executable location")
	cmdOptions := a.pythonCmdOptions()
	a.Supervisor.Add("locatePython", "python", []string{"-c", "import sys; print(sys.executable)"},
		cmdOptions)
	a.Supervisor.SuperviseAll()
	a.Supervisor.Remove("locatePython")
	return nil
}

func (a *App) pipList() error {
	if !a.isPython() {
		return nil
	}
	fmt.Println("Displaying installed packages")
	cmdOptions := a.pythonCmdOptions()
	commandStr := "pip list"
	sourcedCmd := fmt.Sprintf("source %s && %s", a.pythonVenvPath().ActivatePath, commandStr)
	a.Supervisor.Add("showPackages", "/bin/bash", []string{"-c", sourcedCmd},
		cmdOptions)
	a.Supervisor.SuperviseAll()
	a.Supervisor.Remove("showPackages")
	return nil
}

func (a *App) setupPython() error {
	if !a.isPython() {
		return nil
	}
	fmt.Println("Setting up Python app")
	err := a.makeVenv()
	if err != nil {
		fmt.Println("Error setting up venv")
		return err
	}
	fmt.Println("Setting up requirements.txt")
	err = a.installRequirementsTxt()
	if err != nil {
		fmt.Println("Error installing requirements.txt")
		return err
	}
	fmt.Println("Showing python executable location")
	err = a.showPythonExeLocation()
	if err != nil {
		fmt.Println("Error showing python executable location")
		return err
	}
	fmt.Println("Listing installed packages")
	err = a.pipList()
	if err != nil {
		fmt.Println("Error listing installed packages")
		return err
	}
	return nil
}

func (a *App) setup() error {
	fmt.Println("Setting up app")
	a.UpdateStatus(StatusSetup)
	if a.isPython() {
		return a.setupPython()
	}
	return nil
}

func (a *App) startPython() error {
	if !a.isPython() {
		return nil
	}
	err := a.setup()
	if err != nil {
		fmt.Println("Error setting up app")
		return err
	}
	fmt.Println("Starting app command")
	cmdOptions := a.pythonCmdOptions()
	a.UpdateStatus(StatusRunning)
	commandStr := strings.Join(a.Command, " ")
	sourcedCmd := fmt.Sprintf("source %s && %s", a.pythonVenvPath().ActivatePath, commandStr)
	a.Supervisor.Add(a.ID, "/bin/bash", []string{"-c", sourcedCmd},
		cmdOptions)
	a.Supervisor.SuperviseAll()
	a.UpdateStatus(StatusTerminated)
	return nil
}

func (a *App) Start() error {
	if a.Status != StatusTerminated {
		return errors.New("app is already running or starting to run")
	}
	if a.Supervisor == nil {
		a.newSuperVisor()
	}
	err := KillPort(a.PreferredPort)
	if err != nil {
		fmt.Println("Error killing port", err)
	}
	fmt.Println("Starting app")
	a.UpdateStatus(StatusStarting)
	go func() {
		if a.isPython() {
			err := a.startPython()
			if err != nil {
				return
			}
		}
	}()
	return nil
}

func (a *App) Logs() string {
	return a.LogLines.String()
}

func (a *App) Stop() {
	fmt.Println("Stopping app")
	for _, supervisedApp := range a.Supervisor.ListAll() {
		err := a.Supervisor.Stop(supervisedApp)
		if err != nil {
			fmt.Println("Error stopping app", err)
		}
		a.Supervisor.Remove(supervisedApp)
	}
	a.Supervisor.StopAll(true)
	a.Supervisor.UnWatchLogs(a.LogChan)
	a.Supervisor = nil
	a.UpdateStatus(StatusTerminated)
}
