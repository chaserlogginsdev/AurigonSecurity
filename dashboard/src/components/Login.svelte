<script>
  import { api } from '../lib/api.js';
  import { saveSession } from '../lib/stores.js';

  export let onLogin = () => {};

  let tenantSlug = '';
  let username = '';
  let password = '';
  let error = null;
  let loading = false;

  async function login() {
    loading = true; error = null;
    if (!tenantSlug.trim()) { error = 'Please enter your workspace ID.'; loading = false; return; }
    try {
      const data = await api.login(tenantSlug.trim(), username, password);
      saveSession(data.token, data.username, data.role, data.tenant_id, data.tenant_name);
      onLogin();
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  function keydown(e) { if (e.key === 'Enter') login(); }
</script>

<div class="shell">
  <div class="card">
    <div class="brand">
      <span class="icon">⬡</span>
      <span class="name">Aurigon Security</span>
    </div>
    <p class="sub">Sign in to your dashboard</p>

    {#if error}
      <div class="error">{error}</div>
    {/if}

    <div class="field">
      <label for="t">Workspace ID</label>
      <input id="t" type="text" placeholder="e.g. acme"
        bind:value={tenantSlug} on:keydown={keydown} autocomplete="organization"/>
    </div>
    <div class="field">
      <label for="u">Username</label>
      <input id="u" type="text" placeholder="admin"
        bind:value={username} on:keydown={keydown} autocomplete="username"/>
    </div>
    <div class="field">
      <label for="p">Password</label>
      <input id="p" type="password" placeholder="••••••••"
        bind:value={password} on:keydown={keydown} autocomplete="current-password"/>
    </div>
    <button on:click={login} disabled={loading}>
      {loading ? 'Signing in…' : 'Sign in'}
    </button>
  </div>
</div>

<style>
  .shell { min-height: 100vh; display: flex; align-items: center; justify-content: center; background: #0d0f12; }
  .card { width: 360px; background: #111318; border: 1px solid #1e2028; border-radius: 14px; padding: 36px 32px; display: flex; flex-direction: column; gap: 16px; }
  .brand { display: flex; align-items: center; gap: 10px; }
  .icon { font-size: 24px; color: #6c8fff; }
  .name { font-size: 18px; font-weight: 600; color: #f0f1f3; }
  .sub { font-size: 13px; color: #4a4f5e; margin-top: -8px; }
  .error { background: #2a1010; border: 1px solid #5a2020; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #e55; }
  .field { display: flex; flex-direction: column; gap: 6px; }
  label { font-size: 11px; font-weight: 600; color: #6a7090; text-transform: uppercase; letter-spacing: 0.07em; }
  input { background: #0d0f12; border: 1px solid #1e2028; border-radius: 8px; color: #d0d3e0; font-size: 14px; padding: 10px 14px; outline: none; transition: border-color 0.15s; font-family: inherit; }
  input:focus { border-color: #6c8fff55; }
  input::placeholder { color: #2e3248; }
  button { background: #6c8fff; color: #fff; border: none; border-radius: 8px; padding: 11px; font-size: 14px; font-weight: 600; cursor: pointer; transition: background 0.15s, opacity 0.15s; font-family: inherit; }
  button:hover { background: #5a7aee; }
  button:disabled { opacity: 0.6; cursor: not-allowed; }
</style>