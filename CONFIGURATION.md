# PulsePoint Configuration Reference

## Overview

PulsePoint uses a hierarchical configuration system that allows flexibility in how you configure the application. Configuration sources are merged in the following order (later sources override earlier ones):

1. **Built-in Defaults** - Sensible defaults compiled into the binary
2. **System Configuration** - `/etc/pulsepoint/config.yaml`
3. **User Configuration** - `~/.pulsepoint/config.yaml`
4. **Environment Variables** - `PULSEPOINT_*` variables
5. **Command-line Flags** - Highest priority

## Configuration File Format

PulsePoint uses YAML format for configuration files. The main configuration file is located at `~/.pulsepoint/config.yaml` by default.

## Complete Configuration Reference

```yaml
# PulsePoint Configuration File
# Version: 1.0

# ============================================================================
# SYNC CONFIGURATION
# ============================================================================
sync:
  # Sync interval for periodic synchronization
  # Format: duration string (e.g., "5m", "1h", "30s")
  # Default: 5m
  interval: 5m
  
  # Number of files to process in a single batch
  # Range: 1-1000
  # Default: 100
  batch_size: 100
  
  # Maximum number of concurrent sync operations
  # Range: 1-32
  # Default: 4
  max_concurrent: 4
  
  # Number of retry attempts for failed operations
  # Range: 0-10
  # Default: 3
  retry_attempts: 3
  
  # Delay between retry attempts
  # Format: duration string
  # Default: 2s
  retry_delay: 2s
  
  # Default conflict resolution strategy
  # Options: keep-local, keep-remote, keep-both, skip
  # Default: keep-local
  conflict_resolution: keep-local
  
  # Enable automatic sync on file changes
  # Default: true
  auto_sync: true
  
  # Minimum time between syncs for the same file
  # Format: duration string
  # Default: 1s
  debounce_interval: 1s

# ============================================================================
# FILE HANDLING
# ============================================================================
files:
  # Maximum file size to sync (in bytes)
  # Default: 1073741824 (1GB)
  # Set to 0 for no limit
  max_file_size: 1073741824
  
  # Preserve file timestamps during sync
  # Default: true
  preserve_timestamps: true
  
  # Preserve file permissions during sync
  # Default: false (Windows compatibility)
  preserve_permissions: false
  
  # Follow symbolic links
  # Default: false
  follow_symlinks: false
  
  # Hash algorithm for file integrity checking
  # Options: md5, sha1, sha256
  # Default: sha256
  hash_algorithm: sha256
  
  # File patterns to ignore (gitignore syntax)
  # These patterns are applied in addition to .pulseignore files
  ignore_patterns:
    - "*.tmp"
    - "*.cache"
    - "*.swp"
    - "*~"
    - ".DS_Store"
    - "Thumbs.db"
    - "desktop.ini"
    - ".git/"
    - "node_modules/"
    - "__pycache__/"
    - "*.pyc"
  
  # File patterns to explicitly include (overrides ignore patterns)
  include_patterns: []
    # - "*.important"
    # - "critical/**"

# ============================================================================
# PROVIDER CONFIGURATION
# ============================================================================
providers:
  # Google Drive Configuration
  google:
    # Whether Google Drive is configured and ready to use
    configured: false
    
    # Path to Google OAuth2 credentials JSON file
    # Can also be set via GOOGLE_CREDENTIALS_FILE environment variable
    credentials_file: ~/.pulsepoint/credentials/google_credentials.json
    
    # Path to store OAuth2 tokens
    # Can also be set via GOOGLE_TOKEN_FILE environment variable
    token_file: ~/.pulsepoint/tokens/google_token.json
    
    # Google Drive folder ID to use as root (optional)
    # Leave empty to use My Drive root
    root_folder_id: ""
    
    # OAuth2 scopes to request
    # Default includes full Drive access
    scopes:
      - https://www.googleapis.com/auth/drive
    
    # File size threshold for simple upload (in bytes)
    # Files smaller than this use simple upload
    # Default: 5242880 (5MB)
    simple_upload_threshold: 5242880
    
    # File size threshold for resumable upload (in bytes)
    # Files larger than this use resumable upload
    # Default: 104857600 (100MB)
    resumable_upload_threshold: 104857600
    
    # Chunk size for large file uploads (in bytes)
    # Default: 8388608 (8MB)
    chunk_size: 8388608
    
    # Maximum number of API retry attempts
    # Default: 3
    max_retries: 3
    
    # Rate limiting (requests per second)
    # Default: 10
    rate_limit: 10
  
  # Future provider configurations
  # dropbox:
  #   configured: false
  #   app_key: ""
  #   app_secret: ""
  #   token_file: ~/.pulsepoint/tokens/dropbox_token.json
  
  # onedrive:
  #   configured: false
  #   client_id: ""
  #   client_secret: ""
  #   token_file: ~/.pulsepoint/tokens/onedrive_token.json
  
  # s3:
  #   configured: false
  #   access_key: ""
  #   secret_key: ""
  #   bucket: ""
  #   region: us-east-1

# ============================================================================
# LOGGING CONFIGURATION
# ============================================================================
logging:
  # Log level
  # Options: debug, info, warn, error, fatal
  # Default: info
  level: info
  
  # Log output format
  # Options: json, console
  # Default: console
  format: console
  
  # Log file path
  # Leave empty to disable file logging
  # Default: ~/.pulsepoint/logs/pulsepoint.log
  file: ~/.pulsepoint/logs/pulsepoint.log
  
  # Maximum size of a log file before rotation (in MB)
  # Default: 100
  max_size: 100
  
  # Maximum number of old log files to keep
  # Default: 5
  max_backups: 5
  
  # Maximum age of log files in days
  # Default: 30
  max_age: 30
  
  # Compress rotated log files
  # Default: true
  compress: true
  
  # Include timestamp in log entries
  # Default: true
  timestamp: true
  
  # Include caller information in log entries
  # Default: false
  caller: false

# ============================================================================
# DATABASE CONFIGURATION
# ============================================================================
database:
  # Database file path
  # Default: ~/.pulsepoint/pulsepoint.db
  path: ~/.pulsepoint/pulsepoint.db
  
  # Enable database backups
  # Default: true
  backup_enabled: true
  
  # Backup interval
  # Format: duration string
  # Default: 24h
  backup_interval: 24h
  
  # Backup retention (number of backups to keep)
  # Default: 7
  backup_retention: 7
  
  # Database timeout for operations
  # Format: duration string
  # Default: 30s
  timeout: 30s
  
  # Enable write-ahead logging (improves performance)
  # Default: true
  wal_enabled: true
  
  # Auto-vacuum database (cleanup deleted data)
  # Default: true
  auto_vacuum: true
  
  # Vacuum interval
  # Format: duration string
  # Default: 168h (1 week)
  vacuum_interval: 168h

# ============================================================================
# PERFORMANCE TUNING
# ============================================================================
performance:
  # Enable caching for improved performance
  # Default: true
  enable_caching: true
  
  # Cache TTL (time-to-live)
  # Format: duration string
  # Default: 5m
  cache_ttl: 5m
  
  # Maximum memory usage for caching (in bytes)
  # Default: 104857600 (100MB)
  max_memory_usage: 104857600
  
  # Network bandwidth limit (bytes per second)
  # Set to 0 for no limit
  # Default: 0
  bandwidth_limit: 0
  
  # Enable compression for network transfers
  # Default: false
  enable_compression: false
  
  # Enable delta synchronization (only sync changes)
  # Default: false (coming in v2)
  enable_delta_sync: false
  
  # Number of worker threads for processing
  # Default: 4
  worker_threads: 4
  
  # Queue size for pending operations
  # Default: 1000
  queue_size: 1000

# ============================================================================
# MONITORING & METRICS
# ============================================================================
monitoring:
  # Enable metrics collection
  # Default: true
  enabled: true
  
  # Metrics collection interval
  # Format: duration string
  # Default: 30s
  interval: 30s
  
  # Export metrics to file
  # Leave empty to disable
  export_file: ~/.pulsepoint/metrics/metrics.json
  
  # Enable performance profiling
  # Default: false
  profiling: false
  
  # Profiling output directory
  profile_dir: ~/.pulsepoint/profiles

# ============================================================================
# ADVANCED SETTINGS
# ============================================================================
advanced:
  # Enable experimental features
  # Default: false
  experimental: false
  
  # API timeout for cloud provider operations
  # Format: duration string
  # Default: 30s
  api_timeout: 30s
  
  # Maximum idle connections for HTTP client
  # Default: 100
  max_idle_connections: 100
  
  # Idle connection timeout
  # Format: duration string
  # Default: 90s
  idle_connection_timeout: 90s
  
  # TLS handshake timeout
  # Format: duration string
  # Default: 10s
  tls_handshake_timeout: 10s
  
  # Enable HTTP/2
  # Default: true
  http2_enabled: true
  
  # DNS cache TTL
  # Format: duration string
  # Default: 5m
  dns_cache_ttl: 5m
```

## Environment Variables

All configuration options can be set via environment variables. The format is:
`PULSEPOINT_<SECTION>_<KEY>`

### Examples

```bash
# Sync configuration
export PULSEPOINT_SYNC_INTERVAL="10m"
export PULSEPOINT_SYNC_BATCH_SIZE="200"
export PULSEPOINT_SYNC_CONFLICT_RESOLUTION="keep-remote"

# File configuration
export PULSEPOINT_FILES_MAX_FILE_SIZE="2147483648"  # 2GB
export PULSEPOINT_FILES_PRESERVE_TIMESTAMPS="true"

# Provider configuration
export PULSEPOINT_PROVIDERS_GOOGLE_CONFIGURED="true"
export PULSEPOINT_PROVIDERS_GOOGLE_CREDENTIALS_FILE="/path/to/creds.json"

# Logging configuration
export PULSEPOINT_LOGGING_LEVEL="debug"
export PULSEPOINT_LOGGING_FILE="/var/log/pulsepoint/app.log"

# Database configuration
export PULSEPOINT_DATABASE_PATH="/var/lib/pulsepoint/data.db"
export PULSEPOINT_DATABASE_BACKUP_ENABLED="true"

# Performance configuration
export PULSEPOINT_PERFORMANCE_ENABLE_CACHING="true"
export PULSEPOINT_PERFORMANCE_WORKER_THREADS="8"
```

## Command-line Flags

Command-line flags take precedence over all other configuration sources.

### Global Flags

```bash
--config string       Config file path (default: ~/.pulsepoint/config.yaml)
--verbose, -v        Enable verbose output
--debug              Enable debug mode
--quiet, -q          Suppress non-error output
--no-color           Disable colored output
```

### Command-specific Flags

```bash
# Sync command
pulsepoint sync /path \
  --strategy mirror \
  --conflict keep-both \
  --workers 8 \
  --batch-size 200 \
  --force \
  --dry-run

# Auth command
pulsepoint auth google \
  --credentials /path/to/creds.json \
  --token-file /path/to/token.json \
  --status \
  --revoke

# Config command
pulsepoint config set sync.interval 10m
pulsepoint config get sync.interval
pulsepoint config list
```

## Configuration Validation

PulsePoint validates configuration on startup. Invalid configurations will result in an error message with details about what needs to be corrected.

### Validation Rules

1. **Duration strings** must be valid Go duration format (e.g., "5m", "1h30m")
2. **File paths** must be valid and accessible (for write operations)
3. **Numeric values** must be within acceptable ranges
4. **Enum values** must match predefined options
5. **Required fields** must be present

### Common Validation Errors

```yaml
# ❌ Invalid duration format
sync:
  interval: 5 minutes  # Should be: 5m

# ❌ Invalid enum value
sync:
  conflict_resolution: always-local  # Should be: keep-local

# ❌ Out of range value
sync:
  max_concurrent: 100  # Maximum is 32

# ❌ Invalid file path (no ~ expansion in some fields)
database:
  path: ~/data.db  # Should use absolute path or $HOME
```

## Best Practices

### 1. Environment-specific Configuration

Create separate configuration files for different environments:

```bash
# Development
pulsepoint --config ~/.pulsepoint/config.dev.yaml sync /path

# Production
pulsepoint --config ~/.pulsepoint/config.prod.yaml sync /path
```

### 2. Secure Credential Storage

Never store credentials directly in configuration files. Use:
- Environment variables for CI/CD
- Separate credential files with restricted permissions
- OS keychain integration (coming in v2)

### 3. Performance Tuning

For large file sets:
```yaml
sync:
  batch_size: 500
  max_concurrent: 8
performance:
  worker_threads: 8
  queue_size: 5000
```

For slow networks:
```yaml
sync:
  retry_attempts: 5
  retry_delay: 5s
providers:
  google:
    chunk_size: 4194304  # 4MB
```

For limited bandwidth:
```yaml
performance:
  bandwidth_limit: 1048576  # 1MB/s
  enable_compression: true
```

### 4. Logging Configuration

Development:
```yaml
logging:
  level: debug
  format: console
  caller: true
```

Production:
```yaml
logging:
  level: info
  format: json
  file: /var/log/pulsepoint/app.log
  max_size: 500
  max_backups: 10
```

## Migration from Older Versions

When upgrading PulsePoint, configuration migration may be required:

```bash
# Backup existing configuration
cp ~/.pulsepoint/config.yaml ~/.pulsepoint/config.yaml.backup

# Run migration
pulsepoint config migrate

# Verify configuration
pulsepoint config validate
```

## Troubleshooting Configuration Issues

### Debug Configuration Loading

```bash
# Show effective configuration
pulsepoint config show

# Show configuration sources
pulsepoint config sources

# Validate configuration
pulsepoint config validate
```

### Common Issues

1. **Configuration not loading**
   - Check file permissions: `ls -la ~/.pulsepoint/config.yaml`
   - Verify YAML syntax: `yamllint ~/.pulsepoint/config.yaml`

2. **Environment variables not working**
   - Check variable naming: Must start with `PULSEPOINT_`
   - Verify export: `env | grep PULSEPOINT`

3. **Unexpected behavior**
   - Check configuration precedence
   - Use `--config` flag to specify exact file
   - Enable debug logging to see configuration loading

---

For more information, see the [main documentation](README.md) or run `pulsepoint help config`.
