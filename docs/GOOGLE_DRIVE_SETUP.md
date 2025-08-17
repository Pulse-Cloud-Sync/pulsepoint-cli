# Google Drive Setup Guide for PulsePoint

## Prerequisites

Before you can use PulsePoint with Google Drive, you need to:

1. Have a Google account
2. Set up a Google Cloud Project
3. Enable the Google Drive API
4. Create OAuth2 credentials

## Step-by-Step Setup Guide

### 1. Create a Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Click on the project dropdown at the top
3. Click "New Project"
4. Enter a project name (e.g., "PulsePoint Sync")
5. Click "Create"

### 2. Enable Google Drive API

1. In the Google Cloud Console, ensure your project is selected
2. Go to "APIs & Services" > "Library"
3. Search for "Google Drive API"
4. Click on "Google Drive API"
5. Click "Enable"

### 3. Create OAuth2 Credentials

1. Go to "APIs & Services" > "Credentials"
2. Click "Create Credentials" > "OAuth client ID"
3. If prompted, configure the OAuth consent screen:
   - Choose "External" user type (unless you have a Google Workspace account)
   - Fill in the required fields:
     - App name: "PulsePoint"
     - User support email: Your email
     - Developer contact: Your email
   - Add scopes: `https://www.googleapis.com/auth/drive.file`
   - Add test users (your email)
4. For Application type, select "Desktop app"
5. Name it "PulsePoint Desktop Client"
6. Click "Create"
7. Download the credentials JSON file

### 4. Configure PulsePoint

#### Option A: Using the Setup Script

```bash
# Run the setup script
./scripts/setup-google-auth.sh

# Follow the prompts to either:
# - Enter your Client ID and Secret manually, or
# - Provide the path to your downloaded credentials file
```

#### Option B: Manual Setup

1. Create the credentials directory:
```bash
mkdir -p ~/.pulsepoint/credentials
```

2. Copy your credentials file:
```bash
cp ~/Downloads/client_secret_*.json ~/.pulsepoint/credentials/google_credentials.json
```

#### Option C: Using Environment Variables

Set these environment variables in your shell profile:

```bash
export GOOGLE_CLIENT_ID="your-client-id"
export GOOGLE_CLIENT_SECRET="your-client-secret"
```

### 5. Authenticate PulsePoint

Once credentials are configured, authenticate PulsePoint:

```bash
# Start the authentication process
pulsepoint auth google

# This will:
# 1. Open your browser for authorization
# 2. Ask you to grant permissions
# 3. Save the access token locally
```

### 6. Verify Authentication

Check that authentication was successful:

```bash
pulsepoint auth google --status
```

You should see:
```
✅ Authenticated
   Provider: Google Drive
   Account: your-email@gmail.com
```

## Configuration Options

You can customize Google Drive behavior in your PulsePoint config:

```yaml
# ~/.pulsepoint/config.yaml
providers:
  google:
    root_folder_id: ""  # Optional: specific folder ID to use as root
    simple_upload_threshold: 5242880  # 5MB - files smaller use simple upload
    resumable_upload_threshold: 104857600  # 100MB - files larger use resumable
    chunk_size: 8388608  # 8MB - chunk size for large uploads
    max_retries: 3
    rate_limit: 10  # requests per second
```

## Troubleshooting

### Common Issues

#### 1. "Error 400: redirect_uri_mismatch"

This means the redirect URI doesn't match. Ensure your OAuth client is configured as a "Desktop app" type.

#### 2. "Error 403: access_denied"

You may need to:
- Add your email as a test user in the OAuth consent screen
- Ensure the Google Drive API is enabled
- Check that the required scopes are configured

#### 3. "Token expired"

PulsePoint should automatically refresh tokens, but if issues persist:

```bash
# Revoke and re-authenticate
pulsepoint auth google --revoke
pulsepoint auth google
```

#### 4. "Quota exceeded"

Google Drive API has usage limits:
- 1,000,000,000 requests per day
- 1,000 requests per 100 seconds per user

If you hit limits, you may need to:
- Wait for quota reset
- Reduce sync frequency
- Optimize file operations

### Getting Your Root Folder ID

If you want to sync to a specific folder instead of root:

1. Go to Google Drive in your browser
2. Navigate to the folder
3. The URL will be: `https://drive.google.com/drive/folders/FOLDER_ID`
4. Copy the FOLDER_ID and add to config:

```yaml
providers:
  google:
    root_folder_id: "your-folder-id-here"
```

## Security Best Practices

1. **Never commit credentials to version control**
   - Add `google_credentials.json` to `.gitignore`
   - Use environment variables in CI/CD

2. **Protect your token file**
   - Token file contains access to your Drive
   - Located at `~/.pulsepoint/tokens/google_token.json`
   - Set appropriate file permissions: `chmod 600`

3. **Use application-specific folders**
   - Consider using a dedicated folder for PulsePoint syncs
   - Reduces risk of accidental file operations

4. **Regular token rotation**
   - Periodically revoke and re-authenticate
   - Especially after any security concerns

## API Limitations

Be aware of Google Drive API limitations:

- **File size**: Maximum 5TB per file
- **File name**: Maximum 255 characters
- **Path depth**: No official limit, but deep nesting can cause issues
- **Rate limits**: 1000 requests per 100 seconds
- **Daily quota**: 1 billion requests (rarely hit)

## Need Help?

If you encounter issues:

1. Check the logs: `pulsepoint logs --tail 50`
2. Run in verbose mode: `pulsepoint --verbose sync start`
3. Check authentication: `pulsepoint auth google --status`
4. Review this guide and ensure all steps were followed

For additional support, please open an issue on the GitHub repository.
