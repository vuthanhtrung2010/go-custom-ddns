#!/bin/bash

# Exit on error
set -e

REPO="vuthanhtrung2010/go-custom-ddns"
BIN_DIR="/usr/local/bin"
BIN_NAME="go-custom-ddns"
ENV_FILE="/etc/go-custom-ddns.env"

echo "Checking for root privileges..."
if [ "$EUID" -ne 0 ]; then
  echo "Please run this script as root (sudo ./install.sh)"
  exit 1
fi

echo "Fetching latest release from GitHub..."
# Note: Ensure your GitHub releases have a generic binary name or adjust the grep below.
LATEST_URL=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep "browser_download_url.*linux-amd64" | cut -d '"' -f 4)

if [ -z "$LATEST_URL" ]; then
    echo "Could not find a valid release URL. Have you published a linux-amd64 binary yet?"
    exit 1
fi

echo "Downloading $LATEST_URL..."
curl -L -o $BIN_DIR/$BIN_NAME $LATEST_URL
chmod +x $BIN_DIR/$BIN_NAME

echo "Starting interactive setup..."
$BIN_DIR/$BIN_NAME -setup

echo "Creating systemd service..."
cat <<EOF > /etc/systemd/system/go-custom-ddns.service
[Unit]
Description=Go Custom Cloudflare DDNS Updater
After=network-online.target

[Service]
Type=oneshot
EnvironmentFile=$ENV_FILE
ExecStart=$BIN_DIR/$BIN_NAME
# Working directory so old-ip.txt saves in a predictable spot
WorkingDirectory=/etc/
EOF

echo "Creating systemd timer (Runs every 5 minutes)..."
cat <<EOF > /etc/systemd/system/go-custom-ddns.timer
[Unit]
Description=Run Go Custom DDNS every 5 minutes

[Timer]
OnBootSec=1min
OnUnitActiveSec=5min

[Install]
WantedBy=timers.target
EOF

echo "Enabling and starting systemd timer..."
systemctl daemon-reload
systemctl enable --now go-custom-ddns.timer

echo "Installation complete! Your DDNS is now running automatically."
echo "You can check the logs anytime with: journalctl -u go-custom-ddns.service"