#!/bin/bash

# PulsePoint Google Drive Authentication Setup Script
# This script helps set up Google Drive authentication for PulsePoint

set -e

PULSEPOINT_DIR="$HOME/.pulsepoint"
CREDENTIALS_DIR="$PULSEPOINT_DIR/credentials"
TOKENS_DIR="$PULSEPOINT_DIR/tokens"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}PulsePoint Google Drive Authentication Setup${NC}"
echo "============================================="
echo

# Create directories if they don't exist
mkdir -p "$CREDENTIALS_DIR"
mkdir -p "$TOKENS_DIR"

# Check if credentials already exist
if [ -f "$CREDENTIALS_DIR/google_credentials.json" ]; then
    echo -e "${YELLOW}Google credentials file already exists at:${NC}"
    echo "$CREDENTIALS_DIR/google_credentials.json"
    echo
    read -p "Do you want to replace it? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Keeping existing credentials."
        exit 0
    fi
fi

echo -e "${YELLOW}Setting up Google Drive API credentials...${NC}"
echo
echo "You have two options:"
echo "1. Use environment variables (GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET)"
echo "2. Use a credentials JSON file from Google Cloud Console"
echo
read -p "Choose option (1 or 2): " option

if [ "$option" == "1" ]; then
    # Option 1: Environment variables
    echo
    echo "Please enter your Google OAuth2 credentials:"
    read -p "Client ID: " client_id
    read -s -p "Client Secret: " client_secret
    echo
    
    if [ -z "$client_id" ] || [ -z "$client_secret" ]; then
        echo -e "${RED}Error: Client ID and Client Secret are required${NC}"
        exit 1
    fi
    
    # Create credentials JSON file
    cat > "$CREDENTIALS_DIR/google_credentials.json" << EOF
{
  "installed": {
    "client_id": "$client_id",
    "client_secret": "$client_secret",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "redirect_uris": ["http://localhost:8080/callback", "urn:ietf:wg:oauth:2.0:oob"]
  }
}
EOF
    
    echo -e "${GREEN}Credentials file created successfully!${NC}"
    
elif [ "$option" == "2" ]; then
    # Option 2: Copy existing credentials file
    echo
    echo "Please provide the path to your Google credentials JSON file"
    echo "(Downloaded from Google Cloud Console)"
    read -p "Path to credentials file: " creds_path
    
    if [ ! -f "$creds_path" ]; then
        echo -e "${RED}Error: File not found: $creds_path${NC}"
        exit 1
    fi
    
    # Copy credentials file
    cp "$creds_path" "$CREDENTIALS_DIR/google_credentials.json"
    echo -e "${GREEN}Credentials file copied successfully!${NC}"
    
else
    echo -e "${RED}Invalid option${NC}"
    exit 1
fi

echo
echo -e "${GREEN}Setup complete!${NC}"
echo
echo "Next steps:"
echo "1. Run: pulsepoint auth google"
echo "2. Follow the browser prompt to authorize PulsePoint"
echo "3. Start syncing with: pulsepoint sync start"
echo
echo "Your credentials are stored at:"
echo "  $CREDENTIALS_DIR/google_credentials.json"
echo
echo -e "${YELLOW}Note: Keep your credentials file secure and never commit it to version control${NC}"
