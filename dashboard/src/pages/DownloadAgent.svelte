<script>
  import { onMount } from 'svelte';
  import { token } from '../lib/stores.js';
  import { api } from '../lib/api.js';

  let agentKey = null;
  let backendURL = '';
  let loading = true;
  let error = null;
  let copied = false;
  let psCopied = false;

  onMount(load);

  async function load() {
    loading = true; error = null;
    try {
      const data = await api.getAgentKey($token);
      agentKey = data.token;
      backendURL = data.backend_url;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function copyKey() {
    await navigator.clipboard.writeText(agentKey);
    copied = true;
    setTimeout(() => copied = false, 2000);
  }

  $: psCommand = agentKey
    ? `.\\AurigonAgentSetup.exe /SILENT /AGENTKEY="${agentKey}"`
    : '';

  async function copyPs() {
    await navigator.clipboard.writeText(psCommand);
    psCopied = true;
    setTimeout(() => psCopied = false, 2000);
  }
</script>

<div class="page">
  <div class="topbar">
    <div>
      <h1>Download Agent</h1>
      <p class="sub">Deploy the Aurigon agent to any Windows machine using your permanent agent key</p>
    </div>
  </div>

  {#if loading}
    <div class="state"><div class="spinner"></div></div>
  {:else if error}
    <div class="err">{error}</div>
  {:else}
    <div class="layout">
      <!-- Download card -->
      <div class="card">
        <div class="card-title">1. Download the installer</div>
        <p class="card-sub">Runs as a Windows service on any machine you want to manage.</p>
        <a class="download-btn" href="/downloads/AurigonAgentSetup.exe" download>
          ⬇ Download AurigonAgentSetup.exe
        </a>
      </div>

      <!-- Agent key card -->
      <div class="card">
        <div class="card-title">2. Your agent key</div>
        <p class="card-sub">
          This key is permanent and unique to your workspace. Every agent you install
          uses this same key — you never need to generate a new one per machine.
        </p>

        <div class="key-box">
          <code class="key-text">{agentKey}</code>
          <button class="copy-btn {copied ? 'copied' : ''}" on:click={copyKey}>
            {copied ? '✓ Copied' : 'Copy'}
          </button>
        </div>

        <p class="key-hint">Connects to: <code>{backendURL}</code></p>
      </div>

      <!-- Install instructions card -->
      <div class="card">
        <div class="card-title">3. Install</div>

        <div class="install-method">
          <div class="method-label">Option A — Run the installer</div>
          <p class="method-text">
            Double-click <code>AurigonAgentSetup.exe</code> on the target machine, paste your
            agent key when prompted, and finish the wizard. The agent installs as a
            Windows service and starts automatically.
          </p>
        </div>

        <div class="install-method">
          <div class="method-label">Option B — Silent install via PowerShell</div>
          <p class="method-text">For scripted deployment (Intune, GPO, SCCM, or manual PowerShell):</p>
          <div class="code-box">
            <code>{psCommand}</code>
            <button class="copy-btn small {psCopied ? 'copied' : ''}" on:click={copyPs}>
              {psCopied ? '✓' : 'Copy'}
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .page { padding: 36px 40px; max-width: 800px; }
  .topbar { margin-bottom: 28px; }
  h1 { font-size: 22px; font-weight: 700; color: #f0f1f3; letter-spacing: -0.02em; }
  .sub { font-size: 13px; color: #4a4f5e; margin-top: 3px; }

  .layout { display: flex; flex-direction: column; gap: 16px; }
  .card { background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 24px; }
  .card-title { font-size: 14px; font-weight: 700; color: #e2e4e9; margin-bottom: 6px; }
  .card-sub { font-size: 13px; color: #4a4f5e; line-height: 1.6; margin-bottom: 16px; }

  .download-btn {
    display: inline-flex; align-items: center; gap: 8px;
    background: linear-gradient(135deg, #7d9cff, #6c8fff);
    color: #fff; text-decoration: none;
    padding: 11px 20px; border-radius: 8px;
    font-size: 14px; font-weight: 600;
    transition: all 0.15s;
  }
  .download-btn:hover { transform: translateY(-1px); box-shadow: 0 6px 20px rgba(108,143,255,.35); }

  .key-box { display: flex; align-items: center; gap: 10px; background: #0a0c10; border: 1px solid #1e2028; border-radius: 8px; padding: 12px 14px; }
  .key-text { flex: 1; font-size: 11px; font-family: 'JetBrains Mono', monospace; color: #6c8fff; word-break: break-all; }
  .copy-btn { background: #1a2240; color: #6c8fff; border: 1px solid #6c8fff44; border-radius: 6px; padding: 7px 14px; font-size: 12px; font-weight: 600; cursor: pointer; flex-shrink: 0; font-family: inherit; transition: all 0.15s; white-space: nowrap; }
  .copy-btn:hover { background: #24306a; }
  .copy-btn.copied { background: #0d2e1f; color: #3ecf8e; border-color: #3ecf8e44; }
  .copy-btn.small { padding: 4px 10px; font-size: 11px; }
  .key-hint { font-size: 12px; color: #3a4a5e; margin-top: 10px; }
  .key-hint code { background: #1e2028; padding: 1px 6px; border-radius: 3px; font-family: 'JetBrains Mono', monospace; color: #6a7090; }

  .install-method { margin-bottom: 18px; }
  .install-method:last-child { margin-bottom: 0; }
  .method-label { font-size: 12px; font-weight: 700; color: #8a8fa8; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 6px; }
  .method-text { font-size: 13px; color: #6a7090; line-height: 1.6; margin-bottom: 10px; }
  .method-text code { background: #1a1d26; padding: 1px 6px; border-radius: 3px; font-family: 'JetBrains Mono', monospace; color: #8a8fa8; }

  .code-box { display: flex; align-items: center; gap: 10px; background: #0a0c10; border: 1px solid #1e2028; border-radius: 8px; padding: 10px 14px; }
  .code-box code { flex: 1; font-size: 11px; font-family: 'JetBrains Mono', monospace; color: #8a8fa8; word-break: break-all; }

  .err { background: #2a1010; border: 1px solid #5a2020; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #e55; }
  .state { display: flex; align-items: center; justify-content: center; padding: 60px; }
  .spinner { width: 22px; height: 22px; border: 2px solid #1e2028; border-top-color: #6c8fff; border-radius: 50%; animation: spin 0.7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>