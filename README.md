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

# Install the binary
go install ./cmd/jai
```
> **Note:** Ensure your Go binary path (`$GOPATH/bin` or `$HOME/go/bin`) is in your system's `PATH`.

### Initial Setup

```bash
# Initialize configuration
jai config init

# Edit the configuration file with your settings: ~/.jai/config.yaml
```

### Basic Workflow

```bash
# 1. Create a new epic
jai epic
# ... an editor opens to draft the epic.
# After saving, the epic is enriched by AI and a Jira ticket is created.

# 2. Or, focus on an existing epic
jai focus "Name of my epic" # Fuzzy match on title
jai focus "SRE-1234"       # Exact match on key

# 3. Create a new task under the current epic
jai task
# ... editor opens, AI enriches, Jira ticket created, and focus is set.

# 4. Create a new sub-task
jai subtask
# ... same workflow, linked to the parent task.
```

## 🔁 Workflow Summary

The core workflow is designed to be fast and seamless. The `epic`, `task`, and `subtask` commands handle the entire lifecycle from drafting to Jira creation.

### Starting Work

```bash
# To work on a new feature, start by creating an epic:
jai epic

# To work on an existing epic, focus it first:
jai focus "SRE-1234"

# With an epic in context, create a task:
jai task

# With a task in context, create a sub-task:
jai subtask
```

### Disabling AI Enrichment or Jira Creation

You can skip the AI or Jira steps using flags:

```bash
# Create a task without AI enrichment
jai task --no-enrich

# Create an epic locally without creating a Jira ticket
jai epic --no-create
```

### Checking Your Context

At any time, see what you're focused on:

```bash
jai status
```

## 📖 Commands

### Core Commands

- `epic` - Create a new epic. Opens an editor for drafting, enriches with AI, and creates a Jira ticket.
- `task` - Create a new task under the current epic. Follows the same draft -> enrich -> create workflow.
- `subtask` - Create a new sub-task under the current task.

### Context Management

- `focus <query>` - Set current context by fuzzy-matching an epic/task title or key.
- `status` - Show the current focused epic and/or task.

### Configuration

- `config init` - Initialize a new configuration file.
- `config show` - Show the current configuration.

## �� Project Structure

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
  epic_link_field: customfield_XXXXX  # Replace XXXXX with your field ID

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

## Jira Epic Link Field Configuration

If your Jira instance uses a custom field for linking tasks to epics (common in Jira Cloud), you must specify the correct field ID in your configuration for proper epic-task linking.

### How to Find Your Epic Link Field ID
1. Go to **Jira Administration → Issues → Custom Fields**.
2. Search for **Epic Link**.
3. Click the three dots (`...`) next to Epic Link and select **View field information** or **Configure**.
4. In the URL, look for a number at the end (e.g., `id=10009`).
5. Your field ID will be `customfield_10009` (replace `10009` with your value).

### Example Configuration
```yaml
jira:
  epic_link_field: customfield_XXXXX  # Replace XXXXX with your field ID
```

This is required for tasks to be properly linked to epics in Jira. If not set, you may see errors or tasks may not be linked to epics. 