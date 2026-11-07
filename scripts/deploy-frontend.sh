#!/usr/bin/env bash
# ==============================================================================
# Aether Network - Frontend Portal Automated Deployment Pipeline (Linux / macOS)
# Target: AWS EC2 Public Production Web Server (34.207.176.57)
# ==============================================================================

set -euo pipefail

EC2_HOST="${1:-34.207.176.57}"
EC2_USER="${2:-ubuntu}"
KEY_PATH="${3:-./aether-key.pem}"

echo "=================================================="
echo "  Aether Portal Automated Cloud Deployment Pipeline"
echo "  Target Host: ${EC2_HOST}"
echo "  Target User: ${EC2_USER}"
echo "=================================================="

# 1. Locate SSH Key
if [ ! -f "$KEY_PATH" ]; then
    if [ -f "../aether-key.pem" ]; then
        KEY_PATH="../aether-key.pem"
    elif [ -f "../../aether-key.pem" ]; then
        KEY_PATH="../../aether-key.pem"
    else
        echo "[ERROR] aether-key.pem not found. Please provide path as 3rd argument."
        exit 1
    fi
fi
chmod 400 "$KEY_PATH" 2>/dev/null || true

# 2. Locate Frontend Directory
FRONTEND_DIR="./frontend"
if [ ! -d "$FRONTEND_DIR" ]; then
    if [ -d "../frontend" ]; then
        FRONTEND_DIR="../frontend"
    else
        echo "[ERROR] Frontend directory not found."
        exit 1
    fi
fi

echo "[1/4] Validating frontend source files in ${FRONTEND_DIR}..."
test -f "${FRONTEND_DIR}/index.html" || { echo "[ERROR] Missing index.html"; exit 1; }
test -f "${FRONTEND_DIR}/styles.css" || { echo "[ERROR] Missing styles.css"; exit 1; }
test -f "${FRONTEND_DIR}/app.js"     || { echo "[ERROR] Missing app.js"; exit 1; }
echo "      Source files verified successfully."

echo "[2/4] Uploading frontend bundle to production server..."
scp -i "$KEY_PATH" -o StrictHostKeyChecking=no -r "${FRONTEND_DIR}/"* "${EC2_USER}@${EC2_HOST}:/var/www/html/"

echo "[3/4] Updating production web server permissions & reloading Nginx..."
ssh -i "$KEY_PATH" -o StrictHostKeyChecking=no "${EC2_USER}@${EC2_HOST}" "sudo chown -R www-data:www-data /var/www/html && sudo systemctl reload nginx"

echo "[4/4] Verifying production HTTP health status..."
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://${EC2_HOST}/" || echo "failed")

if [ "$HTTP_STATUS" = "200" ]; then
    echo "=================================================="
    echo "  DEPLOYMENT SUCCESSFUL!"
    echo "  Public Web Portal URL: http://${EC2_HOST}/"
    echo "  RPC API Endpoint:     http://${EC2_HOST}:26657"
    echo "  REST API Endpoint:    http://${EC2_HOST}:1317"
    echo "=================================================="
else
    echo "[WARNING] Server responded with HTTP status: ${HTTP_STATUS}"
fi
