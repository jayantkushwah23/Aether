/**
 * AETHER NETWORK - COMMERCIAL WEB PORTAL & TOKEN FACTORY
 * Live Layer-1 Node: 34.207.176.57
 * Chain ID: aether-1
 */

// Core Network Constants
const AETHER_NETWORK = {
  chainId: "aether-1",
  chainName: "Aether Mainnet",
  rpc: "http://34.207.176.57:26657",
  rest: "http://34.207.176.57:1317",
  baseDenom: "ucurts",
  displayDenom: "curtis",
  decimals: 6,
  bech32Prefix: "aether"
};

// Keplr Chain Profile Definition
const AETHER_KEPLR_CONFIG = {
  chainId: AETHER_NETWORK.chainId,
  chainName: AETHER_NETWORK.chainName,
  rpc: AETHER_NETWORK.rpc,
  rest: AETHER_NETWORK.rest,
  bip44: {
    coinType: 118
  },
  bech32Config: {
    bech32PrefixAccAddr: "aether",
    bech32PrefixAccPub: "aetherpub",
    bech32PrefixValAddr: "aethervaloper",
    bech32PrefixValPub: "aethervaloperpub",
    bech32PrefixConsAddr: "aethervalcons",
    bech32PrefixConsPub: "aethervalconspub"
  },
  currencies: [
    {
      coinDenom: "CURTIS",
      coinMinimalDenom: "ucurts",
      coinDecimals: 6,
      coinGeckoId: "aether"
    }
  ],
  feeCurrencies: [
    {
      coinDenom: "CURTIS",
      coinMinimalDenom: "ucurts",
      coinDecimals: 6,
      coinGeckoId: "aether",
      gasPriceStep: {
        low: 0.01,
        average: 0.025,
        high: 0.04
      }
    }
  ],
  stakeCurrency: {
    coinDenom: "CURTIS",
    coinMinimalDenom: "ucurts",
    coinDecimals: 6,
    coinGeckoId: "aether"
  },
  features: ["cosmwasm", "ibc-transfer", "ibc-go"]
};

// Global State
let currentAccount = {
  address: null,
  pubKey: null,
  balanceUcurts: "0",
  balanceCurtis: "0.00"
};

let lastDeployedTxHash = null;
let lastDeployedDenom = null;

// ============================================================================
// 1. Keplr Wallet Integration Module
// ============================================================================

async function connectWallet() {
  const btn = document.getElementById("connectKeplrBtn");
  if (btn) btn.disabled = true;

  try {
    if (!window.keplr) {
      appendTerminalLog("Keplr extension not detected in browser context.", "error");
      showToast("Keplr Wallet extension not found. Please install Keplr from keplr.app.", "error");
      window.open("https://www.keplr.app/download", "_blank");
      if (btn) btn.disabled = false;
      return;
    }

    appendTerminalLog(`Registering chain: ${AETHER_NETWORK.chainId} via experimentalSuggestChain...`, "info");
    await window.keplr.experimentalSuggestChain(AETHER_KEPLR_CONFIG);
    appendTerminalLog(`Chain registration acknowledged by Keplr.`, "success");

    appendTerminalLog(`Requesting permission to access ${AETHER_NETWORK.chainId}...`, "info");
    await window.keplr.enable(AETHER_NETWORK.chainId);

    const offlineSigner = window.keplr.getOfflineSigner(AETHER_NETWORK.chainId);
    const accounts = await offlineSigner.getAccounts();

    if (!accounts || accounts.length === 0) {
      throw new Error("No accounts available in connected Keplr instance.");
    }

    currentAccount.address = accounts[0].address;
    currentAccount.pubKey = accounts[0].pubkey;

    appendTerminalLog(`Authenticated wallet: ${currentAccount.address}`, "success");
    showToast(`Connected: ${currentAccount.address.slice(0, 10)}...${currentAccount.address.slice(-6)}`, "success");

    renderWalletBadge(currentAccount.address);
    await refreshAccountBalance();
    updateDenomPreview();

  } catch (err) {
    appendTerminalLog(`Wallet authentication error: ${err.message}`, "error");
    showToast(`Connection failed: ${err.message}`, "error");
    console.error("Keplr connection error:", err);
  } finally {
    if (btn) btn.disabled = false;
  }
}

function renderWalletBadge(address) {
  const container = document.getElementById("walletControl");
  if (!container) return;

  const shortAddr = `${address.slice(0, 10)}...${address.slice(-6)}`;

  container.innerHTML = `
    <div class="wallet-badge">
      <div class="wallet-badge-avatar">Æ</div>
      <div class="wallet-badge-meta">
        <span class="wallet-badge-addr mono" title="${address}">${shortAddr}</span>
        <span class="wallet-badge-bal mono" id="walletBalanceDisplay">Fetching balance...</span>
      </div>
      <button class="btn-copy-sm" onclick="copyAddress('${address}')" title="Copy Address">Copy</button>
    </div>
  `;
}

async function refreshAccountBalance() {
  if (!currentAccount.address) return;

  try {
    const res = await fetch(`${AETHER_NETWORK.rest}/cosmos/bank/v1beta1/balances/${currentAccount.address}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();

    const ucurtsCoin = (data.balances || []).find(c => c.denom === AETHER_NETWORK.baseDenom);
    const rawAmount = ucurtsCoin ? ucurtsCoin.amount : "0";
    currentAccount.balanceUcurts = rawAmount;

    const curtisDec = (parseInt(rawAmount, 10) / 1000000).toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 6
    });

    currentAccount.balanceCurtis = curtisDec;

    const balElem = document.getElementById("walletBalanceDisplay");
    if (balElem) {
      balElem.innerText = `${curtisDec} CURTIS`;
    }
  } catch (err) {
    console.warn("Balance fetch error:", err);
  }
}

function copyAddress(addr) {
  navigator.clipboard.writeText(addr).then(() => {
    showToast("Address copied to clipboard!", "info");
  });
}

// ============================================================================
// 2. Multi-Tenant Token Factory Panel Helpers
// ============================================================================

function autoDeriveSubdenom() {
  const symbol = document.getElementById("tokenSymbol").value.trim().toLowerCase();
  const subdenomInput = document.getElementById("subdenomSlug");

  if (symbol && (!subdenomInput.dataset.manualEdited || subdenomInput.value === "")) {
    subdenomInput.value = "u" + symbol.replace(/[^a-z0-9]/g, "");
  }

  updateDenomPreview();
}

document.addEventListener("DOMContentLoaded", () => {
  const subdenomInput = document.getElementById("subdenomSlug");
  if (subdenomInput) {
    subdenomInput.addEventListener("input", () => {
      subdenomInput.dataset.manualEdited = "true";
      updateDenomPreview();
    });
  }
});

function updateDenomPreview() {
  const subdenom = document.getElementById("subdenomSlug").value.trim() || "{subdenom}";
  const preview = document.getElementById("denomPreview");
  const creator = currentAccount.address || "aether1...";

  const fullDenom = `factory/${creator}/${subdenom}`;
  if (preview) {
    preview.innerText = fullDenom;
  }
}

function copyDenomPath() {
  const preview = document.getElementById("denomPreview");
  if (preview) {
    navigator.clipboard.writeText(preview.innerText).then(() => {
      showToast("Canonical denom copied!", "info");
    });
  }
}

function handleSupplyModeChange() {
  const mode = document.getElementById("supplyMode").value;
  const collateralRow = document.getElementById("collateralRow");
  const maxSupplyGroup = document.getElementById("maxSupply");
  const maxSupplyReq = document.getElementById("maxSupplyReq");

  if (mode === "3") { // Collateralized
    collateralRow.style.display = "grid";
    maxSupplyGroup.required = true;
    if (maxSupplyReq) maxSupplyReq.style.display = "inline";
  } else if (mode === "2") { // Uncapped
    collateralRow.style.display = "none";
    maxSupplyGroup.required = false;
    if (maxSupplyReq) maxSupplyReq.style.display = "none";
  } else { // Capped
    collateralRow.style.display = "none";
    maxSupplyGroup.required = true;
    if (maxSupplyReq) maxSupplyReq.style.display = "inline";
  }
}

function toggleAdvancedOptions() {
  const body = document.getElementById("advancedBody");
  const icon = document.getElementById("advancedIcon");
  if (body.style.display === "none" || body.style.display === "") {
    body.style.display = "block";
    icon.classList.add("open");
  } else {
    body.style.display = "none";
    icon.classList.remove("open");
  }
}

function updateFeeDisplay() {
  const bps = document.getElementById("transferFeeBps").value;
  const pct = (bps / 100).toFixed(2);
  document.getElementById("transferFeeDisplay").innerText = `${bps} bps (${pct}%)`;
}

function setFeeCollectorToCreator() {
  if (!currentAccount.address) {
    showToast("Please connect your wallet first.", "error");
    return;
  }
  document.getElementById("feeCollector").value = currentAccount.address;
  showToast("Fee collector set to current account address.", "info");
}

// ============================================================================
// 3. Live Transaction Processing Layer (MsgCreateDenom)
// ============================================================================

async function handleTokenDeployment(event) {
  event.preventDefault();

  if (!currentAccount.address) {
    showToast("Wallet required: Connect Keplr before deploying.", "error");
    await connectWallet();
    return;
  }

  const name = document.getElementById("tokenName").value.trim();
  const symbol = document.getElementById("tokenSymbol").value.trim().toUpperCase();
  const subdenom = document.getElementById("subdenomSlug").value.trim().toLowerCase();
  const initialSupply = document.getElementById("initialSupply").value.trim();
  let maxSupply = document.getElementById("maxSupply").value.trim();
  const supplyMode = parseInt(document.getElementById("supplyMode").value, 10);
  const transferFeeBps = parseInt(document.getElementById("transferFeeBps").value, 10);
  const feeCollector = document.getElementById("feeCollector").value.trim() || currentAccount.address;
  const canBurn = document.getElementById("canBurnCheck").checked;
  const collateralDenom = document.getElementById("collateralDenom").value.trim() || "ucurts";
  const collateralRatio = document.getElementById("collateralRatio").value.trim() || "1.0";

  if (supplyMode === 2 && (!maxSupply || maxSupply === "0")) {
    maxSupply = "0"; // Uncapped mode uses zero placeholder
  }

  const deployBtn = document.getElementById("deployBtn");
  deployBtn.disabled = true;
  deployBtn.innerHTML = `
    <svg class="spinner" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
      <circle cx="12" cy="12" r="10" stroke-opacity="0.25"></circle>
      <path d="M12 2a10 10 0 0 1 10 10" stroke-linecap="round"></path>
    </svg>
    Signing Transaction with Keplr...
  `;

  appendTerminalLog(`========================================`, "info");
  appendTerminalLog(`DEPLOYMENT INITIALIZED: ${name} [${symbol}]`, "info");
  appendTerminalLog(`Target Denom: factory/${currentAccount.address}/${subdenom}`, "cyan");
  appendTerminalLog(`Supply Mode: ${supplyMode === 1 ? 'CAPPED' : supplyMode === 2 ? 'UNCAPPED' : 'COLLATERALIZED'}`, "info");
  appendTerminalLog(`Transfer Fee: ${transferFeeBps} bps | Fee Collector: ${feeCollector}`, "info");

  try {
    // 1. Query Sequence & Account Number from REST endpoint
    appendTerminalLog(`Querying sequence from auth module at ${AETHER_NETWORK.rest}...`, "info");
    const authRes = await fetch(`${AETHER_NETWORK.rest}/cosmos/auth/v1beta1/accounts/${currentAccount.address}`);
    if (!authRes.ok) {
      throw new Error(`Failed to retrieve account state: HTTP ${authRes.status}`);
    }
    const authData = await authRes.json();

    let accountNumber = "0";
    let sequence = "0";

    if (authData.account) {
      accountNumber = authData.account.account_number || "0";
      sequence = authData.account.sequence || "0";
    }

    appendTerminalLog(`Account State: #account=${accountNumber}, seq=${sequence}`, "info");

    // 2. Build the MsgCreateDenom transaction message
    const msgCreateDenom = {
      type: "cosmos.currencies.v1.MsgCreateDenom",
      value: {
        creator: currentAccount.address,
        subdenom: subdenom,
        symbol: symbol,
        name: name,
        decimals: 6,
        supply_mode: supplyMode,
        max_supply: maxSupply,
        transfer_fee_bps: transferFeeBps,
        can_burn: canBurn,
        collateral_denom: collateralDenom,
        collateral_ratio: collateralRatio,
        fee_collector_address: feeCollector
      }
    };

    // 3. Construct the Amino SignDoc for Keplr
    const signDoc = {
      chain_id: AETHER_NETWORK.chainId,
      account_number: accountNumber.toString(),
      sequence: sequence.toString(),
      fee: {
        amount: [{ denom: AETHER_NETWORK.baseDenom, amount: "5000" }],
        gas: "250000"
      },
      msgs: [msgCreateDenom],
      memo: `Aether Token Factory: Deploy ${symbol}`
    };

    appendTerminalLog("Prompting Keplr cryptographic signature modal...", "cyan");
    const signResponse = await window.keplr.signAmino(
      AETHER_NETWORK.chainId,
      currentAccount.address,
      signDoc
    );

    appendTerminalLog("Cryptographic signature approved by signer.", "success");

    // 4. Construct the Broadcast Envelope
    const broadcastEnvelope = {
      tx: {
        msg: signResponse.signed.msgs,
        fee: signResponse.signed.fee,
        signatures: [
          {
            pub_key: signResponse.signature.pub_key,
            signature: signResponse.signature.signature
          }
        ],
        memo: signResponse.signed.memo
      },
      mode: "sync"
    };

    appendTerminalLog(`Broadcasting transaction to CometBFT RPC: ${AETHER_NETWORK.rpc}...`, "info");

    const rpcPayload = {
      jsonrpc: "2.0",
      id: Date.now(),
      method: "broadcast_tx_sync",
      params: {
        tx: btoa(JSON.stringify(broadcastEnvelope.tx))
      }
    };

    const broadcastRes = await fetch(AETHER_NETWORK.rpc, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(rpcPayload)
    });

    const broadcastData = await broadcastRes.json();

    if (broadcastData.error) {
      throw new Error(`RPC Broadcast Error: ${broadcastData.error.message || JSON.stringify(broadcastData.error)}`);
    }

    const txResult = broadcastData.result || {};
    const txHash = txResult.hash || "TX_SUBMITTED";

    lastDeployedTxHash = txHash;
    lastDeployedDenom = `factory/${currentAccount.address}/${subdenom}`;

    appendTerminalLog(`Transaction submitted successfully!`, "success");
    appendTerminalLog(`CometBFT TX Hash: ${txHash}`, "green");
    appendTerminalLog(`Waiting for block finalization...`, "info");

    // 5. Present Success Alert
    showSuccessAlert(txHash, lastDeployedDenom);
    showToast(`Token ${symbol} successfully deployed!`, "success");

    // 6. Refresh balance in background
    setTimeout(refreshAccountBalance, 2500);

  } catch (err) {
    appendTerminalLog(`Transaction Execution Failed: ${err.message}`, "rose");
    showToast(`Deployment error: ${err.message}`, "error");
    console.error("Token deployment error:", err);
  } finally {
    deployBtn.disabled = false;
    deployBtn.innerHTML = `
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path>
      </svg>
      Deploy Sovereign Token
    `;
  }
}

function showSuccessAlert(hash, denom) {
  const box = document.getElementById("txSuccessAlert");
  const hashElem = document.getElementById("successTxHash");
  const denomElem = document.getElementById("successDenom");

  if (hashElem) hashElem.innerText = hash;
  if (denomElem) denomElem.innerText = denom;
  if (box) box.style.display = "flex";
}

function copyTxHash() {
  if (lastDeployedTxHash) {
    navigator.clipboard.writeText(lastDeployedTxHash).then(() => {
      showToast("Transaction hash copied!", "info");
    });
  }
}

// ============================================================================
// 4. Real-Time Telemetry & Terminal Logger
// ============================================================================

function appendTerminalLog(message, style = "info") {
  const container = document.getElementById("terminalLogs");
  if (!container) return;

  const row = document.createElement("div");
  row.className = "log-entry";

  const stamp = new Date().toTimeString().split(" ")[0];
  let colorClass = "log-cyan";
  if (style === "success" || style === "green") colorClass = "log-green";
  if (style === "error" || style === "rose") colorClass = "log-rose";
  if (style === "warn" || style === "amber") colorClass = "log-amber";

  row.innerHTML = `<span class="log-stamp">[${stamp}]</span> <span class="${colorClass}">${escapeHtml(message)}</span>`;
  container.appendChild(row);

  const window = document.getElementById("terminalWindow");
  if (window) {
    window.scrollTop = window.scrollHeight;
  }
}

function clearConsole() {
  const container = document.getElementById("terminalLogs");
  if (container) container.innerHTML = "";
  appendTerminalLog("Terminal output reset by operator.", "info");
}

function escapeHtml(text) {
  const div = document.createElement("div");
  div.innerText = text;
  return div.innerHTML;
}

function showToast(message, type = "info") {
  const container = document.getElementById("toastContainer");
  if (!container) return;

  const toast = document.createElement("div");
  toast.className = `toast ${type}`;
  toast.innerHTML = `<span>${escapeHtml(message)}</span>`;

  container.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = "0";
    toast.style.transform = "translateX(20px)";
    toast.style.transition = "all 0.3s ease";
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}

// Poll Node Consensus Telemetry
async function pollConsensusTelemetry() {
  const heightElem = document.getElementById("telemetryHeight");
  try {
    const res = await fetch(`${AETHER_NETWORK.rpc}/status`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();

    if (data.result && data.result.sync_info) {
      const height = data.result.sync_info.latest_block_height;
      if (heightElem) {
        heightElem.innerText = `#${parseInt(height, 10).toLocaleString()}`;
      }
    }
  } catch (err) {
    if (heightElem) heightElem.innerText = "Connecting...";
  }
}

// Global Startup Initializer
window.addEventListener("DOMContentLoaded", () => {
  appendTerminalLog("===================================================", "cyan");
  appendTerminalLog("AETHER NETWORK COMMERCIAL WEB PORTAL v1.0.0 ONLINE", "cyan");
  appendTerminalLog("Active Chain: aether-1 | Node Endpoint: 34.207.176.57", "info");
  appendTerminalLog("Base Denomination: ucurts (Curtis 1:1,000,000)", "info");
  appendTerminalLog("Ready for Keplr wallet handshake.", "info");
  appendTerminalLog("===================================================", "cyan");

  updateDenomPreview();
  pollConsensusTelemetry();
  setInterval(pollConsensusTelemetry, 3500);

  // If Keplr is already unlocked in browser, listen for keystore change
  if (window.keplr) {
    window.addEventListener("keplr_keystorechange", () => {
      appendTerminalLog("Keplr account switch detected. Refreshing profile...", "amber");
      connectWallet();
    });
  }
});
