# PulsePoint Troubleshooting Guide

## Quick Diagnostics

Before diving into specific issues, run these diagnostic commands:

```bash
# Check version and build info
pulsepoint --version

# Verify configuration
pulsepoint config validate

# Check authentication status
pulsepoint auth google --status

# Test connectivity
pulsepoint status

# View recent logs
pulsepoint logs --tail 50
```

## Common Issues and Solutions

### 🔐 Authentication Issues

#### Problem: "Authentication failed" error

**Symptoms:**
- Cannot authenticate with Google Drive
- Token refresh fails
- "401 Unauthorized" errors

**Solutions:**

1. **Check credentials:**
```bash
# Verify credentials file exists
ls -la ~/.pulsepoint/credentials/google_credentials.json

# Check environment variables
env | grep GOOGLE_
```

2. **Re-authenticate:**
```bash
# Revoke existing authentication
pulsepoint auth google --revoke

# Clear token file manually if needed
rm ~/.pulsepoint/tokens/google_token.json

# Re-authenticate
pulsepoint auth google
```

3. **Verify OAuth2 settings in Google Cloud Console:**
- Ensure OAuth2 client is type "Desktop"
- Check that redirect URIs include `http://localhost:8080/callback`
- Verify Google Drive API is enabled

#### Problem: "Token expired" errors

**Solutions:**

1. **Check token validity:**
```bash
pulsepoint auth google --status
```

2. **Force token refresh:**
```bash
# Re-authenticate to get fresh tokens
pulsepoint auth google
```

3. **Check system time:**
```bash
# Ensure system time is correct
date
# Sync time if needed
sudo ntpdate -s time.nist.gov
```

---

### 📁 Sync Issues

#### Problem: Files not syncing

**Symptoms:**
- Files remain in "pending" status
- No upload activity
- Sync appears stuck

**Solutions:**

1. **Check sync status:**
```bash
pulsepoint status
pulsepoint list --pending
```

2. **Verify file patterns:**
```bash
# Check if files are ignored
cat .pulseignore
pulsepoint config get files.ignore_patterns
```

3. **Force sync:**
```bash
# Force full sync
pulsepoint sync /path --force --full

# Increase verbosity to see what's happening
pulsepoint sync /path --verbose
```

4. **Check file size limits:**
```bash
# Verify max file size setting
pulsepoint config get files.max_file_size

# Temporarily increase limit
pulsepoint sync /path --max-size 2GB
```

#### Problem: Sync conflicts

**Symptoms:**
- "Conflict detected" messages
- Files marked as conflicted
- Sync stops for certain files

**Solutions:**

1. **List conflicts:**
```bash
pulsepoint list --conflicts
```

2. **Resolve conflicts:**
```bash
# Auto-resolve with strategy
pulsepoint sync /path --conflict keep-local
pulsepoint sync /path --conflict keep-remote
pulsepoint sync /path --conflict keep-both

# Manual resolution
pulsepoint resolve /path/to/file --keep local
```

3. **Prevent future conflicts:**
```yaml
# config.yaml
sync:
  conflict_resolution: keep-local  # or keep-remote, keep-both
```

---

### 🔄 File Monitoring Issues

#### Problem: Changes not detected

**Symptoms:**
- File modifications not triggering sync
- Pulse command not responding to changes
- Watcher appears inactive

**Solutions:**

1. **Check watcher status:**
```bash
# Verify pulse is running
ps aux | grep pulsepoint

# Check if path is being watched
pulsepoint status --verbose
```

2. **Restart watcher:**
```bash
# Stop existing pulse
pkill -f "pulsepoint pulse"

# Start with debug output
pulsepoint pulse /path --verbose
```

3. **Check system limits:**
```bash
# Linux: Check inotify limits
cat /proc/sys/fs/inotify/max_user_watches

# Increase if needed
echo 524288 | sudo tee /proc/sys/fs/inotify/max_user_watches
```

4. **Verify ignore patterns:**
```bash
# Ensure files aren't ignored
pulsepoint config get files.ignore_patterns
```

---

### 🚀 Performance Issues

#### Problem: Slow sync speed

**Symptoms:**
- Uploads taking too long
- High memory usage
- System becomes unresponsive

**Solutions:**

1. **Optimize configuration:**
```yaml
# config.yaml
sync:
  batch_size: 50        # Reduce batch size
  max_concurrent: 2     # Reduce concurrent operations

performance:
  bandwidth_limit: 1048576  # Limit to 1MB/s
  worker_threads: 2         # Reduce workers
```

2. **Check resource usage:**
```bash
# Monitor PulsePoint
top -p $(pgrep pulsepoint)

# Check disk I/O
iotop -p $(pgrep pulsepoint)
```

3. **Clean up database:**
```bash
# Compact database
pulsepoint maintenance compact

# Clear old logs
pulsepoint maintenance clean-logs --older-than 30d
```

#### Problem: High memory usage

**Solutions:**

1. **Limit cache size:**
```yaml
performance:
  enable_caching: true
  max_memory_usage: 52428800  # 50MB limit
```

2. **Reduce batch processing:**
```bash
pulsepoint sync /path --batch-size 10 --workers 2
```

---

### 🗄️ Database Issues

#### Problem: Database corrupted

**Symptoms:**
- "Database error" messages
- Cannot read sync state
- Crash on startup

**Solutions:**

1. **Backup and recreate:**
```bash
# Backup existing database
cp ~/.pulsepoint/pulsepoint.db ~/.pulsepoint/pulsepoint.db.backup

# Remove corrupted database
rm ~/.pulsepoint/pulsepoint.db

# Reinitialize
pulsepoint init
```

2. **Restore from backup:**
```bash
# List available backups
ls -la ~/.pulsepoint/backups/

# Restore latest backup
cp ~/.pulsepoint/backups/pulsepoint.db.latest ~/.pulsepoint/pulsepoint.db
```

3. **Export and reimport:**
```bash
# Export state
pulsepoint export --output state.json

# Create new database
rm ~/.pulsepoint/pulsepoint.db
pulsepoint init

# Import state
pulsepoint import --input state.json
```

---

### 🌐 Network Issues

#### Problem: Connection timeouts

**Symptoms:**
- "Connection timeout" errors
- "Network unreachable" messages
- Intermittent failures

**Solutions:**

1. **Check connectivity:**
```bash
# Test Google API connectivity
curl -I https://www.googleapis.com/drive/v3/about

# Check DNS
nslookup googleapis.com
```

2. **Increase timeouts:**
```yaml
# config.yaml
advanced:
  api_timeout: 60s
  tls_handshake_timeout: 30s
```

3. **Configure proxy (if needed):**
```bash
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

4. **Retry configuration:**
```yaml
sync:
  retry_attempts: 5
  retry_delay: 10s
```

---

### 📝 Logging Issues

#### Problem: No logs or missing information

**Solutions:**

1. **Enable debug logging:**
```bash
# Via command line
pulsepoint sync /path --debug

# Via environment
export PULSEPOINT_LOGGING_LEVEL=debug

# Via config
pulsepoint config set logging.level debug
```

2. **Check log file location:**
```bash
# Default location
tail -f ~/.pulsepoint/logs/pulsepoint.log

# Check configured location
pulsepoint config get logging.file
```

3. **Fix log rotation issues:**
```yaml
logging:
  max_size: 100      # MB
  max_backups: 5
  max_age: 30        # days
  compress: false    # Disable if causing issues
```

---

## Platform-Specific Issues

### macOS

#### Problem: "Operation not permitted" errors

**Solution:**
```bash
# Grant full disk access
System Preferences > Security & Privacy > Privacy > Full Disk Access
# Add PulsePoint binary
```

#### Problem: Keychain access issues

**Solution:**
```bash
# Reset keychain access
security unlock-keychain ~/Library/Keychains/login.keychain
```

### Linux

#### Problem: Permission denied errors

**Solutions:**
```bash
# Fix permissions
chmod 700 ~/.pulsepoint
chmod 600 ~/.pulsepoint/tokens/*
chmod 600 ~/.pulsepoint/credentials/*
```

#### Problem: Too many open files

**Solution:**
```bash
# Increase file limits
ulimit -n 4096

# Permanent fix in /etc/security/limits.conf
* soft nofile 4096
* hard nofile 8192
```

### Windows

#### Problem: Path too long errors

**Solution:**
```powershell
# Enable long path support
New-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem" `
  -Name "LongPathsEnabled" -Value 1 -PropertyType DWORD -Force
```

#### Problem: Antivirus blocking operations

**Solution:**
- Add PulsePoint to antivirus exceptions
- Exclude ~/.pulsepoint directory from scanning

---

## Debug Mode

Enable comprehensive debugging:

```bash
# Maximum verbosity
PULSEPOINT_LOGGING_LEVEL=debug \
PULSEPOINT_LOGGING_CALLER=true \
pulsepoint sync /path --verbose --debug

# Save debug output
pulsepoint sync /path --debug 2>&1 | tee debug.log
```

### Debug Checklist

1. **Configuration:**
```bash
pulsepoint config show > config-dump.txt
pulsepoint config validate
```

2. **Authentication:**
```bash
pulsepoint auth google --status --verbose
```

3. **File System:**
```bash
ls -la ~/.pulsepoint/
df -h  # Check disk space
```

4. **Network:**
```bash
ping -c 4 googleapis.com
traceroute googleapis.com
```

5. **System Resources:**
```bash
free -h  # Memory
df -h    # Disk
uptime   # Load average
```

---

## Getting Help

### Collect Diagnostic Information

Before reporting an issue, collect:

```bash
# Create diagnostic bundle
pulsepoint support --bundle diagnostic.zip
```

This includes:
- Configuration (sanitized)
- Recent logs
- System information
- Error traces

### Log Locations

- **Logs**: `~/.pulsepoint/logs/`
- **Database**: `~/.pulsepoint/pulsepoint.db`
- **Config**: `~/.pulsepoint/config.yaml`
- **Tokens**: `~/.pulsepoint/tokens/` (DO NOT SHARE)
- **Credentials**: `~/.pulsepoint/credentials/` (DO NOT SHARE)

### Support Channels

1. **Documentation**: Check [README.md](../README.md) and [CONFIGURATION.md](CONFIGURATION.md)
2. **GitHub Issues**: [github.com/pulsepoint/pulsepoint/issues](https://github.com/pulsepoint/pulsepoint/issues)
3. **Community Forum**: [discuss.pulsepoint.io](https://discuss.pulsepoint.io)
4. **Email Support**: support@pulsepoint.io

### Reporting Issues

Include:
1. PulsePoint version: `pulsepoint --version`
2. Operating system and version
3. Steps to reproduce
4. Error messages (full text)
5. Relevant configuration (sanitized)
6. Debug logs if available

**Security Note**: Never share:
- OAuth tokens
- Client secrets
- API keys
- Personal credentials

---

## Recovery Procedures

### Complete Reset

If all else fails, perform a complete reset:

```bash
# 1. Backup important data
cp -r ~/.pulsepoint ~/.pulsepoint.backup

# 2. Remove PulsePoint data
rm -rf ~/.pulsepoint

# 3. Reinitialize
pulsepoint init

# 4. Re-authenticate
pulsepoint auth google

# 5. Reconfigure
pulsepoint config set sync.interval 5m
# ... other settings

# 6. Start fresh sync
pulsepoint sync /path --full
```

### Emergency Stop

Stop all PulsePoint operations:

```bash
# Stop all processes
pkill -f pulsepoint

# Remove lock files
rm -f ~/.pulsepoint/*.lock

# Clear queue
rm -f ~/.pulsepoint/queue/*
```

---

## Preventive Measures

### Regular Maintenance

```bash
# Weekly: Compact database
pulsepoint maintenance compact

# Monthly: Clean old logs
pulsepoint maintenance clean-logs --older-than 30d

# Quarterly: Full backup
pulsepoint backup --full --output backup.tar.gz
```

### Monitoring

Set up monitoring:

```bash
# Health check script
#!/bin/bash
if ! pulsepoint status --check; then
    echo "PulsePoint is not healthy"
    # Send alert
fi
```

### Best Practices

1. **Regular backups**: Enable automatic database backups
2. **Log rotation**: Configure appropriate log retention
3. **Monitor disk space**: Ensure adequate free space
4. **Update regularly**: Keep PulsePoint updated
5. **Test configuration**: Validate after changes

---

*For additional help, consult the [main documentation](../README.md) or contact support.*
