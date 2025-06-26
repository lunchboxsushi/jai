# JAI - Jira As Interface

A CLI-first, markdown-native workflow tool for managing Jira tickets with speed, clarity, and flow.

## 🔥 Idea

`jai` is a local-first CLI tool that:

* Lets developers write tasks and sub-tasks in markdown
* Auto-enriches raw task descriptions into manager-optimized Jira tickets
* Tracks current working context (epic, task, subtask) for seamless sub-task creation
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
# ... an editor opens to draft the epic. Save and close when done.
# The epic is enriched by AI, reviewed, and a Jira ticket is created.
# Context is set to this epic.

# 2. Create a new task under the current epic
jai task
# ... editor opens, add info/title, save and close.
# AI enriches, Jira ticket is created, and context is set to this task.

# 3. Create a new sub-task under the current task
jai subtask
# ... editor opens, add info/title, save and close.
# AI enriches, Jira ticket is created, and context is set to this subtask.

# 4. Focus on an existing epic, task, or subtask
jai focus "SRE-1234"       # Fuzzy match on key or title, e.g jai focus "1234" 

# 5. Check your current context
jai status
```

## 🔄 Workflow Summary

The core workflow is designed for speed and minimal friction. The `epic`, `task`, and `subtask` commands handle the entire lifecycle from drafting to Jira creation, with context automatically updated for you.

### Typical Flow for New Work

1. `jai epic` — Draft a new epic in your editor.
2. Add info and title, then save and close the editor.
3. The epic is AI-enriched and reviewed (if enabled).
4. A Jira ticket is created for the epic.
5. Context is set to this epic (for status/focus).

Repeat the same process for `jai task` (under the current epic) and `jai subtask` (under the current task).

> **Note:** Most developers primarily create and work on subtasks, as tasks and epics are often owned at a higher level.

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

- `epic` - Create a new epic. Opens an editor for drafting, enriches with AI, and creates a Jira ticket. **No arguments.**
- `task` - Create a new task under the current epic. Same workflow. **No arguments.**
- `subtask` - Create a new sub-task under the current task. **No arguments.**

### Context Management

- `focus <query>` - Set current context by fuzzy-matching an epic, task, or subtask title or key.
- `status` - Show the current focused epic, task, and subtask.
- `open [level]` - Open the current focus item (or specified level) in Jira browser.

### Configuration

- `config init` - Initialize a new configuration file.
- `config show` - Show the current configuration.

### Import & Export

- `import <ticket-id>` - Import a Jira ticket and its hierarchy (parents/children) as markdown files.

## 🗂️ Project Structure

```text
~/.local/share/jai/
├── tickets/                           # All epics/tasks/subtasks go here
│   ├── observability-refactor.md      # Epic + tasks + subtasks (Markdown)
│   ├── sso-cleanup.md                 # Another epic/task set
│   ├── inbox.md                       # Quick capture area
│   └── _archive/                      # Closed/deprecated tickets
│       └── 2024-old-epic.md
├── current.json                       # Current epic/task/subtask focus
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

## 🕵️ Review Page Example

Before a Jira ticket is created (if review is enabled), you'll see a review page like this in your editor:

**Epic Review Example:**
```markdown
# Review Epic Before Creating Jira Ticket

File: /path/to/OBS-123-Observability-Refactor.md

## Epic Content to be Created:
### Title
Observability Refactor

### Content
Epic description and context...

---
Review the epic above. The epic will be added to the file and a Jira epic will be created.
Save and exit to proceed, or delete all content to cancel.
```

**Task Review Example:**
```markdown
# Review Task Before Creating Jira Ticket

File: /path/to/OBS-456-Implement-distributed-tracing.md

## Task Content to be Created:
### Title
Implement distributed tracing

### Content
Task description with acceptance criteria...

---
Review the task above. The task will be created as a separate file and a Jira ticket will be created.
Save and exit to proceed, or delete all content to cancel.
```

**Subtask Review Example:**
```markdown
# Review Subtask Before Creating Jira Ticket

File: /path/to/OBS-457-Set-up-Jaeger.md

## Subtask Content to be Created:
### Title
Set up Jaeger

### Content
Subtask details and implementation notes...

### Parent Task
OBS-456

---
Review the subtask above. The subtask will be created as a separate file and a Jira ticket will be created.
Save and exit to proceed, or delete all content to cancel.
```

## 🏷️ Metadata Section

Each ticket in a markdown file can include a metadata section to track important fields. This section appears after the ticket content and looks like this:

```markdown
---
*Metadata:*
- Key: OBS-123
- Status: In Progress
- Priority: High
- EpicKey: OBS-123         # For tasks, links to the parent epic
- ParentKey: OBS-456       # For subtasks, links to the parent task
- TaskKey: OBS-456         # (Alternative for subtasks, links to parent task)
---
```

**Metadata keys:**
- `Key`: The Jira key for this ticket (e.g., OBS-123)
- `Status`: The current status (e.g., To Do, In Progress, Done)
- `Priority`: Ticket priority (e.g., High, Medium, Low)
- `EpicKey`: For tasks, the parent epic's key
- `ParentKey`: For tasks, the parent epic; for subtasks, the parent task
- `TaskKey`: For subtasks, the parent task (alternative to ParentKey)

> The metadata section is used by JAI to track the full ticket hierarchy and sync with Jira. You can edit these fields manually if needed, but they are usually managed automatically.

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
  prompt_template: "templates/enrichment_prompt.txt"  # Path to custom prompt template

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

## 🛠️ Development

### Prerequisites

- Go 1.24+
- Jira Cloud instance
- OpenAI API key (or other AI provider)

### Building

```bash
# Build for current platform
# (from project root)
go install ./cmd/jai
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
- [ ] Advanced markdown templates
- [ ] CLI completion scripts

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

### Customizing AI Enrichment

JAI allows you to customize the AI enrichment prompts to match your team's needs:

1. **Default Template**: JAI creates a default prompt template at `~/.jai/templates/enrichment_prompt.txt`
2. **Template Variables**: Use `{{TICKET_TYPE}}`, `{{TITLE}}`, `{{RAW_CONTENT}}`, `{{CONTEXT}}` in your templates
3. **AI Expressions**: Use `{{expression}}` to have AI evaluate sub-queries (e.g., `{{give me 5 examples of infrastructure risks}}`)
4. **Custom Templates**: Set `ai.prompt_template` in your config to use a custom template file

**AI Expression Evaluation:**
AI expressions within `{{double braces}}` are evaluated separately and replaced with actual generated content, ensuring specific requests are fulfilled rather than just rephrased.

**Example custom expression in template:**
```
**Risk Assessment:**
{{What are the top 3 infrastructure risks for this type of change?}}

**Dependencies:**
{{List potential system dependencies for this task}}
```

The AI will evaluate these expressions and replace them with actual content. 