<script>
  import { onMount, onDestroy } from 'svelte';
  import { selectedMachine, machineTab, token, currentUser } from '../lib/stores.js';
  import { api, formatDate, formatDateTime, isOnline } from '../lib/api.js';

  let accounts = [];
  let groups = [];
  let actions = [];
  let loading = true;
  let error = null;
  let search = '';
  let filter = 'all';
  let actionStatus = {};
  let pollInterval;

  // Add account modal state
  let showAddModal = false;
  let newUsername = '';
  let newPassword = '';
  let newIsAdmin = false;
  let addError = null;
  let addLoading = false;

  $: machine = $selectedMachine;

  async function load() {
    if (!machine) return;
    loading = true; error = null;
    try {
      const [accs, acts] = await Promise.all([
        api.getAccounts($token, machine.id),
        api.getActionStatus($token, machine.id),
      ]);
      accounts = accs || [];
      actions = acts || [];
      actionStatus = {};
      for (const a of actions) {
        if (a.status === 'pending') actionStatus[a.username] = 'pending';
      }
    } catch (e) {
      if (e.status === 401) { /* handled by app */ return; }
      error = e.message;
    } finally { loading = false; }
  }

  async function loadGroups() {
    if (!machine || groups.length > 0) return;
    try { groups = await api.getGroups($token, machine.id) || []; }
    catch {}
  }

  async function triggerAction(type, username, params = {}) {
    actionStatus = { ...actionStatus, [username]: 'pending' };
    try {
      await api.createAction($token, machine.id, type, username, params);
    } catch (e) {
      actionStatus = { ...actionStatus, [username]: null };
      alert('Failed to queue action: ' + e.message);
    }
  }

  function openAddModal() {
    showAddModal = true;
    newUsername = ''; newPassword = ''; newIsAdmin = false; addError = null;
  }

  function closeAddModal() {
    showAddModal = false;
  }

  async function submitAddAccount() {
    addError = null;
    if (!newUsername.trim()) { addError = 'Username is required.'; return; }
    if (newPassword.length < 8) { addError = 'Password must be at least 8 characters.'; return; }
    addLoading = true;
    try {
      await api.createAction($token, machine.id, 'create_account', newUsername.trim(), {
        password: newPassword,
        is_admin: newIsAdmin ? 'true' : 'false',
      });
      showAddModal = false;
      actionStatus = { ...actionStatus, [newUsername.trim()]: 'pending' };
    } catch (e) {
      addError = e.message;
    } finally {
      addLoading = false;
    }
  }

  async function deleteAccount(username) {
    if (!confirm(`Delete account "${username}" from ${machine.hostname}? This cannot be undone.`)) return;
    await triggerAction('delete_account', username);
  }

  async function toggleAdmin(username, currentlyAdmin) {
    const verb = currentlyAdmin ? 'remove admin privileges from' : 'grant admin privileges to';
    if (!confirm(`Are you sure you want to ${verb} "${username}"?`)) return;
    await triggerAction(currentlyAdmin ? 'remove_admin' : 'set_admin', username);
  }

  function startPolling() {
    clearInterval(pollInterval);
    pollInterval = setInterval(async () => {
      try {
        const acts = await api.getActionStatus($token, machine.id);
        const newStatus = {};
        for (const a of acts) {
          if (a.status === 'pending') newStatus[a.username] = 'pending';
        }
        const wasPending = Object.values(actionStatus).some(s => s === 'pending');
        const stillPending = Object.values(newStatus).some(s => s === 'pending');
        actionStatus = newStatus;
        // Always refresh the account list on this tick — not just when a
        // pending action clears — so the UI never shows stale data for
        // accounts that were changed by a previous action or by the agent's
        // own inventory sync.
        if (wasPending && !stillPending) {
          await load();
        } else {
          accounts = await api.getAccounts($token, machine.id) || accounts;
        }
      } catch {}
    }, 8000);
  }

  onMount(() => { load(); startPolling(); });
  onDestroy(() => clearInterval(pollInterval));

  $: if (machine) { load(); startPolling(); }
  $: if ($machineTab === 'groups') loadGroups();

  $: filtered = accounts.filter(a => {
    const s = search.toLowerCase();
    const matchSearch = a.username.toLowerCase().includes(s) || (a.sid||'').toLowerCase().includes(s);
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
</script>

{#if machine}
<div class="page">
  <!-- Header -->
  <div class="topbar">
    <div class="topbar-left">
      <div class="machine-header">
        <span class="status-dot {isOnline(machine.last_seen) ? 'online' : 'offline'}"></span>
        <h1>{machine.hostname}</h1>
        <span class="machine-id">{machine.id}</span>
      </div>
      <p class="sub">Last seen {formatDateTime(machine.last_seen)}</p>
    </div>
  </div>

  <!-- Tabs -->
  <div class="tabs">
    <button class="tab {$machineTab === 'accounts' ? 'active' : ''}"
      on:click={() => machineTab.set('accounts')}>Accounts</button>
    <button class="tab {$machineTab === 'groups' ? 'active' : ''}"
      on:click={() => machineTab.set('groups')}>Groups</button>
    <button class="tab {$machineTab === 'actions' ? 'active' : ''}"
      on:click={() => machineTab.set('actions')}>
      Actions
      {#if actions.filter(a => a.status === 'pending').length > 0}
        <span class="tab-badge">{actions.filter(a => a.status === 'pending').length}</span>
      {/if}
    </button>
  </div>

  <!-- ── Accounts tab ── -->
  {#if $machineTab === 'accounts'}
    <div class="tab-controls">
      <div class="stats-row">
        <button class="stat {filter==='all'?'active':''}" on:click={()=>filter='all'}>
          <span class="n">{stats.total}</span><span class="l">Total</span>
        </button>
        <button class="stat {filter==='enabled'?'active':''}" on:click={()=>filter='enabled'}>
          <span class="n green">{stats.enabled}</span><span class="l">Enabled</span>
        </button>
        <button class="stat {filter==='disabled'?'active':''}" on:click={()=>filter='disabled'}>
          <span class="n muted">{stats.disabled}</span><span class="l">Disabled</span>
        </button>
        <button class="stat {filter==='admin'?'active':''}" on:click={()=>filter='admin'}>
          <span class="n amber">{stats.admins}</span><span class="l">Admins</span>
        </button>
      </div>
      <div class="tab-controls-right">
        <input class="search" type="text" placeholder="Search username or SID…" bind:value={search}/>
        <button class="add-btn" on:click={openAddModal}>+ Add Account</button>
      </div>
    </div>

    {#if loading}
      <div class="state"><div class="spinner"></div></div>
    {:else if error}
      <div class="state error"><p>{error}</p></div>
    {:else if filtered.length === 0}
      <div class="state"><p>No accounts match.</p></div>
    {:else}
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>Username</th><th>Status</th><th>Role</th><th>Last logon</th><th>Description</th><th>Actions</th></tr>
          </thead>
          <tbody>
            {#each filtered as a}
              <tr>
                <td class="bold">{a.username}</td>
                <td>
                  {#if a.enabled}
                    <span class="badge green">Enabled</span>
                  {:else}
                    <span class="badge gray">Disabled</span>
                  {/if}
                </td>
                <td>
                  {#if a.is_admin}
                    <span class="badge amber">Admin</span>
                  {:else}
                    <span class="badge ghost">User</span>
                  {/if}
                </td>
                <td class="muted">{formatDate(a.last_logon)}</td>
                <td class="muted">{a.description || '—'}</td>
                <td>
                  {#if a.username === $currentUser}
                    <span class="self">—</span>
                  {:else if actionStatus[a.username] === 'pending'}
                    <span class="pending"><span class="mini-spin"></span> Pending…</span>
                  {:else}
                    <div class="act-group">
                      {#if a.enabled}
                        <button class="act disable" on:click={() => triggerAction('disable_account', a.username)}>Disable</button>
                      {:else}
                        <button class="act enable" on:click={() => triggerAction('enable_account', a.username)}>Enable</button>
                      {/if}
                      <button class="act admin" on:click={() => toggleAdmin(a.username, a.is_admin)}>
                        {a.is_admin ? 'Remove Admin' : 'Make Admin'}
                      </button>
                      <button class="act delete" on:click={() => deleteAccount(a.username)}>Delete</button>
                    </div>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

  <!-- ── Groups tab ── -->
  {:else if $machineTab === 'groups'}
    {#if groups.length === 0}
      <div class="state"><div class="spinner"></div></div>
    {:else}
      <div class="groups-grid">
        {#each groups as group}
          <div class="group-card">
            <div class="group-name">{group.name}</div>
            {#if group.description}<div class="group-desc">{group.description}</div>{/if}
            <div class="group-members">
              {#each (group.members || []) as member}
                <span class="member">{member}</span>
              {/each}
              {#if !group.members || group.members.length === 0}
                <span class="no-members">No members</span>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}

  <!-- ── Actions tab ── -->
  {:else if $machineTab === 'actions'}
    {#if actions.length === 0}
      <div class="state"><p>No actions yet for this machine.</p></div>
    {:else}
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>Type</th><th>Username</th><th>Status</th><th>By</th><th>Created</th><th>Executed</th><th>Result</th></tr>
          </thead>
          <tbody>
            {#each actions as a}
              <tr>
                <td><code class="action-type">{a.type}</code></td>
                <td class="bold">{a.username}</td>
                <td>
                  {#if a.status === 'completed'}
                    <span class="badge green">Completed</span>
                  {:else if a.status === 'pending'}
                    <span class="badge amber">Pending</span>
                  {:else}
                    <span class="badge red">Failed</span>
                  {/if}
                </td>
                <td class="muted">{a.created_by}</td>
                <td class="muted">{formatDateTime(a.created_at)}</td>
                <td class="muted">{formatDateTime(a.executed_at)}</td>
                <td class="muted">{a.result || '—'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}

  <!-- ── Add Account Modal ── -->
  {#if showAddModal}
    <div class="modal-overlay" on:click={closeAddModal}>
      <div class="modal" on:click|stopPropagation>
        <div class="modal-title">Add local account</div>
        <p class="modal-sub">Creates a new local user account on {machine.hostname}.</p>

        {#if addError}
          <div class="modal-error">{addError}</div>
        {/if}

        <div class="modal-fields">
          <div class="field">
            <label for="new-username">Username</label>
            <input id="new-username" type="text" placeholder="jsmith" bind:value={newUsername}/>
          </div>
          <div class="field">
            <label for="new-password">Password</label>
            <input id="new-password" type="password" placeholder="Min 8 characters" bind:value={newPassword}/>
          </div>
          <label class="checkbox-row">
            <input type="checkbox" bind:checked={newIsAdmin}/>
            <span>Grant local administrator privileges</span>
          </label>
        </div>

        <div class="modal-actions">
          <button class="modal-cancel" on:click={closeAddModal}>Cancel</button>
          <button class="modal-submit" on:click={submitAddAccount} disabled={addLoading}>
            {addLoading ? 'Creating…' : 'Create Account'}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
{/if}

<style>
  .page { padding: 36px 40px; max-width: 1200px; display: flex; flex-direction: column; gap: 0; }
  .topbar { margin-bottom: 24px; }
  .machine-header { display: flex; align-items: center; gap: 10px; margin-bottom: 4px; }
  h1 { font-size: 22px; font-weight: 700; color: #f0f1f3; letter-spacing: -0.02em; }
  .machine-id { font-size: 12px; color: #3a3f52; font-family: 'JetBrains Mono', monospace; }
  .sub { font-size: 12px; color: #4a4f5e; }
  .status-dot { width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0; }
  .status-dot.online { background: #3ecf8e; box-shadow: 0 0 6px #3ecf8e66; }
  .status-dot.offline { background: #3a3f52; }

  .tabs { display: flex; gap: 0; border-bottom: 1px solid #1e2028; margin-bottom: 24px; }
  .tab { background: none; border: none; border-bottom: 2px solid transparent; color: #6a7090; font-size: 13px; font-weight: 500; padding: 10px 18px; cursor: pointer; transition: all 0.15s; font-family: inherit; margin-bottom: -1px; display: flex; align-items: center; gap: 6px; }
  .tab:hover { color: #c8cad4; }
  .tab.active { color: #6c8fff; border-bottom-color: #6c8fff; }
  .tab-badge { background: #6c8fff; color: #fff; font-size: 10px; font-weight: 700; padding: 1px 5px; border-radius: 8px; }

  .tab-controls { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20px; gap: 16px; flex-wrap: wrap; }
  .tab-controls-right { display: flex; align-items: center; gap: 10px; }
  .add-btn { background: linear-gradient(135deg, #7d9cff, #6c8fff); color: #fff; border: none; border-radius: 8px; padding: 9px 16px; font-size: 13px; font-weight: 600; cursor: pointer; font-family: inherit; transition: all 0.15s; white-space: nowrap; }
  .add-btn:hover { transform: translateY(-1px); box-shadow: 0 4px 14px rgba(108,143,255,.35); }
  .stats-row { display: flex; gap: 10px; }
  .stat { display: flex; flex-direction: column; gap: 2px; background: #111318; border: 1px solid #1e2028; border-radius: 8px; padding: 12px 16px; cursor: pointer; min-width: 80px; text-align: left; font-family: inherit; transition: all 0.12s; border: 1px solid #1e2028; }
  .stat:hover { border-color: #2a2f3e; }
  .stat.active { border-color: #6c8fff44; background: #111b30; }
  .n { font-size: 20px; font-weight: 700; color: #f0f1f3; font-family: 'JetBrains Mono', monospace; line-height: 1; }
  .n.green { color: #3ecf8e; }
  .n.muted { color: #4a4f5e; }
  .n.amber { color: #f5a623; }
  .l { font-size: 10px; color: #4a4f5e; text-transform: uppercase; letter-spacing: 0.06em; }
  .search { background: #111318; border: 1px solid #1e2028; border-radius: 8px; color: #d0d3e0; font-size: 13px; padding: 8px 14px; width: 240px; outline: none; transition: border-color 0.15s; font-family: inherit; }
  .search:focus { border-color: #6c8fff55; }
  .search::placeholder { color: #3a3f52; }

  .table-wrap { background: #111318; border: 1px solid #1e2028; border-radius: 10px; overflow: hidden; }
  table { width: 100%; border-collapse: collapse; }
  thead tr { border-bottom: 1px solid #1e2028; }
  th { text-align: left; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.07em; color: #4a4f5e; padding: 12px 16px; }
  tbody tr { border-bottom: 1px solid #171921; transition: background 0.1s; }
  tbody tr:last-child { border-bottom: none; }
  tbody tr:hover { background: #13161e; }
  td { padding: 12px 16px; font-size: 13px; color: #c8cad4; vertical-align: middle; }
  .bold { font-weight: 600; color: #e2e4e9; }
  .muted { color: #4a4f5e; font-size: 12px; }

  .badge { display: inline-block; font-size: 10px; font-weight: 700; padding: 2px 7px; border-radius: 4px; letter-spacing: 0.04em; }
  .badge.green  { background: #0d2e1f; color: #3ecf8e; }
  .badge.gray   { background: #1a1d25; color: #4a4f5e; }
  .badge.amber  { background: #2e1f08; color: #f5a623; }
  .badge.red    { background: #2a1010; color: #e55; }
  .badge.ghost  { background: transparent; color: #3a3f52; border: 1px solid #1e2028; }

  .act-group { display: flex; gap: 6px; flex-wrap: wrap; }
  .act { font-size: 11px; font-weight: 600; padding: 4px 10px; border-radius: 5px; border: none; cursor: pointer; font-family: inherit; white-space: nowrap; }
  .act.disable { background: #2a1010; color: #e55; border: 1px solid #5a2020; }
  .act.disable:hover { background: #3a1515; }
  .act.enable  { background: #0d2e1f; color: #3ecf8e; border: 1px solid #1a5a3a; }
  .act.enable:hover  { background: #0f3824; }
  .act.admin   { background: #2e1f08; color: #f5a623; border: 1px solid #4a3510; }
  .act.admin:hover { background: #3a2810; }
  .act.delete  { background: #1a1d25; color: #6a7090; border: 1px solid #262b38; }
  .act.delete:hover { background: #2a1010; color: #e55; border-color: #5a2020; }
  .pending { display: inline-flex; align-items: center; gap: 6px; font-size: 11px; color: #4a4f5e; }
  .self { color: #2a2f3e; }
  .action-type { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #6a7090; background: #1a1d26; padding: 2px 6px; border-radius: 4px; }

  .groups-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 12px; }
  .group-card { background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 18px 20px; }
  .group-name { font-size: 14px; font-weight: 600; color: #e2e4e9; margin-bottom: 4px; }
  .group-desc { font-size: 12px; color: #4a4f5e; margin-bottom: 10px; }
  .group-members { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 10px; }
  .member { font-size: 11px; background: #1a1d26; color: #6a7090; border-radius: 4px; padding: 2px 8px; font-family: 'JetBrains Mono', monospace; }
  .no-members { font-size: 11px; color: #3a3f52; font-style: italic; }

  .state { display: flex; align-items: center; justify-content: center; padding: 60px; color: #4a4f5e; font-size: 13px; }
  .state.error { color: #e55; }
  .spinner { width: 22px; height: 22px; border: 2px solid #1e2028; border-top-color: #6c8fff; border-radius: 50%; animation: spin 0.7s linear infinite; }
  .mini-spin { display: inline-block; width: 10px; height: 10px; border: 1.5px solid #2a2f3e; border-top-color: #6c8fff; border-radius: 50%; animation: spin 0.7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }

  /* ── Modal ── */
  .modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,.6); display: flex; align-items: center; justify-content: center; z-index: 100; backdrop-filter: blur(2px); }
  .modal { width: 380px; background: #14161c; border: 1px solid #262b38; border-radius: 12px; padding: 26px; box-shadow: 0 20px 60px rgba(0,0,0,.5); }
  .modal-title { font-size: 16px; font-weight: 700; color: #f0f1f3; margin-bottom: 4px; }
  .modal-sub { font-size: 12px; color: #4a4f5e; margin-bottom: 18px; line-height: 1.5; }
  .modal-error { background: #2a1010; border: 1px solid #5a2020; border-radius: 7px; padding: 9px 12px; font-size: 12px; color: #e55; margin-bottom: 14px; }
  .modal-fields { display: flex; flex-direction: column; gap: 14px; margin-bottom: 20px; }
  .modal-fields .field { display: flex; flex-direction: column; gap: 5px; }
  .modal-fields label { font-size: 11px; font-weight: 600; color: #6a7090; text-transform: uppercase; letter-spacing: 0.06em; }
  .modal-fields input[type="text"], .modal-fields input[type="password"] { background: #0d0f12; border: 1px solid #1e2028; border-radius: 7px; color: #d0d3e0; font-size: 13px; padding: 9px 12px; outline: none; font-family: inherit; transition: border-color 0.15s; }
  .modal-fields input:focus { border-color: #6c8fff55; }
  .modal-fields input::placeholder { color: #2e3248; }
  .checkbox-row { display: flex; align-items: center; gap: 8px; font-size: 12px; color: #8a8fa8; cursor: pointer; }
  .checkbox-row input { width: 14px; height: 14px; accent-color: #6c8fff; cursor: pointer; }
  .modal-actions { display: flex; justify-content: flex-end; gap: 10px; }
  .modal-cancel { background: none; border: 1px solid #262b38; color: #6a7090; border-radius: 7px; padding: 8px 16px; font-size: 13px; font-weight: 600; cursor: pointer; font-family: inherit; }
  .modal-cancel:hover { color: #c8cad4; }
  .modal-submit { background: #6c8fff; color: #fff; border: none; border-radius: 7px; padding: 8px 18px; font-size: 13px; font-weight: 600; cursor: pointer; font-family: inherit; transition: background 0.15s; }
  .modal-submit:hover { background: #5a7aee; }
  .modal-submit:disabled { opacity: 0.6; cursor: not-allowed; }
</style>