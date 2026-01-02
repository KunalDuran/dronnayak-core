package main

import "fmt"

func GetInstallerScript(serverPath, uuid string) string {
	text := `# check device architecture
ARCH=$(uname -m)

mkdir -p /opt/dronnayak
cd /opt/dronnayak

sudo rm -rf dronnayak

# download binary
wget "%s/bin/${ARCH}" -O dronnayak

# make binary executable
chmod +x dronnayak

# create config
echo "{
	\"uuid\": \"%s\",
	\"server_path\": \"%s\",
	\"tunnel_ports\": [\"5760\", \"22\"]
}" > config.json

# setup as service
echo "[Unit]
    Description=Device Manager
    After=multi-user.target

[Service]
    Type=simple
    Restart=always
    ExecStart=/opt/dronnayak/dronnayak

[Install]
    WantedBy=multi-user.target" > dronnayak.service

    
sudo cp dronnayak.service /etc/systemd/system/dronnayak.service
sudo systemctl daemon-reload
sudo systemctl enable dronnayak
sudo systemctl start dronnayak`

	return fmt.Sprintf(text, serverPath, uuid, serverPath)
}
