#!/usr/bin/env bash
# ==============================================================================
# Aether Network - 5-Minute Local Node Quickstart Script (Linux / macOS)
# ==============================================================================

set -euo pipefail

CHAIN_ID="aether-1"
MONIKER="local-validator"
DENOM="ucurts"
BUILD_DIR="./build"
BINARY="${BUILD_DIR}/simd"

echo "=========================================================="
echo "  AETHER NETWORK: 5-Minute Local Testnet Initialization"
echo "  Chain ID:    ${CHAIN_ID}"
echo "  Base Denom:  ${DENOM} (Curtis atomic unit, 6 decimals)"
echo "=========================================================="

# 1. Compile Binary
echo "[1/5] Compiling native 'simd' blockchain binary..."
mkdir -p "$BUILD_DIR"
go build -o "$BINARY" ./cmd/simd
echo "      Binary compiled successfully at ${BINARY}"

# 2. Reset Home Directory
SIMAPP_HOME="${HOME}/.simapp"
echo "[2/5] Initializing chain configuration at ${SIMAPP_HOME}..."
rm -rf "$SIMAPP_HOME"
"$BINARY" init "$MONIKER" --chain-id "$CHAIN_ID" --default-denom "$DENOM"

# 3. Create Validator Key
echo "[3/5] Generating local validator test key..."
"$BINARY" keys add validator --keyring-backend test

# 4. Allocate Genesis Balances & Generate GenTx
echo "[4/5] Injecting genesis balances and forging genesis validator transaction..."
"$BINARY" genesis add-genesis-account validator 1000000000000"$DENOM" --keyring-backend test
"$BINARY" genesis gentx validator 100000000000"$DENOM" --chain-id "$CHAIN_ID" --keyring-backend test
"$BINARY" genesis collect-gentxs

# 5. Configure REST API and CORS
if [ -f "${SIMAPP_HOME}/config/app.toml" ]; then
    sed -i.bak 's/enable = false/enable = true/g' "${SIMAPP_HOME}/config/app.toml" 2>/dev/null || true
    sed -i.bak 's/enabled-unsafe-cors = false/enabled-unsafe-cors = true/g' "${SIMAPP_HOME}/config/app.toml" 2>/dev/null || true
fi
if [ -f "${SIMAPP_HOME}/config/config.toml" ]; then
    sed -i.bak 's/cors_allowed_origins = \[\]/cors_allowed_origins = ["*"]/g' "${SIMAPP_HOME}/config/config.toml" 2>/dev/null || true
fi

echo "=========================================================="
echo "  AETHER NODE READY TO START!"
echo "  CometBFT RPC:   http://localhost:26657"
echo "  Cosmos REST:    http://localhost:1317"
echo "  Starting block mining now... (Press Ctrl+C to stop)"
echo "=========================================================="

"$BINARY" start
