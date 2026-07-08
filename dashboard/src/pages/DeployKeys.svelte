<script>
  import { onMount } from 'svelte';
  import { token } from '../lib/stores.js';
  import { api, formatDate, formatDateTime } from '../lib/api.js';

  let keys = [];
  let loading = true;
  let error = null;

  let label = '';
  let backendURL = '';
  let generatedKey = null;
  let generating = false;
  let generateError = null;
  let copied = false;

  onMount(async () => {
    backendURL = window.location.origin;
    await load();
  });

  async function load() {
    loading = true; error = null;
    try { keys = await api.getDeployKeys($token) || []; }
    catch (e) { error = e.message; }
    finally { loading = false; }
  }

  async function generate() {
    if (!label.trim()) { generateError = 'Please enter a label.'; return; }
    if (!backendURL.trim()) { generateError = 'Please enter the backend URL.'; return; }
    generating = true; generateError = null; generatedKey = null; copied = false;
    try {
      const data = await api.generateDeployKey($token, label.trim(), backendURL.trim());
      generatedKey = data.token;
      label = '';
      await load();
    } catch (e) { generateError = e.message; }
    finally { generating = false; }
  }

  async function revoke(id) {
    if (!confirm('Revoke this key? Agents using it will stop reporting immediately.')) return;
    try {
      await api.revokeDeployKey($token, id);
      await load();
    } catch (e) { alert('Failed to revoke: ' + e.message); }
  }

  async function copy() {
    await navigator.clipboard.writeText(generatedKey);
    copied = true;
    setTimeout(() => copied = false, 2000);
  }

  $: activeKeys  = keys.filter(k => !k.revoked);
  $: revokedKeys = keys.filter(k => k.revoked);
</script>

<div class="page">
  <div class="topbar">
    <div>
      <h1>Deploy Keys</h1>
      <p class="sub">Generate keys to authenticate agents — one per customer or environment</p>
    </div>
  </div>

  <div class="layout">
    <!-- Generate -->
    <div class="card generate-card">
      <div class="card-title">Generate a new key</div>
      <p class="card-sub">
        Share the key with the agent installer. That's all the IT admin needs —
        no backend URL, no separate credentials.
      </p>

      {#if generatedKey}
        <div class="key-result">
          <div class="key-result-header">
            <span class="check">✓</span>
            <span>Key generated — copy it now</span>
          </div>
          <div class="key-token-row">
            <code class="key-token">{generatedKey}</code>
            <button class="copy-btn {copied ? 'copied' : ''}" on:click={copy}>
              {copied ? '✓ Copied' : 'Copy'}
            </button>
          </div>
          <p class="key-hint">
            This key won't be shown again. Agents set <code>AURIGON_DEPLOY_KEY</code> to this value.
          </p>
        </div>
      {/if}

      {#if generateError}
        <div class="err">{generateError}</div>
      {/if}

      <div class="fields">
        <div class="field">
          <label>Label</label>
          <input type="text" placeholder="e.g. Acme Corp or Lab Environment" bind:value={label}/>
        </div>
        <div class="field">
          <label>Backend URL</label>
          <input type="text" placeholder="http://10.0.0.5:8080" bind:value={backendURL}/>
          <span class="field-hint">Agents will connect to this address</span>
        </div>
        <button class="generate-btn" on:click={generate} disabled={generating}>
          {generating ? 'Generating…' : 'Generate Deploy Key'}
        </button>
      </div>
    </div>

    <!-- Active keys -->
    <div class="keys-section">
      <div class="section-title">Active keys ({activeKeys.length})</div>

      {#if loading}
        <div class="state"><div class="spinner"></div></div>
      {:else if error}
        <div class="err">{error}</div>
      {:else if activeKeys.length === 0}
        <div class="empty">No active keys. Generate one to get started.</div>
      {:else}
        <div class="key-list">
          {#each activeKeys as key}
            <div class="key-row">
              <div class="key-info">
                <div class="key-label">{key.label}</div>
                <div class="key-meta">
                  Created {formatDate(key.created_at)} by {key.created_by}
                  {#if key.last_used} · Last used {formatDateTime(key.last_used)}{/if}
                </div>
                <div class="key-id">ID: {key.id}</div>
              </div>
              <button class="revoke-btn" on:click={() => revoke(key.id)}>Revoke</button>
            </div>
          {/each}
        </div>
      {/if}

      {#if revokedKeys.length > 0}
        <div class="section-title" style="margin-top:24px">Revoked ({revokedKeys.length})</div>
        <div class="key-list">
          {#each revokedKeys as key}
            <div class="key-row revoked">
              <div class="key-info">
                <div class="key-label">{key.label} <span class="revoked-tag">Revoked</span></div>
                <div class="key-meta">
                  Created {formatDate(key.created_at)} by {key.created_by}
                  {#if key.last_used} · Last used {formatDateTime(key.last_used)}{/if}
                </div>
                <div class="key-id">ID: {key.id}</div>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .page { padding: 36px 40px; max-width: 1100px; }
  .topbar { margin-bottom: 28px; }
  h1 { font-size: 22px; font-weight: 700; color: #f0f1f3; letter-spacing: -0.02em; }
  .sub { font-size: 13px; color: #4a4f5e; margin-top: 3px; }
  .layout { display: grid; grid-template-columns: 400px 1fr; gap: 24px; align-items: start; }
  .card { background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 24px; }
  .card-title { font-size: 14px; font-weight: 600; color: #e2e4e9; margin-bottom: 6px; }
  .card-sub { font-size: 13px; color: #4a4f5e; line-height: 1.6; margin-bottom: 20px; }
  .section-title { font-size: 12px; font-weight: 600; color: #6a7090; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 12px; }
  .fields { display: flex; flex-direction: column; gap: 14px; }
  .field { display: flex; flex-direction: column; gap: 5px; }
  label { font-size: 11px; font-weight: 600; color: #6a7090; text-transform: uppercase; letter-spacing: 0.07em; }
  input { background: #0d0f12; border: 1px solid #1e2028; border-radius: 7px; color: #d0d3e0; font-size: 13px; padding: 9px 12px; outline: none; font-family: inherit; transition: border-color 0.15s; }
  input:focus { border-color: #6c8fff55; }
  input::placeholder { color: #2e3248; }
  .field-hint { font-size: 11px; color: #3a3f52; }
  .generate-btn { background: #6c8fff; color: #fff; border: none; border-radius: 7px; padding: 10px; font-size: 13px; font-weight: 600; cursor: pointer; font-family: inherit; transition: background 0.15s; }
  .generate-btn:hover { background: #5a7aee; }
  .generate-btn:disabled { opacity: 0.6; cursor: not-allowed; }

  .key-result { background: #0a1a10; border: 1px solid #1a4a2a; border-radius: 8px; padding: 16px; margin-bottom: 18px; }
  .key-result-header { display: flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 600; color: #3ecf8e; margin-bottom: 12px; }
  .check { font-size: 16px; }
  .key-token-row { display: flex; gap: 8px; align-items: flex-start; margin-bottom: 10px; }
  .key-token { background: #060810; border: 1px solid #1e2028; border-radius: 6px; padding: 8px 10px; font-size: 10px; font-family: 'JetBrains Mono', monospace; color: #6c8fff; word-break: break-all; flex: 1; display: block; }
  .copy-btn { background: #1a2240; color: #6c8fff; border: 1px solid #6c8fff44; border-radius: 6px; padding: 6px 12px; font-size: 12px; font-weight: 600; cursor: pointer; flex-shrink: 0; font-family: inherit; white-space: nowrap; transition: all 0.15s; }
  .copy-btn.copied { background: #0d2e1f; color: #3ecf8e; border-color: #3ecf8e44; }
  .key-hint { font-size: 11px; color: #3a4a5e; line-height: 1.5; }
  .key-hint code { background: #1e2028; padding: 1px 5px; border-radius: 3px; font-family: 'JetBrains Mono', monospace; color: #6a7090; }

  .key-list { display: flex; flex-direction: column; gap: 8px; }
  .key-row { display: flex; align-items: center; justify-content: space-between; background: #111318; border: 1px solid #1e2028; border-radius: 8px; padding: 14px 16px; gap: 12px; }
  .key-row.revoked { opacity: 0.45; }
  .key-info { flex: 1; min-width: 0; }
  .key-label { font-size: 13px; font-weight: 600; color: #d0d3e0; margin-bottom: 4px; }
  .key-meta { font-size: 11px; color: #4a4f5e; }
  .key-id { font-size: 10px; color: #3a3f52; font-family: 'JetBrains Mono', monospace; margin-top: 3px; }
  .revoked-tag { font-size: 10px; background: #2a1010; color: #e55; padding: 1px 6px; border-radius: 4px; font-weight: 600; margin-left: 6px; }
  .revoke-btn { background: #2a1010; color: #e55; border: 1px solid #5a2020; border-radius: 6px; padding: 5px 12px; font-size: 11px; font-weight: 600; cursor: pointer; flex-shrink: 0; font-family: inherit; transition: background 0.15s; }
  .revoke-btn:hover { background: #3a1515; }

  .err { background: #2a1010; border: 1px solid #5a2020; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #e55; margin-bottom: 14px; }
  .empty { font-size: 13px; color: #4a4f5e; padding: 20px 0; }
  .state { display: flex; align-items: center; justify-content: center; padding: 40px; }
  .spinner { width: 20px; height: 20px; border: 2px solid #1e2028; border-top-color: #6c8fff; border-radius: 50%; animation: spin 0.7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>