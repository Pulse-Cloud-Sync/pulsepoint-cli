# PulsePoint - Cloud Sync CLI Tool

[![Go Version](https://img.shields.io/badge/go-1.23%2B-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/pulsepoint/pulsepoint)

PulsePoint is a powerful, production-ready CLI tool for real-time file synchronization between local directories and cloud storage providers. Built with Go for performance and reliability, PulsePoint monitors your files and keeps them perfectly synchronized with your cloud storage.

## ✨ Features

- **🔄 Real-time Synchronization**: Instantly sync file changes to the cloud
- **☁️ Cloud Provider Support**: Currently supports Google Drive (more coming soon)
- **🔐 Secure Authentication**: OAuth2 with secure token storage
- **📁 Smart File Monitoring**: Efficient file system watching with ignore patterns
- **⚔️ Conflict Resolution**: Multiple strategies for handling sync conflicts
- **🎯 Flexible Sync Strategies**: One-way, mirror, and backup modes
- **📊 Performance Optimized**: Concurrent operations, smart batching, and caching
- **🔧 Highly Configurable**: YAML configuration with environment variable support
- **📝 Comprehensive Logging**: Structured logging with rotation
- **🚀 Single Binary**: Easy deployment with no dependencies

## 📦 Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/pulsepoint/pulsepoint.git
cd pulsepoint

# Build the binary
make build

# Install to system (optional)
make install
```

### Binary Releases

Download the latest release for your platform from the [releases page](https://github.com/pulsepoint/pulsepoint/releases).

```bash
# Linux/macOS
chmod +x pulsepoint
sudo mv pulsepoint /usr/local/bin/

# Verify installation
pulsepoint --version
```

## 🚀 Quick Start

### 1. Initialize Configuration

```bash
pulsepoint init
```

This creates a configuration file at `~/.pulsepoint/config.yaml`.

### 2. Authenticate with Google Drive

First, set up Google Cloud credentials:
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Drive API
4. Create OAuth2 credentials (Desktop application type)
5. Download the credentials JSON file

Then authenticate PulsePoint:

**📖 See [Google Drive Setup Guide](docs/GOOGLE_DRIVE_SETUP.md) for detailed instructions.**

Quick setup:
```bash
# Run the interactive setup script
./scripts/setup-google-auth.sh

# Then authenticate
pulsepoint auth google
```

Alternative methods:
```bash
# Using credentials file
pulsepoint auth google --credentials /path/to/credentials.json

# Or using environment variables
export GOOGLE_CLIENT_ID="your-client-id"
export GOOGLE_CLIENT_SECRET="your-client-secret"
pulsepoint auth google
```

### 3. Start Syncing

```bash
# One-time sync
pulsepoint sync /path/to/local/folder

# Continuous monitoring and sync
pulsepoint pulse /path/to/local/folder

# With specific sync strategy
pulsepoint sync /path/to/folder --strategy mirror

# Dry run to preview changes
pulsepoint sync /path/to/folder --dry-run
```

## 📖 Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `pulsepoint init` | Initialize configuration |
| `pulsepoint auth <provider>` | Authenticate with cloud provider |
| `pulsepoint sync <path>` | Perform one-time synchronization |
| `pulsepoint pulse <path>` | Start continuous monitoring and sync |
| `pulsepoint status` | Show current sync status |
| `pulsepoint list` | List synced files |
| `pulsepoint config` | Manage configuration |
| `pulsepoint logs` | View sync logs |

### Authentication Options

```bash
# Check authentication status
pulsepoint auth google --status

# Revoke authentication
pulsepoint auth google --revoke

# Use specific credentials file
pulsepoint auth google --credentials /path/to/creds.json
```

### Sync Options

```bash
# Sync strategies
pulsepoint sync /path --strategy one-way    # Local to remote only (default)
pulsepoint sync /path --strategy mirror     # Exact copy, delete extras
pulsepoint sync /path --strategy backup     # Preserve all versions

# Conflict resolution
pulsepoint sync /path --conflict keep-local   # Local wins (default)
pulsepoint sync /path --conflict keep-remote  # Remote wins
pulsepoint sync /path --conflict keep-both    # Rename conflicts
pulsepoint sync /path --conflict skip         # Skip conflicts

# Other options
pulsepoint sync /path --force      # Force sync even if no changes
pulsepoint sync /path --full       # Full sync instead of incremental
pulsepoint sync /path --dry-run    # Preview without making changes
pulsepoint sync /path --workers 8  # Number of concurrent workers
```

## ⚙️ Configuration

PulsePoint uses a layered configuration system:

1. **Default Configuration** (built-in)
2. **System Configuration** (`/etc/pulsepoint/config.yaml`)
3. **User Configuration** (`~/.pulsepoint/config.yaml`)
4. **Environment Variables** (PULSEPOINT_*)
5. **Command-line Flags** (highest priority)

### Configuration File Example

```yaml
# ~/.pulsepoint/config.yaml

# Sync configuration
sync:
  interval: 5m
  batch_size: 100
  max_concurrent: 4
  retry_attempts: 3
  retry_delay: 2s
  conflict_resolution: keep-local

# File handling
files:
  max_file_size: 1073741824  # 1GB
  preserve_timestamps: true
  preserve_permissions: false
  ignore_patterns:
    - "*.tmp"
    - "*.cache"
    - ".DS_Store"
    - "Thumbs.db"

# Provider configuration
providers:
  google:
    configured: true
    credentials_file: ~/.pulsepoint/credentials/google_credentials.json
    token_file: ~/.pulsepoint/tokens/google_token.json
    simple_upload_threshold: 5242880      # 5MB
    resumable_upload_threshold: 104857600 # 100MB
    chunk_size: 8388608                   # 8MB
    max_retries: 3

# Logging
logging:
  level: info
  file: ~/.pulsepoint/logs/pulsepoint.log
  max_size: 100  # MB
  max_backups: 5
  max_age: 30    # days

# Database
database:
  path: ~/.pulsepoint/pulsepoint.db
  backup_enabled: true
  backup_interval: 24h
```

### Environment Variables

```bash
# Authentication
export GOOGLE_CLIENT_ID="your-client-id"
export GOOGLE_CLIENT_SECRET="your-client-secret"
export GOOGLE_CREDENTIALS_FILE="/path/to/credentials.json"
export GOOGLE_TOKEN_FILE="/path/to/token.json"

# PulsePoint configuration
export PULSEPOINT_CONFIG="/custom/config.yaml"
export PULSEPOINT_LOG_LEVEL="debug"
export PULSEPOINT_DB_PATH="/custom/db/path.db"
```

## 📁 File Ignore Patterns

PulsePoint supports `.gitignore`-style patterns for excluding files from sync:

### .pulseignore File

Create a `.pulseignore` file in your sync directory:

```gitignore
# Temporary files
*.tmp
*.cache
*.swp
*~

# OS files
.DS_Store
Thumbs.db
desktop.ini

# Development
node_modules/
.git/
.venv/
__pycache__/
*.pyc

# Sensitive files
.env
secrets.yaml
*.key
*.pem

# Large files
*.iso
*.dmg
*.zip
*.tar.gz

# Custom patterns
build/
dist/
logs/
```

## 🎯 Sync Strategies

### One-Way Sync (Default)
- Syncs files from local to remote only
- Remote changes are not pulled down
- Best for: Backup scenarios, publishing workflows

### Mirror Sync
- Creates an exact copy in the remote
- Deletes remote files not present locally
- Best for: Exact replicas, deployment scenarios

### Backup Sync
- Never deletes any files
- Preserves all versions
- Best for: Archival, version preservation

## ⚔️ Conflict Resolution

PulsePoint automatically detects and resolves conflicts:

| Strategy | Description | Use Case |
|----------|-------------|----------|
| `keep-local` | Local version wins | Development work |
| `keep-remote` | Remote version wins | Collaboration |
| `keep-both` | Rename and keep both | Important files |
| `skip` | Skip conflicted files | Manual review |

## 📊 Performance

PulsePoint is optimized for performance:

- **Concurrent Operations**: Parallel uploads/downloads
- **Smart Batching**: Groups small files for efficiency
- **Delta Sync**: Only syncs changed portions (coming in v2)
- **Caching**: Metadata caching reduces API calls
- **Rate Limiting**: Respects API quotas automatically

### Performance Tuning

```yaml
# For large file sets
sync:
  batch_size: 500
  max_concurrent: 8

# For slow networks
providers:
  google:
    chunk_size: 4194304  # 4MB chunks
    max_retries: 5
```

## 🔍 Troubleshooting

### Common Issues

#### Authentication Failed
```bash
# Check credentials
pulsepoint auth google --status

# Re-authenticate
pulsepoint auth google --revoke
pulsepoint auth google
```

#### Sync Not Working
```bash
# Check logs
pulsepoint logs

# Enable debug logging
pulsepoint sync /path --verbose

# Verify file patterns
cat .pulseignore
```

#### Permission Errors
```bash
# Check file permissions
ls -la ~/.pulsepoint/

# Fix permissions
chmod 700 ~/.pulsepoint
chmod 600 ~/.pulsepoint/tokens/*
```

### Debug Mode

Enable detailed logging for troubleshooting:

```bash
# Command-line
pulsepoint sync /path -v

# Environment
export PULSEPOINT_LOG_LEVEL=debug

# Configuration
logging:
  level: debug
```

## 🔐 Security

PulsePoint prioritizes security:

- **Token Security**: Tokens stored with 0600 permissions
- **No Credential Logging**: Sensitive data never logged
- **OAuth2 Standards**: Full RFC 6749 compliance
- **State Validation**: CSRF protection in OAuth flow
- **Encrypted Storage**: Token encryption at rest

## 🧪 Testing

Run the test suite:

```bash
# All tests
make test

# Unit tests only
make test-unit

# Integration tests
make test-integration

# With coverage
make test-coverage
```

## 📈 Monitoring

Monitor PulsePoint operations:

```bash
# Current status
pulsepoint status

# Statistics
pulsepoint status --stats

# Watch mode
watch -n 5 pulsepoint status
```

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
# Development setup
git clone https://github.com/pulsepoint/pulsepoint.git
cd pulsepoint
make dev

# Run tests
make test

# Build
make build
```

## 📝 License

PulsePoint is released under the MIT License. See [LICENSE](LICENSE) for details.

## 🚀 Roadmap

### Current Version (v1.0)
- ✅ Google Drive integration
- ✅ Real-time file monitoring
- ✅ Multiple sync strategies
- ✅ Conflict resolution
- ✅ OAuth2 authentication

### Version 2.0 (Planned)
- 🔄 Bidirectional synchronization
- 📦 Dropbox integration
- ☁️ OneDrive integration
- 🗂️ AWS S3 support
- 🔒 Client-side encryption
- 📊 Web dashboard
- 🔄 Delta synchronization
- 📱 Mobile companion app

## 💬 Support

- **Documentation**: [docs.pulsepoint.io](https://docs.pulsepoint.io)
- **Issues**: [GitHub Issues](https://github.com/pulsepoint/pulsepoint/issues)
- **Discussions**: [GitHub Discussions](https://github.com/pulsepoint/pulsepoint/discussions)
- **Email**: support@pulsepoint.io

## 🙏 Acknowledgments

PulsePoint is built with these excellent libraries:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [Zap](https://github.com/uber-go/zap) - Structured logging
- [BoltDB](https://github.com/etcd-io/bbolt) - Embedded database
- [fsnotify](https://github.com/fsnotify/fsnotify) - File system monitoring
- [Google API Go Client](https://github.com/googleapis/google-api-go-client) - Google Drive integration

---

**PulsePoint** - Keep your files in perfect sync, effortlessly. 🔄