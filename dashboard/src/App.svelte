<script>
  import { onMount } from 'svelte';

  let token = null;
  let currentUser = null;
  let currentRole = null;
  let loginUsername = '';
  let loginPassword = '';
  let loginError = null;
  let loginLoading = false;

  let machines = [];
  let accounts = [];
  let selectedMachine = null;
  let loading = false;
  let error = null;
  let search = '';
  let filter = 'all';
  let actionStatus = {};
  let recentActions = [];

  // View: 'accounts' | 'settings' | 'deploy'
  let view = 'accounts';

  // Settings state
  let currentPassword = '';
  let newPassword = '';
  let confirmPassword = '';
  let passwordError = null;
  let passwordSuccess = false;
  let passwordLoading = false;

  // Deploy keys state
  let deployKeys = [];
  let deployKeysLoading = false;
  let deployKeysError = null;
  let newKeyLabel = '';
  let newKeyBackendURL = '';
  let generatedKey = null;
  let generateLoading = false;
  let generateError = null;

  const BASE = '';

  onMount(() => {
    const saved = sessionStorage.getItem('aurigon_token');
    const savedUser = sessionStorage.getItem('aurigon_user');
    const savedRole = sessionStorage.getItem('aurigon_role');
    if (saved) {
      token = saved;
      currentUser = savedUser;
      currentRole = savedRole;
      loadMachines();
    }
  });

  // ── Auth ────────────────────────────────────────────────────────────────

  async function login() {
    loginLoading = true; loginError = null;
    try {
      const res = await fetch(`${BASE}/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: loginUsername, password: loginPassword }),
      });
      if (!res.ok) throw new Error('Invalid username or password');
      const data = await res.json();
      token = data.token;
      currentUser = data.username;
      currentRole = data.role;
      sessionStorage.setItem('aurigon_token', token);
      sessionStorage.setItem('aurigon_user', currentUser);
      sessionStorage.setItem('aurigon_role', currentRole);
      await loadMachines();
    } catch (e) { loginError = e.message; }
    finally { loginLoading = false; }
  }

  function logout() {
    token = null; currentUser = null; currentRole = null;
    machines = []; accounts = []; selectedMachine = null;
    view = 'accounts';
    sessionStorage.removeItem('aurigon_token');
    sessionStorage.removeItem('aurigon_user');
    sessionStorage.removeItem('aurigon_role');
  }

  function authHeaders() {
    return { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' };
  }

  // ── Change password ──────────────────────────────────────────────────────

  async function changePassword() {
    passwordError = null; passwordSuccess = false;
    if (newPassword.length < 8) { passwordError = 'New password must be at least 8 characters.'; return; }
    if (newPassword !== confirmPassword) { passwordError = 'New passwords do not match.'; return; }
    passwordLoading = true;
    try {
      const res = await fetch(`${BASE}/change-password`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      });
      if (!res.ok) throw new Error(await res.text());
      passwordSuccess = true;
      currentPassword = ''; newPassword = ''; confirmPassword = '';
    } catch (e) { passwordError = e.message; }
    finally { passwordLoading = false; }
  }

  function openSettings() {
    view = 'settings';
    passwordError = null; passwordSuccess = false;
    currentPassword = ''; newPassword = ''; confirmPassword = '';
  }

  // ── Deploy keys ──────────────────────────────────────────────────────────

  async function openDeploy() {
    view = 'deploy';
    generatedKey = null;
    generateError = null;
    newKeyLabel = '';
    newKeyBackendURL = window.location.origin;
    await loadDeployKeys();
  }

  async function loadDeployKeys() {
    deployKeysLoading = true; deployKeysError = null;
    try {
      const res = await fetch(`${BASE}/deploy-keys`, { headers: authHeaders() });
      if (!res.ok) throw new Error(await res.text());
      deployKeys = await res.json();
    } catch (e) { deployKeysError = e.message; }
    finally { deployKeysLoading = false; }
  }

  async function generateKey() {
    if (!newKeyLabel.trim()) { generateError = 'Please enter a label.'; return; }
    if (!newKeyBackendURL.trim()) { generateError = 'Please enter the backend URL.'; return; }
    generateLoading = true; generateError = null; generatedKey = null;
    try {
      const res = await fetch(`${BASE}/deploy-keys/generate`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ label: newKeyLabel.trim(), backend_url: newKeyBackendURL.trim() }),
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      generatedKey = data.token;
      newKeyLabel = '';
      await loadDeployKeys();
    } catch (e) { generateError = e.message; }
    finally { generateLoading = false; }
  }

  async function revokeKey(id) {
    if (!confirm('Revoke this deploy key? Agents using it will stop reporting.')) return;
    try {
      const res = await fetch(`${BASE}/deploy-keys/revoke`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ id }),
      });
      if (!res.ok) throw new Error(await res.text());
      await loadDeployKeys();
    } catch (e) { alert('Failed to revoke key: ' + e.message); }
  }

  function copyToClipboard(text) {
    navigator.clipboard.writeText(text);
  }

  // ── Machines & accounts ──────────────────────────────────────────────────

  async function loadMachines() {
    loading = true; error = null;
    try {
      const res = await fetch(`${BASE}/machines`, { headers: authHeaders() });
      if (res.status === 401) { logout(); return; }
      if (!res.ok) throw new Error(await res.text());
      machines = await res.json();
      if (machines.length > 0) await selectMachine(machines[0]);
      else loading = false;
    } catch (e) { error = e.message; loading = false; }
  }

  async function selectMachine(machine) {
    selectedMachine = machine; loading = true; error = null;
    search = ''; filter = 'all'; actionStatus = {};
    view = 'accounts';
    try {
      const [accRes, actRes] = await Promise.all([
        fetch(`${BASE}/accounts?machine_id=${machine.id}`, { headers: authHeaders() }),
        fetch(`${BASE}/actions/status?machine_id=${machine.id}`, { headers: authHeaders() }),
      ]);
      if (accRes.status === 401) { logout(); return; }
      accounts = await accRes.json();
      recentActions = actRes.ok ? await actRes.json() : [];
      actionStatus = {};
      for (const a of recentActions) {
        if (a.status === 'pending') actionStatus[a.username] = 'pending';
      }
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  async function triggerAction(type, username) {
    if (!selectedMachine) return;
    actionStatus = { ...actionStatus, [username]: 'pending' };
    try {
      const res = await fetch(`${BASE}/actions/create`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ machine_id: selectedMachine.id, type, username }),
      });
      if (!res.ok) throw new Error(await res.text());
    } catch (e) {
      actionStatus = { ...actionStatus, [username]: 'error' };
      alert(`Failed to queue action: ${e.message}`);
    }
  }

  let pollInterval;
  $: if (selectedMachine) {
    clearInterval(pollInterval);
    pollInterval = setInterval(async () => {
      try {
        const res = await fetch(`${BASE}/actions/status?machine_id=${selectedMachine.id}`, { headers: authHeaders() });
        if (!res.ok) return;
        const actions = await res.json();
        const newStatus = {};
        for (const a of actions) {
          if (a.status === 'pending') newStatus[a.username] = 'pending';
        }
        const hadPending = Object.values(actionStatus).some(s => s === 'pending');
        const stillPending = Object.values(newStatus).some(s => s === 'pending');
        if (hadPending && !stillPending) await selectMachine(selectedMachine);
        else actionStatus = newStatus;
      } catch {}
    }, 10000);
  }

  $: filtered = accounts.filter(a => {
    const matchSearch =
      a.username.toLowerCase().includes(search.toLowerCase()) ||
      (a.sid || '').toLowerCase().includes(search.toLowerCase());
    const matchFilter =
      filter === 'all'      ? true :
      filter === 'enabled'  ? a.enabled :
      filter === 'disabled' ? !a.enabled :
      filter === 'admin'    ? a.is_admin : true;
    return matchSearch && matchFilter;
  });

  $: stats = {
    total:    accounts.length,
    enabled:  accounts.filter(a => a.enabled).length,
    disabled: accounts.filter(a => !a.enabled).length,
    admins:   accounts.filter(a => a.is_admin).length,
  };

  function formatDate(d) {
    if (!d) return '—';
    try { return new Date(d).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' }); }
    catch { return '—'; }
  }

  function isOnline(lastSeen) {
    if (!lastSeen) return false;
    return (Date.now() - new Date(lastSeen).getTime()) < 5 * 60 * 1000;
  }

  function handleKeydown(e) { if (e.key === 'Enter') login(); }
</script>

<!-- ── Login ── -->
{#if !token}
<div class="login-shell">
  <div class="login-card">
    <div class="login-brand">
      <span class="login-icon">⬡</span>
      <span class="login-name">Aurigon</span>
    </div>
    <p class="login-sub">Sign in to your dashboard</p>
    {#if loginError}<div class="login-error">{loginError}</div>{/if}
    <div class="field">
      <label class="field-label" for="username">Username</label>
      <input id="username" class="field-input" type="text" placeholder="admin"
        bind:value={loginUsername} on:keydown={handleKeydown} autocomplete="username"/>
    </div>
    <div class="field">
      <label class="field-label" for="password">Password</label>
      <input id="password" class="field-input" type="password" placeholder="••••••••"
        bind:value={loginPassword} on:keydown={handleKeydown} autocomplete="current-password"/>
    </div>
    <button class="login-btn" on:click={login} disabled={loginLoading}>
      {loginLoading ? 'Signing in…' : 'Sign in'}
    </button>
  </div>
</div>

<!-- ── Dashboard ── -->
{:else}
<div class="shell">
  <aside class="sidebar">
    <div class="brand">
      <span class="brand-icon">⬡</span>
      <span class="brand-name">Aurigon</span>
    </div>
    <nav class="nav">
      <div class="nav-label">Machines</div>
      {#if machines.length === 0}
        <p class="no-machines">No machines yet.</p>
      {:else}
        {#each machines as machine}
          <button class="machine-item {selectedMachine?.id === machine.id && view === 'accounts' ? 'active' : ''}"
            on:click={() => selectMachine(machine)}>
            <span class="status-dot {isOnline(machine.last_seen) ? 'online' : 'offline'}"></span>
            <span class="machine-hostname">{machine.hostname}</span>
          </button>
        {/each}
      {/if}

      <div class="nav-label" style="margin-top:20px">Views</div>
      <a class="nav-item nav-disabled" href="#groups">
        <span class="nav-icon">◉</span> Groups <span class="nav-soon">soon</span>
      </a>
    </nav>

    <div class="sidebar-footer">
      {#if currentRole === 'admin'}
        <button class="settings-btn {view === 'deploy' ? 'active' : ''}" on:click={openDeploy}>
          🔑 Deploy Keys
        </button>
      {/if}
      <button class="settings-btn {view === 'settings' ? 'active' : ''}" on:click={openSettings}>
        ⚙ Settings
      </button>
      <div class="user-row">
        <span class="user-name">{currentUser}</span>
        <button class="logout-btn" on:click={logout}>Sign out</button>
      </div>
      <div class="machine-pill">
        <span class="pill-dot"></span>
        <span class="pill-label">{machines.length} machine{machines.length !== 1 ? 's' : ''} registered</span>
      </div>
    </div>
  </aside>

  <main class="main">

    <!-- ── Deploy Keys view ── -->
    {#if view === 'deploy'}
      <header class="topbar">
        <div class="topbar-left">
          <h1 class="page-title">Deploy Keys</h1>
          <p class="page-sub">Generate keys to authenticate agents — one key per customer or environment</p>
        </div>
      </header>

      <!-- Generate new key -->
      <div class="settings-card" style="margin-bottom:24px">
        <h2 class="settings-section-title">Generate a new deploy key</h2>
        <p class="settings-section-sub">
          Give it a label so you know which customer or environment it belongs to.
          Share the key with the agent installer — that's all the IT admin needs.
        </p>

        {#if generatedKey}
          <div class="key-result">
            <div class="key-result-label">✓ Deploy key generated — copy it now, it won't be shown again</div>
            <div class="key-token-wrap">
              <code class="key-token">{generatedKey}</code>
              <button class="copy-btn" on:click={() => copyToClipboard(generatedKey)}>Copy</button>
            </div>
            <p class="key-note">
              Agents use this key as <code>AURIGON_DEPLOY_KEY</code> — it encodes the backend URL and auth secret automatically.
            </p>
          </div>
        {/if}

        {#if generateError}
          <div class="pw-error">{generateError}</div>
        {/if}

        <div class="settings-fields">
          <div class="field">
            <label class="field-label" for="key-label">Label (e.g. "Acme Corp" or "Lab Environment")</label>
            <input id="key-label" class="field-input" type="text" placeholder="Customer or environment name"
              bind:value={newKeyLabel}/>
          </div>
          <div class="field">
            <label class="field-label" for="key-url">Backend URL (what agents will connect to)</label>
            <input id="key-url" class="field-input" type="text" placeholder="http://10.0.0.5:8080"
              bind:value={newKeyBackendURL}/>
          </div>
          <button class="save-btn" on:click={generateKey} disabled={generateLoading}>
            {generateLoading ? 'Generating…' : 'Generate Deploy Key'}
          </button>
        </div>
      </div>

      <!-- Active keys list -->
      <div class="settings-card">
        <h2 class="settings-section-title">Active deploy keys</h2>
        {#if deployKeysLoading}
          <div class="state-box"><div class="spinner"></div><p>Loading…</p></div>
        {:else if deployKeysError}
          <div class="pw-error">{deployKeysError}</div>
        {:else if deployKeys.length === 0}
          <p class="settings-section-sub">No deploy keys yet. Generate one above.</p>
        {:else}
          <div class="key-list">
            {#each deployKeys as key}
              <div class="key-row {key.revoked ? 'revoked' : ''}">
                <div class="key-info">
                  <div class="key-label-text">{key.label}</div>
                  <div class="key-meta">
                    Created {formatDate(key.created_at)} by {key.created_by}
                    {#if key.last_used} · Last used {formatDate(key.last_used)}{/if}
                    {#if key.revoked} · <span class="revoked-badge">Revoked</span>{/if}
                  </div>
                  <div class="key-id">ID: {key.id}</div>
                </div>
                {#if !key.revoked}
                  <button class="revoke-btn" on:click={() => revokeKey(key.id)}>Revoke</button>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>

    <!-- ── Settings view ── -->
    {:else if view === 'settings'}
      <header class="topbar">
        <div class="topbar-left">
          <h1 class="page-title">Settings</h1>
          <p class="page-sub">Manage your account</p>
        </div>
      </header>

      <div class="settings-card">
        <h2 class="settings-section-title">Change password</h2>
        <p class="settings-section-sub">You are signed in as <strong>{currentUser}</strong>.</p>
        {#if passwordSuccess}
          <div class="pw-success">Password changed successfully.</div>
        {/if}
        {#if passwordError}
          <div class="pw-error">{passwordError}</div>
        {/if}
        <div class="settings-fields">
          <div class="field">
            <label class="field-label" for="cur-pw">Current password</label>
            <input id="cur-pw" class="field-input" type="password" placeholder="••••••••"
              bind:value={currentPassword} autocomplete="current-password"/>
          </div>
          <div class="field">
            <label class="field-label" for="new-pw">New password</label>
            <input id="new-pw" class="field-input" type="password" placeholder="Min 8 characters"
              bind:value={newPassword} autocomplete="new-password"/>
          </div>
          <div class="field">
            <label class="field-label" for="confirm-pw">Confirm new password</label>
            <input id="confirm-pw" class="field-input" type="password" placeholder="••••••••"
              bind:value={confirmPassword} autocomplete="new-password"/>
          </div>
          <button class="save-btn" on:click={changePassword} disabled={passwordLoading}>
            {passwordLoading ? 'Saving…' : 'Update password'}
          </button>
        </div>
      </div>

    <!-- ── Accounts view ── -->
    {:else}
      <header class="topbar">
        <div class="topbar-left">
          <h1 class="page-title">Local accounts</h1>
          {#if selectedMachine}
            <p class="page-sub">Windows · {selectedMachine.hostname} · {selectedMachine.id}</p>
          {/if}
        </div>
        <input class="search" type="text" placeholder="Search username or SID…" bind:value={search}/>
      </header>

      <div class="stats-row">
        <button class="stat-card {filter==='all'?'active':''}" on:click={()=>filter='all'}>
          <span class="stat-num">{stats.total}</span><span class="stat-label">Total</span>
        </button>
        <button class="stat-card {filter==='enabled'?'active':''}" on:click={()=>filter='enabled'}>
          <span class="stat-num green">{stats.enabled}</span><span class="stat-label">Enabled</span>
        </button>
        <button class="stat-card {filter==='disabled'?'active':''}" on:click={()=>filter='disabled'}>
          <span class="stat-num muted">{stats.disabled}</span><span class="stat-label">Disabled</span>
        </button>
        <button class="stat-card {filter==='admin'?'active':''}" on:click={()=>filter='admin'}>
          <span class="stat-num amber">{stats.admins}</span><span class="stat-label">Admins</span>
        </button>
      </div>

      {#if loading}
        <div class="state-box"><div class="spinner"></div><p>Loading…</p></div>
      {:else if error}
        <div class="state-box error">
          <p class="error-title">Could not load accounts</p>
          <p class="error-detail">{error}</p>
        </div>
      {:else if machines.length === 0}
        <div class="state-box">
          <p class="empty-title">No machines yet</p>
          <p class="empty-sub">Run the agent on a machine to get started.</p>
        </div>
      {:else if filtered.length === 0}
        <div class="state-box"><p>No accounts match your search.</p></div>
      {:else}
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Username</th><th>Status</th><th>Role</th>
                <th>Last logon</th><th>SID</th><th>Description</th><th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {#each filtered as account}
                <tr>
                  <td class="td-username">{account.username}</td>
                  <td>
                    {#if account.enabled}
                      <span class="badge badge-green">Enabled</span>
                    {:else}
                      <span class="badge badge-gray">Disabled</span>
                    {/if}
                  </td>
                  <td>
                    {#if account.is_admin}
                      <span class="badge badge-amber">Admin</span>
                    {:else}
                      <span class="badge badge-ghost">User</span>
                    {/if}
                  </td>
                  <td class="td-muted">{formatDate(account.last_logon)}</td>
                  <td class="td-sid">{account.sid || '—'}</td>
                  <td class="td-muted">{account.description || '—'}</td>
                  <td class="td-actions">
                    {#if account.username === currentUser}
                      <span class="action-self">—</span>
                    {:else if actionStatus[account.username] === 'pending'}
                      <span class="action-pending">
                        <span class="mini-spinner"></span> Pending…
                      </span>
                    {:else if account.enabled}
                      <button class="action-btn action-disable"
                        on:click={() => triggerAction('disable_account', account.username)}>
                        Disable
                      </button>
                    {:else}
                      <button class="action-btn action-enable"
                        on:click={() => triggerAction('enable_account', account.username)}>
                        Enable
                      </button>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    {/if}

  </main>
</div>
{/if}

<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

  :global(body) {
    background: #0d0f12; color: #e2e4e9;
    font-family: 'Inter', system-ui, sans-serif; font-size: 14px; line-height: 1.5;
  }

  /* ── Login ── */
  .login-shell { min-height: 100vh; display: flex; align-items: center; justify-content: center; }
  .login-card { width: 360px; background: #111318; border: 1px solid #1e2028; border-radius: 14px; padding: 36px 32px; display: flex; flex-direction: column; gap: 16px; }
  .login-brand { display: flex; align-items: center; gap: 10px; }
  .login-icon { font-size: 24px; color: #6c8fff; }
  .login-name { font-size: 20px; font-weight: 600; color: #f0f1f3; }
  .login-sub { font-size: 13px; color: #4a4f5e; margin-top: -8px; }
  .login-error { background: #2a1010; border: 1px solid #5a2020; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #e55; }
  .field { display: flex; flex-direction: column; gap: 6px; }
  .field-label { font-size: 12px; font-weight: 500; color: #6a7090; text-transform: uppercase; letter-spacing: 0.06em; }
  .field-input { background: #0d0f12; border: 1px solid #1e2028; border-radius: 8px; color: #d0d3e0; font-size: 14px; padding: 10px 14px; outline: none; transition: border-color 0.15s; font-family: inherit; }
  .field-input:focus { border-color: #6c8fff55; }
  .field-input::placeholder { color: #2e3248; }
  .login-btn { background: #6c8fff; color: #fff; border: none; border-radius: 8px; padding: 11px; font-size: 14px; font-weight: 600; cursor: pointer; transition: background 0.15s, opacity 0.15s; font-family: inherit; }
  .login-btn:hover { background: #5a7aee; }
  .login-btn:disabled { opacity: 0.6; cursor: not-allowed; }

  /* ── Shell ── */
  .shell { display: flex; min-height: 100vh; }
  .sidebar { width: 220px; min-height: 100vh; background: #111318; border-right: 1px solid #1e2028; display: flex; flex-direction: column; padding: 24px 0; flex-shrink: 0; }
  .brand { display: flex; align-items: center; gap: 10px; padding: 0 20px 28px; border-bottom: 1px solid #1e2028; }
  .brand-icon { font-size: 22px; color: #6c8fff; }
  .brand-name { font-size: 16px; font-weight: 600; color: #f0f1f3; letter-spacing: 0.02em; }
  .nav { padding: 20px 12px 0; flex: 1; overflow-y: auto; }
  .nav-label { font-size: 10px; font-weight: 600; letter-spacing: 0.1em; text-transform: uppercase; color: #4a4f5e; padding: 0 8px; margin-bottom: 6px; }
  .machine-item { display: flex; align-items: center; gap: 8px; width: 100%; padding: 8px 10px; border-radius: 6px; background: none; border: none; color: #8a8fa8; font-size: 13px; cursor: pointer; text-align: left; margin-bottom: 2px; transition: background 0.15s, color 0.15s; }
  .machine-item:hover { background: #1a1d25; color: #d0d3e0; }
  .machine-item.active { background: #1a2240; color: #6c8fff; }
  .status-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
  .status-dot.online { background: #3ecf8e; box-shadow: 0 0 6px #3ecf8e88; }
  .status-dot.offline { background: #3a3f52; }
  .machine-hostname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .no-machines { font-size: 12px; color: #3a3f52; padding: 4px 10px; }
  .nav-item { display: flex; align-items: center; gap: 8px; padding: 8px 10px; border-radius: 6px; color: #8a8fa8; text-decoration: none; font-size: 13px; margin-bottom: 2px; }
  .nav-disabled { opacity: 0.4; pointer-events: none; }
  .nav-icon { font-size: 14px; }
  .nav-soon { margin-left: auto; font-size: 10px; background: #1e2028; color: #4a4f5e; padding: 1px 6px; border-radius: 4px; }

  .sidebar-footer { padding: 16px 20px; margin-top: auto; border-top: 1px solid #1e2028; display: flex; flex-direction: column; gap: 10px; }
  .settings-btn { display: flex; align-items: center; gap: 8px; width: 100%; padding: 7px 10px; border-radius: 6px; background: none; border: 1px solid #1e2028; color: #6a7090; font-size: 12px; cursor: pointer; transition: background 0.15s, color 0.15s, border-color 0.15s; text-align: left; font-family: inherit; }
  .settings-btn:hover { background: #1a1d25; color: #d0d3e0; border-color: #2a2f3e; }
  .settings-btn.active { background: #1a2240; color: #6c8fff; border-color: #6c8fff44; }
  .user-row { display: flex; align-items: center; justify-content: space-between; }
  .user-name { font-size: 13px; color: #6a7090; }
  .logout-btn { background: none; border: 1px solid #1e2028; border-radius: 5px; color: #4a4f5e; font-size: 11px; padding: 3px 8px; cursor: pointer; transition: color 0.15s, border-color 0.15s; font-family: inherit; }
  .logout-btn:hover { color: #e55; border-color: #5a2020; }
  .machine-pill { display: flex; align-items: center; gap: 8px; background: #161920; border: 1px solid #1e2028; border-radius: 8px; padding: 8px 12px; }
  .pill-dot { width: 7px; height: 7px; border-radius: 50%; background: #3ecf8e; box-shadow: 0 0 6px #3ecf8e88; flex-shrink: 0; }
  .pill-label { font-size: 12px; color: #6a7090; }

  /* ── Main ── */
  .main { flex: 1; display: flex; flex-direction: column; min-width: 0; padding: 32px 36px; }
  .topbar { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 28px; gap: 16px; }
  .page-title { font-size: 22px; font-weight: 600; color: #f0f1f3; letter-spacing: -0.01em; }
  .page-sub { font-size: 12px; color: #4a4f5e; margin-top: 3px; font-family: 'JetBrains Mono', monospace; }
  .search { background: #111318; border: 1px solid #1e2028; border-radius: 8px; color: #d0d3e0; font-size: 13px; padding: 8px 14px; width: 260px; outline: none; transition: border-color 0.15s; flex-shrink: 0; font-family: inherit; }
  .search:focus { border-color: #6c8fff55; }
  .search::placeholder { color: #3a3f52; }

  /* ── Settings & Deploy ── */
  .settings-card { background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 28px 32px; max-width: 680px; }
  .settings-section-title { font-size: 16px; font-weight: 600; color: #f0f1f3; margin-bottom: 6px; }
  .settings-section-sub { font-size: 13px; color: #4a4f5e; margin-bottom: 20px; line-height: 1.6; }
  .settings-section-sub strong { color: #8a8fa8; }
  .settings-fields { display: flex; flex-direction: column; gap: 14px; }
  .pw-success { background: #0d2e1f; border: 1px solid #1a5a3a; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #3ecf8e; margin-bottom: 6px; }
  .pw-error { background: #2a1010; border: 1px solid #5a2020; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #e55; margin-bottom: 6px; }
  .save-btn { background: #6c8fff; color: #fff; border: none; border-radius: 8px; padding: 10px 20px; font-size: 14px; font-weight: 600; cursor: pointer; align-self: flex-start; transition: background 0.15s, opacity 0.15s; margin-top: 4px; font-family: inherit; }
  .save-btn:hover { background: #5a7aee; }
  .save-btn:disabled { opacity: 0.6; cursor: not-allowed; }

  /* ── Deploy keys ── */
  .key-result { background: #0d2010; border: 1px solid #1a5a2a; border-radius: 8px; padding: 16px 18px; margin-bottom: 20px; }
  .key-result-label { font-size: 13px; color: #3ecf8e; margin-bottom: 10px; font-weight: 600; }
  .key-token-wrap { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
  .key-token { background: #0a0c10; border: 1px solid #1e2028; border-radius: 6px; padding: 8px 12px; font-size: 11px; font-family: 'JetBrains Mono', monospace; color: #6c8fff; word-break: break-all; flex: 1; }
  .copy-btn { background: #1a2240; color: #6c8fff; border: 1px solid #6c8fff44; border-radius: 6px; padding: 6px 12px; font-size: 12px; font-weight: 600; cursor: pointer; flex-shrink: 0; font-family: inherit; transition: background 0.15s; }
  .copy-btn:hover { background: #2a3860; }
  .key-note { font-size: 12px; color: #4a4f5e; line-height: 1.6; }
  .key-note code { background: #1e2028; padding: 1px 5px; border-radius: 3px; font-family: 'JetBrains Mono', monospace; color: #8a8fa8; }

  .key-list { display: flex; flex-direction: column; gap: 8px; margin-top: 16px; }
  .key-row { display: flex; align-items: center; justify-content: space-between; background: #0d0f12; border: 1px solid #1e2028; border-radius: 8px; padding: 14px 16px; gap: 16px; }
  .key-row.revoked { opacity: 0.45; }
  .key-info { flex: 1; min-width: 0; }
  .key-label-text { font-size: 14px; font-weight: 600; color: #d0d3e0; margin-bottom: 4px; }
  .key-meta { font-size: 12px; color: #4a4f5e; }
  .key-id { font-size: 11px; color: #3a3f52; font-family: 'JetBrains Mono', monospace; margin-top: 3px; }
  .revoked-badge { color: #e55; font-weight: 600; }
  .revoke-btn { background: #2a1010; color: #e55; border: 1px solid #5a2020; border-radius: 6px; padding: 5px 12px; font-size: 12px; font-weight: 600; cursor: pointer; flex-shrink: 0; font-family: inherit; transition: background 0.15s; }
  .revoke-btn:hover { background: #3a1515; }

  /* ── Stats ── */
  .stats-row { display: flex; gap: 12px; margin-bottom: 24px; }
  .stat-card { flex: 1; background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 16px 18px; cursor: pointer; text-align: left; transition: border-color 0.15s, background 0.15s; display: flex; flex-direction: column; gap: 4px; font-family: inherit; }
  .stat-card:hover { border-color: #2a2f3e; background: #13161e; }
  .stat-card.active { border-color: #6c8fff55; background: #111b30; }
  .stat-num { font-size: 26px; font-weight: 600; color: #f0f1f3; line-height: 1; font-family: 'JetBrains Mono', monospace; }
  .stat-num.green { color: #3ecf8e; }
  .stat-num.muted { color: #4a4f5e; }
  .stat-num.amber { color: #f5a623; }
  .stat-label { font-size: 11px; color: #4a4f5e; text-transform: uppercase; letter-spacing: 0.06em; }

  /* ── Table ── */
  .table-wrap { background: #111318; border: 1px solid #1e2028; border-radius: 10px; overflow: hidden; }
  .table { width: 100%; border-collapse: collapse; }
  .table thead tr { border-bottom: 1px solid #1e2028; }
  .table th { text-align: left; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.07em; color: #4a4f5e; padding: 12px 16px; }
  .table tbody tr { border-bottom: 1px solid #171921; transition: background 0.1s; }
  .table tbody tr:last-child { border-bottom: none; }
  .table tbody tr:hover { background: #13161e; }
  .table td { padding: 13px 16px; vertical-align: middle; color: #c8cad4; }
  .td-username { font-weight: 500; color: #e2e4e9; }
  .td-muted { color: #4a4f5e; font-size: 13px; }
  .td-sid { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #3a3f52; }
  .td-actions { white-space: nowrap; }
  .action-btn { font-size: 11px; font-weight: 600; padding: 4px 10px; border-radius: 5px; border: none; cursor: pointer; transition: opacity 0.15s; letter-spacing: 0.03em; font-family: inherit; }
  .action-disable { background: #2a1010; color: #e55; border: 1px solid #5a2020; }
  .action-disable:hover { background: #3a1515; }
  .action-enable { background: #0d2e1f; color: #3ecf8e; border: 1px solid #1a5a3a; }
  .action-enable:hover { background: #0f3824; }
  .action-pending { display: inline-flex; align-items: center; gap: 6px; font-size: 11px; color: #4a4f5e; }
  .action-self { color: #2a2f3e; font-size: 13px; }
  .badge { display: inline-block; font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 4px; letter-spacing: 0.03em; }
  .badge-green  { background: #0d2e1f; color: #3ecf8e; }
  .badge-gray   { background: #1a1d25; color: #4a4f5e; }
  .badge-amber  { background: #2e1f08; color: #f5a623; }
  .badge-ghost  { background: transparent; color: #3a3f52; border: 1px solid #1e2028; }

  /* ── States ── */
  .state-box { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; color: #4a4f5e; padding: 80px 0; }
  .state-box.error { color: #e55; }
  .error-title { font-size: 16px; font-weight: 600; color: #e55; }
  .error-detail { font-size: 13px; color: #a44; font-family: monospace; }
  .empty-title { font-size: 15px; font-weight: 500; color: #4a4f5e; }
  .empty-sub { font-size: 13px; color: #3a3f52; }
  .spinner { width: 24px; height: 24px; border: 2px solid #1e2028; border-top-color: #6c8fff; border-radius: 50%; animation: spin 0.7s linear infinite; }
  .mini-spinner { display: inline-block; width: 10px; height: 10px; border: 1.5px solid #2a2f3e; border-top-color: #6c8fff; border-radius: 50%; animation: spin 0.7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>