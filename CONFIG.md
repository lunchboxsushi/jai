# JAI Configuration Guide

This document outlines all configuration options for the JAI CLI tool.

## 📁 Configuration File Location

The configuration file is located at:
- **Linux/macOS**: `~/.jai/config.yaml`
- **Windows**: `%USERPROFILE%\.jai\config.yaml`

## 🏠 Default Data Directory

JAI stores all local data in:
- **Linux**: `~/.local/share/jai/`
- **macOS**: `~/.local/share/jai/`
- **Windows**: `%USERPROFILE%\.local\share\jai\`

### Data Directory Structure

```
~/.local/share/jai/
├── tickets/                           # All epics/tasks/subtasks
│   ├── epic-key-1.md                 # Epic files (one per epic)
│   ├── epic-key-2.md
│   ├── inbox.md                      # Quick capture area
│   └── _archive/                     # Closed/deprecated tickets
├── current.json                      # Current working context
├── config.json                       # Runtime configuration
└── templates/                        # Markdown templates
    ├── default_epic.md
    ├── default_task.md
    └── default_subtask.md
```

## ⚙️ Configuration Options

### Complete Configuration Example

```yaml
# Jira Cloud Integration
jira:
  url: "https://your-company.atlassian.net"
  username: "your-email@company.com"
  project: "PROJ"
  # Note: token is NOT stored here - use JAI_JIRA_TOKEN environment variable

# AI Enrichment Settings
ai:
  provider: "openai"                  # AI provider: "openai", "anthropic", etc.
  model: "gpt-3.5-turbo"             # Model to use for enrichment
  max_tokens: 500                     # Maximum tokens for AI responses
  # Note: api_key is NOT stored here - use JAI_AI_TOKEN environment variable

# General Settings
general:
  data_dir: "~/.local/share/jai"     # Custom data directory (optional)
  review_before_create: false         # Ask for review before creating Jira tickets
  default_editor: "vim"              # Default editor for task drafting
```

### Jira Configuration

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `jira.url` | string | Yes | Your Jira Cloud instance URL |
| `jira.username` | string | Yes | Your Jira username/email |
| `jira.project` | string | Yes | Default project key for new tickets |
| `jira.token` | **environment only** | Yes | Your Jira API token (via `JAI_JIRA_TOKEN`) |
| `jira.epic_link_field` | string | No | Custom field ID for linking tasks to epics |

**Example:**
```yaml
jira:
  url: "https://acme.atlassian.net"
  username: "john.doe@acme.com"
  project: "SRE"
  # token: NOT stored in config file
  epic_link_field: customfield_XXXXX  # Replace XXXXX with your field ID
```

**Environment Variable:**
```bash
export JAI_JIRA_TOKEN="ATATT3xFfGF0..."  # Get from https://id.atlassian.com/manage-profile/security/api-tokens
```

### AI Configuration

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `ai.provider` | string | No | "openai" | AI provider to use |
| `ai.model` | string | No | "gpt-3.5-turbo" | Model to use for enrichment |
| `ai.max_tokens` | integer | No | 500 | Maximum tokens for AI responses |
| `ai.api_key` | **environment only** | Yes | - | API key for the AI provider (via `JAI_AI_TOKEN`) |

**Example:**
```yaml
ai:
  provider: "openai"
  model: "gpt-4"
  max_tokens: 1000
  # api_key: NOT stored in config file
```

**Environment Variable:**
```bash
export JAI_AI_TOKEN="sk-..."  # Get from https://platform.openai.com/api-keys
```

### General Configuration

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `general.data_dir` | string | No | `~/.local/share/jai` | Custom data directory |
| `general.review_before_create` | boolean | No | false | Ask for review before creating Jira tickets |
| `general.default_editor` | string | No | `$EDITOR` or "vim" | Default editor for task drafting |

**Example:**
```yaml
general:
  data_dir: "/custom/path/to/jai/data"
  review_before_create: true
  default_editor: "code"  # VS Code
```

## 🔧 Environment Variables

**Required Environment Variables (for security):**

| Purpose | Environment Variable | Example |
|---------|---------------------|---------|
| Jira API Token | `JAI_JIRA_TOKEN` | `export JAI_JIRA_TOKEN="ATATT3xFfGF0..."` |
| AI API Key | `JAI_AI_TOKEN` | `export JAI_AI_TOKEN="sk-..."` |

**Optional Environment Variables (override config):**

| Config Path | Environment Variable | Example |
|-------------|---------------------|---------|
| `jira.url` | `JAI_JIRA_URL` | `export JAI_JIRA_URL="https://acme.atlassian.net"` |
| `jira.username` | `JAI_JIRA_USERNAME` | `export JAI_JIRA_USERNAME="john.doe@acme.com"` |
| `jira.project` | `JAI_JIRA_PROJECT` | `export JAI_JIRA_PROJECT="SRE"` |
| `ai.provider` | `JAI_AI_PROVIDER` | `export JAI_AI_PROVIDER="openai"` |
| `ai.model` | `JAI_AI_MODEL` | `export JAI_AI_MODEL="gpt-4"` |
| `ai.max_tokens` | `JAI_AI_MAX_TOKENS` | `export JAI_AI_MAX_TOKENS="1000"` |
| `general.data_dir` | `JAI_GENERAL_DATA_DIR` | `export JAI_GENERAL_DATA_DIR="/custom/path"` |
| `general.review_before_create` | `JAI_GENERAL_REVIEW_BEFORE_CREATE` | `export JAI_GENERAL_REVIEW_BEFORE_CREATE="true"` |
| `general.default_editor` | `JAI_GENERAL_DEFAULT_EDITOR` | `export JAI_GENERAL_DEFAULT_EDITOR="code"` |

## 🚀 Quick Setup

### 1. Interactive Setup (Recommended)

```bash
jai init
```

This interactive wizard will:
- Create the configuration directory and file
- Prompt for Jira settings (URL, username, project)
- Prompt for AI settings (provider, model)
- Set up environment variable instructions
- Create initial data directories

### 2. Manual Setup

```bash
# Initialize configuration
jai config init

# Edit the configuration file
vim ~/.jai/config.yaml

# Set environment variables
export JAI_JIRA_TOKEN="your-jira-api-token"
export JAI_AI_TOKEN="your-openai-api-key"
```

### 3. Verify Configuration

```bash
jai status
```

This will show the status of your configuration and connections.

## 🔒 Security Notes

- **API Tokens**: Jira tokens and AI API keys are **NEVER** stored in configuration files
- **Environment Variables**: Sensitive values are handled via environment variables only
- **File Permissions**: The configuration file should have restricted permissions:
  ```bash
  chmod 600 ~/.jai/config.yaml
  ```
- **Shell Profile**: Add environment variables to your shell profile for persistence:
  ```bash
  echo 'export JAI_JIRA_TOKEN="your-token"' >> ~/.bashrc
  echo 'export JAI_AI_TOKEN="your-key"' >> ~/.bashrc
  source ~/.bashrc
  ```

## 🐛 Troubleshooting

### Common Issues

1. **"Jira configuration incomplete"**
   - Ensure Jira URL, username, and project are set in config
   - Verify `JAI_JIRA_TOKEN` environment variable is set
   - Check that your API token is valid

2. **"AI configuration incomplete"**
   - Ensure AI provider and model are set in config
   - Verify `JAI_AI_TOKEN` environment variable is set
   - Check that you have sufficient API credits

3. **"Permission denied"**
   - Ensure the data directory is writable
   - Check file permissions on the config file

### Debug Mode

Run with verbose output to see detailed information:

```bash
jai --verbose status
```

## 📝 Configuration Validation

JAI validates your configuration when you run commands. Common validation checks:

- Jira URL format (must be a valid Atlassian Cloud URL)
- API token format (Jira tokens start with "ATATT")
- OpenAI API key format (starts with "sk-")
- Data directory accessibility
- Editor availability

## 🔄 Configuration Reload

Configuration is loaded at startup. To reload configuration:

1. Edit the config file
2. Restart JAI
3. Or use `jai config show` to verify changes 

### Jira Epic Link Field

If your Jira instance uses a custom field for linking tasks to epics, you must specify the correct field ID in your configuration.

#### How to Find the Field ID
1. Go to Jira Administration → Issues → Custom Fields.
2. Search for 'Epic Link'.
3. Click the three dots (`...`) next to Epic Link and select 'View field information' or 'Configure'.
4. The URL will end with a number (e.g., `id=10009`).
5. Your field ID is `customfield_10009` (replace `10009` with your value).

#### Example
```yaml
jira:
  epic_link_field: customfield_XXXXX  # Replace XXXXX with your field ID
```

This is required for tasks to be properly linked to epics in Jira. 