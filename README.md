# JAI - Jira As Interface

A CLI-first, markdown-native workflow tool for managing Jira tickets with speed, clarity, and flow.

## 🔥 Idea

`jai` is a local-first CLI tool that:

* Lets developers write tasks and sub-tasks in markdown
* Auto-enriches raw task descriptions into manager-optimized Jira tickets
* Tracks current working context (epic, task) for seamless sub-task creation
* Syncs local markdown files with Jira to reflect status, updates, and structure
* Eliminates click-heavy Jira workflows
* Prioritizes **speed and flow** — minimal typing, no context switching, keyboard-native

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/lunchboxsushi/jai.git
cd jai

# Build the binary
go build -o jai cmd/jai/main.go

# Install (optional)
sudo cp jai /usr/local/bin/
```

### Initial Setup

```bash
# Initialize configuration
jai config init

# Edit the configuration file with your settings
# ~/.jai/config.yaml
```

### Basic Workflow

```bash
# Set epic context
jai epic "SRE-5912"

# Create a new task
jai task

# Add a subtask to current task
jai subtask

# Quick append
jai new "fix login bug"
```

## 🔁 Workflow Summary

### Starting Work on a New Task

```bash
jai epic "SRE-5912"     # Set current epic context
jai task                 # Open new task draft under current epic
jai enrich               # Auto-enrich the task with polished manager-facing language
jai create               # Review (optional), then create Jira ticket
```

### Adding a Sub-task to Current Task

```bash
jai subtask              # Draft and enrich a sub-task under current focused task
```

### Quick Append via `new`

```bash
jai new                  # Append new task/sub-task to current context file
```

### Focusing Existing Tickets

```bash
jai focus "SRE-1234"     # Set focus using Jira ID or fuzzy title match
```

### Syncing

```bash
jai sync --status        # Pull updated Jira ticket status into local markdown
```

### Capturing Ideas

```bash
jai capture "explore OpenPipeline perf boost"
```

## 📖 Commands

### Core Commands

- `epic [title|key]` - Set or switch current epic context
- `task` - Add a new task under the current epic
- `subtask` - Add a new sub-task under the current task
- `new [content]` - Quickly append a task or sub-task to current context

### Context Management

- `focus <query>` - Set current context by fuzzy-matching epic/task title
- `unfocus` - Clear current context
- `status` - Show current focus and context

### Configuration

- `config init` - Initialize configuration
- `config show` - Show current configuration
- `config set <key> <value>` - Set configuration value

## 📁 Project Structure

```text
~/.local/share/jai/
├── tickets/                           # All epics/tasks/subtasks go here
│   ├── observability-refactor.md      # Epic + tasks + subtasks (Markdown)
│   ├── sso-cleanup.md                 # Another epic/task set
│   ├── inbox.md                       # Quick capture area
│   └── _archive/                      # Closed/deprecated tickets
│       └── 2024-old-epic.md
├── current.json                       # Current epic/task focus
├── config.json                        # Config options (e.g. reviewBeforeCreate)
└── templates/
    ├── default_epic.md
    ├── default_task.md
    └── default_subtask.md
```

## 📝 Markdown Format

JAI uses a specific markdown format for tickets:

```markdown
# epic: Observability Refactor [OBS-123]

Epic description and context...

## task: Implement distributed tracing [OBS-456]

Task description with acceptance criteria...

### subtask: Set up Jaeger [OBS-457]

Subtask details and implementation notes...
```

## ⚙️ Configuration

Create a configuration file at `~/.jai/config.yaml`:

```yaml
jira:
  url: "https://company.atlassian.net"
  username: "your-email@company.com"
  token: "your-api-token"
  project: "PROJ"

ai:
  provider: "openai"
  api_key: "your-openai-api-key"
  model: "gpt-3.5-turbo"
  max_tokens: 500

general:
  data_dir: "~/.local/share/jai"
  review_before_create: false
  default_editor: "vim"
```

### Environment Variables

You can also use environment variables:

```bash
export JAI_JIRA_URL="https://company.atlassian.net"
export JAI_JIRA_USERNAME="your-email@company.com"
export JAI_JIRA_TOKEN="your-api-token"
export JAI_AI_API_KEY="your-openai-api-key"
```

## 🛠 Development

### Prerequisites

- Go 1.24+
- Jira Cloud instance
- OpenAI API key (or other AI provider)

### Building

```bash
# Build for current platform
go build -o jai cmd/jai/main.go

# Build for specific platform
GOOS=linux GOARCH=amd64 go build -o jai-linux cmd/jai/main.go
GOOS=darwin GOARCH=amd64 go build -o jai-mac cmd/jai/main.go
```

### Project Structure

```
jai/
├── cmd/
│   ├── jai/
│   │   └── main.go          # Entry point
│   ├── root.go              # Root command
│   ├── epic.go              # Epic command
│   ├── task.go              # Task command
│   ├── subtask.go           # Subtask command
│   ├── focus.go             # Focus command
│   ├── new.go               # New command
│   ├── config.go            # Config command
│   └── status.go            # Status command
├── internal/
│   ├── types/
│   │   └── types.go         # Core data structures
│   ├── context/
│   │   └── manager.go       # Context management
│   ├── markdown/
│   │   └── parser.go        # Markdown parsing
│   ├── ai/
│   │   └── enrichment.go    # AI enrichment
│   └── jira/
│       └── client.go        # Jira integration
├── go.mod                   # Go module
└── README.md               # This file
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

MIT License - see LICENSE file for details.

## 🎯 Roadmap

- [ ] Sync command implementation
- [ ] Import existing Jira tickets
- [ ] Interactive ticket selection
- [ ] More AI providers (Anthropic, etc.)
- [ ] Webhook support for real-time sync
- [ ] Team collaboration features
- [ ] Advanced markdown templates
- [ ] CLI completion scripts
- [ ] Docker support
- [ ] CI/CD pipeline

## 🙏 Acknowledgments

- Inspired by the need for better developer workflows
- Built with Go and the excellent Cobra CLI framework
- Uses OpenAI for intelligent task enrichment
- Integrates with Atlassian Jira Cloud API 