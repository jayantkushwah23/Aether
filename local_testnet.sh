#!/usr/bin/env bash
set -euo pipefail

# ----------------------------------------------------------------------
# Local Single-Validator Testnet Launch Script
# Chain ID: sovereign-1
# Moniker:  my-test-chain
# Founder:  10,000,000,000 unative
# Feeshare: 2000 bps (20%) -> Founder Treasury
# ----------------------------------------------------------------------

CHAIN_ID="sovereign-1"
MONIKER="my-test-chain"
KEYRING_BACKEND="test"
FOUNDER_KEY="founder"
TENANT_KEY="tenant"
STAKE_DENOM="unative"
INITIAL_BALANCE="10000000000unative"
STAKE_AMOUNT="1000000000unative"
FOUNDER_BPS=2000

# Locate repo directory
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -d "$REPO_ROOT/cosmos-sdk-main" ]; then
    SRC_DIR="$REPO_ROOT/cosmos-sdk-main"
else
    SRC_DIR="$REPO_ROOT"
fi

cd "$SRC_DIR"
NODE_HOME="${HOME}/.simapp"
BUILD_DIR="build"
SIMD_BIN="./${BUILD_DIR}/simd"

echo "=========================================================="
echo " Starting Local Testnet Automated Build & Launch"
echo " Chain ID:  $CHAIN_ID"
echo " Moniker:   $MONIKER"
echo " Node Home: $NODE_HOME"
echo "=========================================================="

# 1. Clean any old test data: rm -rf ~/.simapp
echo "[1/7] Cleaning old test data (rm -rf $NODE_HOME)..."
rm -rf "$NODE_HOME"

# 2. Compile the latest project codebase binary: go build -o build/simd ./cmd/simd
echo "[2/7] Compiling project binary (go build -o build/simd ./cmd/simd)..."
mkdir -p "$BUILD_DIR"
if [ -d "./cmd/simd" ]; then
    go build -o "${BUILD_DIR}/simd" ./cmd/simd
else
    go build -o "${BUILD_DIR}/simd" ./simapp/simd
fi

if [ ! -f "$SIMD_BIN" ]; then
    echo "ERROR: Binary compilation failed; $SIMD_BIN not found!" >&2
    exit 1
fi
echo "Compiled binary successfully: $SIMD_BIN"

# 3. Initialize a fresh local single-validator genesis profiles state: ./build/simd init my-test-chain --chain-id sovereign-1
echo "[3/7] Initializing genesis state ($SIMD_BIN init $MONIKER --chain-id $CHAIN_ID)..."
"$SIMD_BIN" init "$MONIKER" --chain-id "$CHAIN_ID" --home "$NODE_HOME"

GENESIS_FILE="$NODE_HOME/config/genesis.json"
APP_TOML="$NODE_HOME/config/app.toml"
CONFIG_TOML="$NODE_HOME/config/config.toml"

# Update default denoms from 'stake' to 'unative' across genesis
if command -v sed >/dev/null 2>&1; then
    echo "Configuring network bond denom to '$STAKE_DENOM'..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s/\"stake\"/\"$STAKE_DENOM\"/g" "$GENESIS_FILE"
    else
        sed -i "s/\"stake\"/\"$STAKE_DENOM\"/g" "$GENESIS_FILE"
    fi
fi

# 4. Generate local testing key named 'founder' and key named 'tenant' using test keyring backend
echo "[4/7] Generating local keys ('$FOUNDER_KEY' and '$TENANT_KEY')..."
"$SIMD_BIN" keys add "$FOUNDER_KEY" --keyring-backend "$KEYRING_BACKEND" --home "$NODE_HOME"
"$SIMD_BIN" keys add "$TENANT_KEY" --keyring-backend "$KEYRING_BACKEND" --home "$NODE_HOME"

FOUNDER_ADDR=$("$SIMD_BIN" keys show "$FOUNDER_KEY" -a --keyring-backend "$KEYRING_BACKEND" --home "$NODE_HOME")
TENANT_ADDR=$("$SIMD_BIN" keys show "$TENANT_KEY" -a --keyring-backend "$KEYRING_BACKEND" --home "$NODE_HOME")

echo "Founder Address: $FOUNDER_ADDR"
echo "Tenant Address:  $TENANT_ADDR"

# 5. Inject 'founder' address into genesis accounts allocation block with 10,000,000,000 unative
echo "[5/7] Injecting genesis account ($INITIAL_BALANCE)..."
"$SIMD_BIN" genesis add-genesis-account "$FOUNDER_ADDR" "$INITIAL_BALANCE" \
    --keyring-backend "$KEYRING_BACKEND" \
    --home "$NODE_HOME"

# Also fund tenant with 100,000,000 unative for local testing
"$SIMD_BIN" genesis add-genesis-account "$TENANT_ADDR" "100000000unative" \
    --keyring-backend "$KEYRING_BACKEND" \
    --home "$NODE_HOME"

# Generate gentx for the single validator to generate blocks
echo "Generating gentx for founder validator ($STAKE_AMOUNT)..."
"$SIMD_BIN" genesis gentx "$FOUNDER_KEY" "$STAKE_AMOUNT" \
    --chain-id "$CHAIN_ID" \
    --keyring-backend "$KEYRING_BACKEND" \
    --home "$NODE_HOME"

echo "Collecting genesis transactions..."
"$SIMD_BIN" genesis collect-gentxs --home "$NODE_HOME"

# 6. Modify generated genesis.json to configure feeshare module params: founder_bps = 2000 (20%), treasury_address = founder address
echo "[6/7] Configuring feeshare genesis parameters (founder_bps: $FOUNDER_BPS, treasury: $FOUNDER_ADDR)..."
if command -v python3 >/dev/null 2>&1; then
    python3 -c "
import json
with open('$GENESIS_FILE', 'r') as f:
    data = json.load(f)

app_state = data.setdefault('app_state', {})
app_state['feeshare'] = {
    'params': {
        'founder_bps': $FOUNDER_BPS,
        'treasury_address': '$FOUNDER_ADDR'
    }
}

with open('$GENESIS_FILE', 'w') as f:
    json.dump(data, f, indent=2)
"
elif command -v jq >/dev/null 2>&1; then
    jq --arg addr "$FOUNDER_ADDR" --argjson bps "$FOUNDER_BPS" \
        '.app_state.feeshare = {"params": {"founder_bps": $bps, "treasury_address": $addr}}' \
        "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"
else
    sed -i "s/\"treasury_address\": *\"[^\"]*\"/\"treasury_address\": \"$FOUNDER_ADDR\"/g" "$GENESIS_FILE" || true
    sed -i "s/\"founder_bps\": *[0-9]*/\"founder_bps\": $FOUNDER_BPS/g" "$GENESIS_FILE" || true
fi

echo "Validating genesis state..."
"$SIMD_BIN" genesis validate-genesis --home "$NODE_HOME"

# Configure gas prices and commit intervals for fast local dev testing
if command -v sed >/dev/null 2>&1; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' 's/enable = false/enable = true/g' "$APP_TOML"
        sed -i '' 's/swagger = false/swagger = true/g' "$APP_TOML"
        sed -i '' "s/minimum-gas-prices = \"\"/minimum-gas-prices = \"0$STAKE_DENOM\"/g" "$APP_TOML"
        sed -i '' 's/timeout_commit = "5s"/timeout_commit = "1s"/g' "$CONFIG_TOML"
    else
        sed -i 's/enable = false/enable = true/g' "$APP_TOML"
        sed -i 's/swagger = false/swagger = true/g' "$APP_TOML"
        sed -i "s/minimum-gas-prices = \"\"/minimum-gas-prices = \"0$STAKE_DENOM\"/g" "$APP_TOML"
        sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' "$CONFIG_TOML"
    fi
fi

# 7. Launch network validator execution loop: ./build/simd start
echo "[7/7] Launching network validator ($SIMD_BIN start)..."
echo "=========================================================="
echo " Chain ID:          $CHAIN_ID"
echo " Founder Address:   $FOUNDER_ADDR"
echo " Tenant Address:    $TENANT_ADDR"
echo " Treasury Routing:  20% gas share -> $FOUNDER_ADDR"
echo " RPC Server:        http://localhost:26657"
echo " REST/Swagger:      http://localhost:1317"
echo "=========================================================="

exec "$SIMD_BIN" start --home "$NODE_HOME"
