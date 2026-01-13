package main

import "fmt"

func GenerateInstallerScript(serverURL, uuid string) string {
	return fmt.Sprintf(`#!/bin/bash
set -e

ARCH=$(uname -m)

INSTALL_DIR=/opt/dronnayak
BIN_URL="%s/bin/${ARCH}"
CONFIG_URL="%s/device/%s/config.json"

echo "Installing dronnayak for $ARCH"

sudo mkdir -p $INSTALL_DIR
cd $INSTALL_DIR

sudo rm -f dronnayak

echo "Downloading binary..."
wget "$BIN_URL" -O dronnayak
chmod +x dronnayak

echo "Fetching config..."
wget "$CONFIG_URL" -O config.json

echo "Installing systemd service..."
cat <<EOF | sudo tee /etc/systemd/system/dronnayak.service
[Unit]
Description=Dronnayak Device Manager
After=network.target

[Service]
Type=simple
Restart=always
WorkingDirectory=/opt/dronnayak
ExecStart=/opt/dronnayak/dronnayak

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable dronnayak
sudo systemctl start dronnayak

echo "Installation complete"
`, serverURL, serverURL, uuid)
}
