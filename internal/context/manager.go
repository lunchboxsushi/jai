package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lunchboxsushi/jai/internal/types"
)

// Manager handles the current working context
type Manager struct {
	contextPath string
	context     *types.Context
}

// NewManager creates a new context manager
func NewManager(dataDir string) *Manager {
	return &Manager{
		contextPath: filepath.Join(dataDir, "current.json"),
		context:     &types.Context{},
	}
}

// Load loads the current context from disk
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.contextPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No context file exists, start with empty context
			m.context = &types.Context{
				Updated: time.Now(),
			}
			return nil
		}
		return fmt.Errorf("failed to read context file: %w", err)
	}

	if err := json.Unmarshal(data, m.context); err != nil {
		return fmt.Errorf("failed to parse context file: %w", err)
	}

	return nil
}

// Save saves the current context to disk
func (m *Manager) Save() error {
	// Ensure directory exists
	dir := filepath.Dir(m.contextPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create context directory: %w", err)
	}

	m.context.Updated = time.Now()
	data, err := json.MarshalIndent(m.context, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	if err := os.WriteFile(m.contextPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write context file: %w", err)
	}

	return nil
}

// Get returns the current context
func (m *Manager) Get() *types.Context {
	return m.context
}

// SetEpic sets the current epic context
func (m *Manager) SetEpic(key, id string) error {
	m.context.EpicKey = key
	m.context.EpicID = id
	m.context.TaskKey = "" // Clear task context when switching epics
	m.context.TaskID = ""
	m.context.SubtaskKey = ""
	m.context.SubtaskID = ""
	return m.Save()
}

// SetEpicAndTask sets both epic and task context without clearing task
func (m *Manager) SetEpicAndTask(epicKey, epicID, taskKey, taskID string) error {
	m.context.EpicKey = epicKey
	m.context.EpicID = epicID
	m.context.TaskKey = taskKey
	m.context.TaskID = taskID
	m.context.SubtaskKey = ""
	m.context.SubtaskID = ""
	return m.Save()
}

// SetTask sets the current task context
func (m *Manager) SetTask(key, id string) error {
	m.context.TaskKey = key
	m.context.TaskID = id
	m.context.SubtaskKey = ""
	m.context.SubtaskID = ""
	return m.Save()
}

// SetSubtask sets the current subtask context
func (m *Manager) SetSubtask(key, id string) error {
	m.context.SubtaskKey = key
	m.context.SubtaskID = id
	return m.Save()
}

// SetFullContext sets the entire context in one go.
func (m *Manager) SetFullContext(epicKey, epicID, taskKey, taskID, subtaskKey, subtaskID string) error {
	m.context.EpicKey = epicKey
	m.context.EpicID = epicID
	m.context.TaskKey = taskKey
	m.context.TaskID = taskID
	m.context.SubtaskKey = subtaskKey
	m.context.SubtaskID = subtaskID
	return m.Save()
}

// Clear clears the current context
func (m *Manager) Clear() error {
	m.context.EpicKey = ""
	m.context.EpicID = ""
	m.context.TaskKey = ""
	m.context.TaskID = ""
	m.context.SubtaskKey = ""
	m.context.SubtaskID = ""
	return m.Save()
}

// HasEpic returns true if an epic is set
func (m *Manager) HasEpic() bool {
	return m.context.EpicKey != ""
}

// HasTask returns true if a task is set
func (m *Manager) HasTask() bool {
	return m.context.TaskKey != ""
}

// HasSubtask returns true if a subtask is set
func (m *Manager) HasSubtask() bool {
	return m.context.SubtaskKey != ""
}

// GetEpicKey returns the current epic key
func (m *Manager) GetEpicKey() string {
	return m.context.EpicKey
}

// GetTaskKey returns the current task key
func (m *Manager) GetTaskKey() string {
	return m.context.TaskKey
}

// GetSubtaskKey returns the current subtask key
func (m *Manager) GetSubtaskKey() string {
	return m.context.SubtaskKey
}

// String returns a string representation of the current context
func (m *Manager) String() string {
	if m.context.EpicKey == "" && m.context.TaskKey == "" {
		return "No context set"
	}

	result := ""
	if m.context.EpicKey != "" {
		result += fmt.Sprintf("Epic: %s", m.context.EpicKey)
	}
	if m.context.TaskKey != "" {
		if result != "" {
			result += " → "
		}
		result += fmt.Sprintf("Task: %s", m.context.TaskKey)
	}
	if m.context.SubtaskKey != "" {
		if result != "" {
			result += " → "
		}
		result += fmt.Sprintf("Subtask: %s", m.context.SubtaskKey)
	}

	return result
}
