#!/bin/bash
set -e
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then GOARCH=amd64; fi
if [ "$ARCH" = "aarch64" ]; then GOARCH=arm64; fi
HUB_IP=$1
if [ -z "$HUB_IP" ]; then
  echo "Usage: $0 <hub-public-ip-or-domain>"
  exit 1
fi
echo "Detected architecture: $ARCH -> $GOARCH"
apt update && apt install -y wireguard qrencode curl
curl -L https://github.com/suyogdahal/go-sdwan/releases/download/latest/hub-linux-$GOARCH -o /usr/local/bin/sdwan-hub
curl -L https://github.com/suyogdahal/go-sdwan/releases/download/latest/cpe-linux-$GOARCH -o /usr/local/bin/sdwan-cpe
chmod +x /usr/local/bin/sdwan-hub /usr/local/bin/sdwan-cpe
read -p "Install as [h]ub or [c]pe? " role
if [[ "$role" == "h" || "$role" == "H" ]]; then
cat > /etc/systemd/system/sdwan-hub.service <<EOF
[Unit]
Description=Go SD-WAN Hub
After=network.target
[Service]
ExecStart=/usr/local/bin/sdwan-hub
Restart=always
RestartSec=5
[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload && systemctl enable --now sdwan-hub
  echo "Hub started! Dashboard -> http://$(hostname -I | awk '{print $1}'):8080/dashboard"
else
  read -p "Enter enrollment token: " token
cat > /etc/systemd/system/sdwan-cpe.service <<EOF
[Unit]
Description=Go SD-WAN CPE
After=network.target
[Service]
ExecStart=/usr/local/bin/sdwan-cpe --hub http://$HUB_IP:8080 --token $token
Restart=always
RestartSec=5
[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload && systemctl enable --now sdwan-cpe
  echo "CPE enrolled and connected!"
fi
