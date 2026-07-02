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
  let groups = [];
  let selectedMachine = null;
  let selectedGroup = null;
  let loading = false;
  let error = null;
  let search = '';
  let filter = 'all';
  let groupSearch = '';
  let actionStatus = {};

  let view = 'accounts'; // 'accounts' | 'groups' | 'audit' | 'users' | 'settings'

  // Settings
  let currentPassword = '';
  let newPassword = '';
  let confirmPassword = '';
  let passwordError = null;
  let passwordSuccess = false;
  let passwordLoading = false;

  // Audit log
  let auditLog = [];
  let auditLoading = false;
  let auditError = null;
  let auditSearch = '';

  // User management
  let users = [];
  let usersLoading = false;
  let usersError = null;
  let newUsername = '';
  let newUserPassword = '';
  let newUserRole = 'viewer';
  let userFormError = null;
  let userFormSuccess = null;
  let userFormLoading = false;

  // Account modals
  let showCreateModal = false;
  let createUsername = '';
  let createPassword = '';
  let createIsAdmin = false;
  let createError = null;
  let createLoading = false;
  let showDeleteModal = false;
  let deleteTargetAccount = null;
  let deleteLoading = false;

  // Group modals
  let showCreateGroupModal = false;
  let newGroupName = '';
  let newGroupDesc = '';
  let groupFormError = null;
  let groupFormLoading = false;
  let showAddMemberModal = false;
  let addMemberUsername = '';
  let addMemberError = null;
  let addMemberLoading = false;
  let showDeleteGroupModal = false;
  let deleteTargetGroup = null;

  const BASE = '';

  onMount(() => {
    const saved = sessionStorage.getItem('aurigon_token');
    const savedUser = sessionStorage.getItem('aurigon_user');
    const savedRole = sessionStorage.getItem('aurigon_role');
    if (saved) { token = saved; currentUser = savedUser; currentRole = savedRole; loadMachines(); }
  });

  // ── Auth ──────────────────────────────────────────────────────────────────────

  async function login() {
    loginLoading = true; loginError = null;
    try {
      const res = await fetch(`${BASE}/login`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: loginUsername, password: loginPassword }),
      });
      if (!res.ok) { const msg = await res.text(); throw new Error(msg.trim() || 'Invalid username or password'); }
      const data = await res.json();
      token = data.token; currentUser = data.username; currentRole = data.role;
      sessionStorage.setItem('aurigon_token', token);
      sessionStorage.setItem('aurigon_user', currentUser);
      sessionStorage.setItem('aurigon_role', currentRole);
      await loadMachines();
    } catch (e) { loginError = e.message; }
    finally { loginLoading = false; }
  }

  function logout() {
    token = null; currentUser = null; currentRole = null;
    machines = []; accounts = []; groups = []; selectedMachine = null; selectedGroup = null;
    view = 'accounts'; auditLog = []; users = [];
    sessionStorage.removeItem('aurigon_token');
    sessionStorage.removeItem('aurigon_user');
    sessionStorage.removeItem('aurigon_role');
  }

  function authHeaders() {
    return { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' };
  }

  $: isAdmin = currentRole === 'admin';

  // ── Account modals ────────────────────────────────────────────────────────────

  function openCreateModal() {
    createUsername = ''; createPassword = ''; createIsAdmin = false; createError = null;
    showCreateModal = true;
  }

  function openDeleteModal(account) { deleteTargetAccount = account; showDeleteModal = true; }

  async function submitCreateAccount() {
    createError = null;
    if (!createUsername) { createError = 'Username is required.'; return; }
    if (createPassword.length < 8) { createError = 'Password must be at least 8 characters.'; return; }
    createLoading = true;
    try {
      const res = await fetch(`${BASE}/actions/create`, {
        method: 'POST', headers: authHeaders(),
        body: JSON.stringify({ machine_id: selectedMachine.id, type: 'create_account', username: createUsername,
          params: { password: createPassword, is_admin: createIsAdmin ? 'true' : 'false' } }),
      });
      if (!res.ok) throw new Error(await res.text());
      actionStatus = { ...actionStatus, [createUsername]: 'pending' };
      showCreateModal = false;
    } catch (e) { createError = e.message; }
    finally { createLoading = false; }
  }

  async function submitDeleteAccount() {
    deleteLoading = true;
    try {
      const res = await fetch(`${BASE}/actions/create`, {
        method: 'POST', headers: authHeaders(),
        body: JSON.stringify({ machine_id: selectedMachine.id, type: 'delete_account',
          username: deleteTargetAccount.username, params: {} }),
      });
      if (!res.ok) throw new Error(await res.text());
      actionStatus = { ...actionStatus, [deleteTargetAccount.username]: 'pending' };
      showDeleteModal = false;
    } catch (e) { error = e.message; }
    finally { deleteLoading = false; }
  }

  // ── Group modals ──────────────────────────────────────────────────────────────

  function openCreateGroupModal() {
    newGroupName = ''; newGroupDesc = ''; groupFormError = null;
    showCreateGroupModal = true;
  }

  async function submitCreateGroup() {
    groupFormError = null;
    if (!newGroupName) { groupFormError = 'Group name is required.'; return; }
    groupFormLoading = true;
    try {
      const res = await fetch(`${BASE}/actions/create`, {
        method: 'POST', headers: authHeaders(),
        body: JSON.stringify({ machine_id: selectedMachine.id, type: 'create_group',
          username: newGroupName, params: { description: newGroupDesc } }),
      });
      if (!res.ok) throw new Error(await res.text());
      showCreateGroupModal = false;
    } catch (e) { groupFormError = e.message; }
    finally { groupFormLoading = false; }
  }

  function openAddMemberModal(group) {
    selectedGroup = group; addMemberUsername = ''; addMemberError = null;
    showAddMemberModal = true;
  }

  async function submitAddMember() {
    addMemberError = null;
    if (!addMemberUsername) { addMemberError = 'Username is required.'; return; }
    addMemberLoading = true;
    try {
      const res = await fetch(`${BASE}/actions/create`, {
        method: 'POST', headers: authHeaders(),
        body: JSON.stringify({ machine_id: selectedMachine.id, type: 'add_to_group',
          username: addMemberUsername, params: { group: selectedGroup.name } }),
      });
      if (!res.ok) throw new Error(await res.text());
      showAddMemberModal = false;
    } catch (e) { addMemberError = e.message; }
    finally { addMemberLoading = false; }
  }

  async function removeMember(group, username) {
    if (!confirm(`Remove ${username} from ${group.name}?`)) return;
    try {
      const res = await fetch(`${BASE}/actions/create`, {
        method: 'POST', headers: authHeaders(),
        body: JSON.stringify({ machine_id: selectedMachine.id, type: 'remove_from_group',
          username, params: { group: group.name } }),
      });
      if (!res.ok) throw new Error(await res.text());
    } catch (e) { alert(e.message); }
  }

  function openDeleteGroupModal(group) { deleteTargetGroup = group; showDeleteGroupModal = true; }

  async function submitDeleteGroup() {
    try {
      const res = await fetch(`${BASE}/actions/create`, {
        method: 'POST', headers: authHeaders(),
        body: JSON.stringify({ machine_id: selectedMachine.id, type: 'delete_group',
          username: deleteTargetGroup.name, params: {} }),
      });
      if (!res.ok) throw new Error(await res.text());
      showDeleteGroupModal = false;
    } catch (e) { alert(e.message); }
  }

  // ── Change password ───────────────────────────────────────────────────────────

  async function changePassword() {
    passwordError = null; passwordSuccess = false;
    if (newPassword.length < 8) { passwordError = 'New password must be at least 8 characters.'; return; }
    if (newPassword !== confirmPassword) { passwordError = 'New passwords do not match.'; return; }
    passwordLoading = true;
    try {
      const res = await fetch(`${BASE}/change-password`, {
        method: 'POST', headers: authHeaders(),
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      });
      if (!res.ok) throw new Error(await res.text());
      passwordSuccess = true;
      currentPassword = ''; newPassword = ''; confirmPassword = '';
    } catch (e) { passwordError = e.message; }
    finally { passwordLoading = false; }
  }

  function openSettings() {
    view = 'settings'; passwordError = null; passwordSuccess = false;
    currentPassword = ''; newPassword = ''; confirmPassword = '';
  }

  // ── User management ───────────────────────────────────────────────────────────

  async function openUsers() {
    view = 'users'; usersLoading = true; usersError = null;
    userFormError = null; userFormSuccess = null;
    newUsername = ''; newUserPassword = ''; newUserRole = 'viewer';
    try {
      const res = await fetch(`${BASE}/users`, { headers: authHeaders() });
      if (!res.ok) throw new Error(await res.text());
      users = await res.json();
    } catch (e) { usersError = e.message; }
    finally { usersLoading = false; }
  }

  async function createUser() {
    userFormError = null; userFormSuccess = null;
    if (!newUsername) { userFormError = 'Username is required.'; return; }
    if (newUserPassword.length < 8) { userFormError = 'Password must be at least 8 characters.'; return; }
    userFormLoading = true;
    try {
      const res = await fetch(`${BASE}/users/create`, {
        method: 'POST', headers: authHeaders(),
        body: JSON.stringify({ username: newUsername, password: newUserPassword, role: newUserRole }),
      });
      if (!res.ok) throw new Error(await res.text());
      userFormSuccess = `User "${newUsername}" created successfully.`;
      newUsername = ''; newUserPassword = ''; newUserRole = 'viewer';
      const listRes = await fetch(`${BASE}/users`, { headers: authHeaders() });
      users = await listRes.json();
    } catch (e) { userFormError = e.message; }
    finally { userFormLoading = false; }
  }

  async function deleteUser(username) {
    if (!confirm(`Delete user "${username}"?`)) return;
    try {
      const res = await fetch(`${BASE}/users/delete`, {
        method: 'POST', headers: authHeaders(),
        body: JSON.stringify({ username }),
      });
      if (!res.ok) throw new Error(await res.text());
      users = users.filter(u => u.username !== username);
    } catch (e) { usersError = e.message; }
  }

  // ── Audit log ─────────────────────────────────────────────────────────────────

  async function openAuditLog() {
    view = 'audit'; auditLoading = true; auditError = null;
    try {
      const res = await fetch(`${BASE}/audit`, { headers: authHeaders() });
      if (res.status === 401) { logout(); return; }
      if (!res.ok) throw new Error(await res.text());
      auditLog = await res.json();
    } catch (e) { auditError = e.message; }
    finally { auditLoading = false; }
  }

  $: filteredAudit = auditLog.filter(a =>
    a.username.toLowerCase().includes(auditSearch.toLowerCase()) ||
    a.hostname.toLowerCase().includes(auditSearch.toLowerCase()) ||
    a.created_by.toLowerCase().includes(auditSearch.toLowerCase()) ||
    a.type.toLowerCase().includes(auditSearch.toLowerCase())
  );

  // ── Data ──────────────────────────────────────────────────────────────────────

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
      const recentActions = actRes.ok ? await actRes.json() : [];
      actionStatus = {};
      for (const a of recentActions) {
        if (a.status === 'pending') actionStatus[a.username] = 'pending';
      }
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  async function openGroups() {
    if (!selectedMachine) return;
    view = 'groups'; loading = true; error = null; groupSearch = '';
    try {
      const res = await fetch(`${BASE}/groups?machine_id=${selectedMachine.id}`, { headers: authHeaders() });
      if (!res.ok) throw new Error(await res.text());
      groups = await res.json();
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  async function triggerAction(type, username, params = {}) {
    if (!selectedMachine) return;
    actionStatus = { ...actionStatus, [username]: 'pending' };
    try {
      const res = await fetch(`${BASE}/actions/create`, {
        method: 'POST', headers: authHeaders(),
        body: JSON.stringify({ machine_id: selectedMachine.id, type, username, params }),
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
        for (const a of actions) { if (a.status === 'pending') newStatus[a.username] = 'pending'; }
        const hadPending = Object.values(actionStatus).some(s => s === 'pending');
        const stillPending = Object.values(newStatus).some(s => s === 'pending');
        if (hadPending && !stillPending) await selectMachine(selectedMachine);
        else actionStatus = newStatus;
      } catch {}
    }, 10000);
  }

  $: filtered = accounts.filter(a => {
    const matchSearch = a.username.toLowerCase().includes(search.toLowerCase()) ||
      (a.sid || '').toLowerCase().includes(search.toLowerCase());
    const matchFilter = filter === 'all' ? true : filter === 'enabled' ? a.enabled :
      filter === 'disabled' ? !a.enabled : filter === 'admin' ? a.is_admin : true;
    return matchSearch && matchFilter;
  });

  $: filteredGroups = groups.filter(g =>
    g.name.toLowerCase().includes(groupSearch.toLowerCase()) ||
    (g.description || '').toLowerCase().includes(groupSearch.toLowerCase())
  );

  $: stats = {
    total: accounts.length, enabled: accounts.filter(a => a.enabled).length,
    disabled: accounts.filter(a => !a.enabled).length, admins: accounts.filter(a => a.is_admin).length,
  };

  function formatDate(d) {
    if (!d) return '—';
    try { return new Date(d).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' }); }
    catch { return '—'; }
  }

  function formatDateTime(d) {
    if (!d) return '—';
    try { return new Date(d).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }); }
    catch { return '—'; }
  }

  function actionLabel(type) {
    const labels = { disable_account: 'Disabled', enable_account: 'Enabled', create_account: 'Created',
      delete_account: 'Deleted', create_group: 'Group created', delete_group: 'Group deleted',
      add_to_group: 'Added to group', remove_from_group: 'Removed from group' };
    return labels[type] || type;
  }

  function actionBadgeClass(type) {
    if (['disable_account','delete_account','delete_group','remove_from_group'].includes(type)) return 'badge-red';
    if (['enable_account','create_account','create_group','add_to_group'].includes(type)) return 'badge-green';
    return 'badge-ghost';
  }

  function isOnline(lastSeen) {
    if (!lastSeen) return false;
    return (Date.now() - new Date(lastSeen).getTime()) < 5 * 60 * 1000;
  }

  function handleKeydown(e) { if (e.key === 'Enter') login(); }
  function handleModalKeydown(e) {
    if (e.key === 'Escape') {
      showCreateModal = false; showDeleteModal = false;
      showCreateGroupModal = false; showAddMemberModal = false; showDeleteGroupModal = false;
    }
  }
</script>

<svelte:window on:keydown={handleModalKeydown}/>

{#if !token}
<!-- ── Login ── -->
<div class="login-shell">
  <div class="login-card">
    <div class="login-brand"><span class="login-icon">⬡</span><span class="login-name">Aurigon</span></div>
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

{:else}
<!-- ── Dashboard ── -->
<div class="shell">
  <aside class="sidebar">
    <div class="brand"><span class="brand-icon">⬡</span><span class="brand-name">Aurigon</span></div>
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
      <button class="nav-item-btn {view === 'groups' ? 'active' : ''}" on:click={openGroups}
        disabled={!selectedMachine}>
        <span class="nav-icon">◉</span> Groups
      </button>
      <button class="nav-item-btn {view === 'audit' ? 'active' : ''}" on:click={openAuditLog}>
        <span class="nav-icon">◈</span> Audit log
      </button>
      {#if isAdmin}
        <button class="nav-item-btn {view === 'users' ? 'active' : ''}" on:click={openUsers}>
          <span class="nav-icon">▦</span> Users
        </button>
      {/if}
    </nav>

    <div class="sidebar-footer">
      <button class="settings-btn {view === 'settings' ? 'active' : ''}" on:click={openSettings}>⚙ Settings</button>
      <div class="user-row">
        <div class="user-info">
          <span class="user-name">{currentUser}</span>
          <span class="role-badge {currentRole}">{currentRole}</span>
        </div>
        <button class="logout-btn" on:click={logout}>Sign out</button>
      </div>
      <div class="machine-pill" style="margin-top:10px">
        <span class="pill-dot"></span>
        <span class="pill-label">{machines.length} machine{machines.length !== 1 ? 's' : ''} registered</span>
      </div>
    </div>
  </aside>

  <main class="main">

    <!-- ── Groups view ── -->
    {#if view === 'groups'}
      <header class="topbar">
        <div class="topbar-left">
          <h1 class="page-title">Local groups</h1>
          {#if selectedMachine}<p class="page-sub">Windows · {selectedMachine.hostname}</p>{/if}
        </div>
        <div class="topbar-right">
          <input class="search" type="text" placeholder="Search groups…" bind:value={groupSearch}/>
          {#if isAdmin && selectedMachine}
            <button class="create-account-btn" on:click={openCreateGroupModal}>+ New group</button>
          {/if}
        </div>
      </header>

      {#if loading}
        <div class="state-box"><div class="spinner"></div><p>Loading…</p></div>
      {:else if error}
        <div class="state-box error"><p class="error-title">{error}</p></div>
      {:else if filteredGroups.length === 0}
        <div class="state-box"><p class="empty-title">No groups found</p><p class="empty-sub">Wait for the agent to upload group data.</p></div>
      {:else}
        <div class="groups-grid">
          {#each filteredGroups as group}
            <div class="group-card">
              <div class="group-header">
                <div class="group-info">
                  <span class="group-name">{group.name}</span>
                  <span class="member-count">{group.members.length} member{group.members.length !== 1 ? 's' : ''}</span>
                </div>
                {#if isAdmin}
                  <button class="action-btn action-disable group-delete-btn"
                    on:click={() => openDeleteGroupModal(group)}>Delete</button>
                {/if}
              </div>
              {#if group.description}
                <p class="group-desc">{group.description}</p>
              {/if}
              <div class="member-list">
                {#each group.members as member}
                  <div class="member-row">
                    <span class="member-name">{member}</span>
                    {#if isAdmin}
                      <button class="member-remove" on:click={() => removeMember(group, member)}>×</button>
                    {/if}
                  </div>
                {/each}
                {#if group.members.length === 0}
                  <p class="no-members">No members</p>
                {/if}
              </div>
              {#if isAdmin}
                <button class="add-member-btn" on:click={() => openAddMemberModal(group)}>+ Add member</button>
              {/if}
            </div>
          {/each}
        </div>
      {/if}

    <!-- ── Users view ── -->
    {:else if view === 'users'}
      <header class="topbar"><div class="topbar-left"><h1 class="page-title">Users</h1><p class="page-sub">Manage dashboard access</p></div></header>
      <div class="settings-card" style="margin-bottom:24px">
        <h2 class="settings-section-title">Add user</h2>
        {#if userFormSuccess}<div class="pw-success">{userFormSuccess}</div>{/if}
        {#if userFormError}<div class="pw-error">{userFormError}</div>{/if}
        <div class="settings-fields" style="margin-top:16px">
          <div class="form-row">
            <div class="field" style="flex:1">
              <label class="field-label" for="new-username">Username</label>
              <input id="new-username" class="field-input" type="text" placeholder="johndoe" bind:value={newUsername}/>
            </div>
            <div class="field" style="flex:1">
              <label class="field-label" for="new-user-pw">Password</label>
              <input id="new-user-pw" class="field-input" type="password" placeholder="Min 8 characters" bind:value={newUserPassword}/>
            </div>
            <div class="field">
              <label class="field-label" for="new-user-role">Role</label>
              <select id="new-user-role" class="field-input field-select" bind:value={newUserRole}>
                <option value="viewer">Viewer</option>
                <option value="admin">Admin</option>
              </select>
            </div>
          </div>
          <button class="save-btn" on:click={createUser} disabled={userFormLoading}>{userFormLoading ? 'Creating…' : 'Create user'}</button>
        </div>
      </div>
      {#if usersLoading}
        <div class="state-box"><div class="spinner"></div><p>Loading…</p></div>
      {:else}
        <div class="table-wrap">
          <table class="table">
            <thead><tr><th>Username</th><th>Role</th><th>Created</th><th></th></tr></thead>
            <tbody>
              {#each users as user}
                <tr>
                  <td class="td-username">{user.username}{#if user.username === currentUser}<span class="you-badge">you</span>{/if}</td>
                  <td>{#if user.role === 'admin'}<span class="badge badge-amber">Admin</span>{:else}<span class="badge badge-ghost">Viewer</span>{/if}</td>
                  <td class="td-muted">{formatDate(user.created_at)}</td>
                  <td class="td-actions">
                    {#if user.username !== currentUser}
                      <button class="action-btn action-disable" on:click={() => deleteUser(user.username)}>Delete</button>
                    {:else}<span class="action-self">—</span>{/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}

    <!-- ── Audit log view ── -->
    {:else if view === 'audit'}
      <header class="topbar">
        <div class="topbar-left"><h1 class="page-title">Audit log</h1><p class="page-sub">All actions across all machines</p></div>
        <input class="search" type="text" placeholder="Search…" bind:value={auditSearch}/>
      </header>
      {#if auditLoading}
        <div class="state-box"><div class="spinner"></div><p>Loading…</p></div>
      {:else if filteredAudit.length === 0}
        <div class="state-box"><p class="empty-title">No actions yet</p></div>
      {:else}
        <div class="table-wrap">
          <table class="table">
            <thead><tr><th>When</th><th>Machine</th><th>Action</th><th>Target</th><th>By</th><th>Status</th><th>Result</th></tr></thead>
            <tbody>
              {#each filteredAudit as entry}
                <tr>
                  <td class="td-muted td-mono">{formatDateTime(entry.created_at)}</td>
                  <td class="td-username">{entry.hostname || entry.machine_id}</td>
                  <td><span class="badge {actionBadgeClass(entry.type)}">{actionLabel(entry.type)}</span></td>
                  <td class="td-username">{entry.username}</td>
                  <td class="td-muted">{entry.created_by}</td>
                  <td>
                    {#if entry.status === 'completed'}<span class="badge badge-green">Done</span>
                    {:else if entry.status === 'pending'}<span class="badge badge-amber">Pending</span>
                    {:else if entry.status === 'failed'}<span class="badge badge-red">Failed</span>
                    {:else}<span class="badge badge-ghost">{entry.status}</span>{/if}
                  </td>
                  <td class="td-muted td-result">{entry.result || '—'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}

    <!-- ── Settings view ── -->
    {:else if view === 'settings'}
      <header class="topbar"><div class="topbar-left"><h1 class="page-title">Settings</h1><p class="page-sub">Manage your account</p></div></header>
      <div class="settings-card">
        <h2 class="settings-section-title">Change password</h2>
        <p class="settings-section-sub">Signed in as <strong>{currentUser}</strong> <span class="role-badge {currentRole}" style="margin-left:4px">{currentRole}</span></p>
        {#if passwordSuccess}<div class="pw-success">Password changed successfully.</div>{/if}
        {#if passwordError}<div class="pw-error">{passwordError}</div>{/if}
        <div class="settings-fields">
          <div class="field"><label class="field-label" for="cur-pw">Current password</label><input id="cur-pw" class="field-input" type="password" placeholder="••••••••" bind:value={currentPassword} autocomplete="current-password"/></div>
          <div class="field"><label class="field-label" for="new-pw">New password</label><input id="new-pw" class="field-input" type="password" placeholder="Min 8 characters" bind:value={newPassword} autocomplete="new-password"/></div>
          <div class="field"><label class="field-label" for="confirm-pw">Confirm new password</label><input id="confirm-pw" class="field-input" type="password" placeholder="••••••••" bind:value={confirmPassword} autocomplete="new-password"/></div>
          <button class="save-btn" on:click={changePassword} disabled={passwordLoading}>{passwordLoading ? 'Saving…' : 'Update password'}</button>
        </div>
      </div>

    <!-- ── Accounts view ── -->
    {:else}
      <header class="topbar">
        <div class="topbar-left">
          <h1 class="page-title">Local accounts</h1>
          {#if selectedMachine}<p class="page-sub">Windows · {selectedMachine.hostname} · {selectedMachine.id}</p>{/if}
        </div>
        <div class="topbar-right">
          <input class="search" type="text" placeholder="Search username or SID…" bind:value={search}/>
          {#if isAdmin && selectedMachine}
            <button class="create-account-btn" on:click={openCreateModal}>+ New account</button>
          {/if}
        </div>
      </header>

      <div class="stats-row">
        <button class="stat-card {filter==='all'?'active':''}" on:click={()=>filter='all'}><span class="stat-num">{stats.total}</span><span class="stat-label">Total</span></button>
        <button class="stat-card {filter==='enabled'?'active':''}" on:click={()=>filter='enabled'}><span class="stat-num green">{stats.enabled}</span><span class="stat-label">Enabled</span></button>
        <button class="stat-card {filter==='disabled'?'active':''}" on:click={()=>filter='disabled'}><span class="stat-num muted">{stats.disabled}</span><span class="stat-label">Disabled</span></button>
        <button class="stat-card {filter==='admin'?'active':''}" on:click={()=>filter='admin'}><span class="stat-num amber">{stats.admins}</span><span class="stat-label">Admins</span></button>
      </div>

      {#if loading}
        <div class="state-box"><div class="spinner"></div><p>Loading…</p></div>
      {:else if error}
        <div class="state-box error"><p class="error-title">Could not load accounts</p><p class="error-detail">{error}</p></div>
      {:else if machines.length === 0}
        <div class="state-box"><p class="empty-title">No machines yet</p><p class="empty-sub">Run the agent on a machine to get started.</p></div>
      {:else if filtered.length === 0}
        <div class="state-box"><p>No accounts match your search.</p></div>
      {:else}
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr><th>Username</th><th>Status</th><th>Role</th><th>Last logon</th><th>SID</th><th>Description</th>{#if isAdmin}<th>Actions</th>{/if}</tr>
            </thead>
            <tbody>
              {#each filtered as account}
                <tr>
                  <td class="td-username">{account.username}</td>
                  <td>{#if account.enabled}<span class="badge badge-green">Enabled</span>{:else}<span class="badge badge-gray">Disabled</span>{/if}</td>
                  <td>{#if account.is_admin}<span class="badge badge-amber">Admin</span>{:else}<span class="badge badge-ghost">User</span>{/if}</td>
                  <td class="td-muted">{formatDate(account.last_logon)}</td>
                  <td class="td-sid">{account.sid || '—'}</td>
                  <td class="td-muted">{account.description || '—'}</td>
                  {#if isAdmin}
                    <td class="td-actions">
                      {#if account.username === currentUser}
                        <span class="action-self">—</span>
                      {:else if actionStatus[account.username] === 'pending'}
                        <span class="action-pending"><span class="mini-spinner"></span> Pending…</span>
                      {:else}
                        <div class="action-group">
                          {#if account.enabled}
                            <button class="action-btn action-disable" on:click={() => triggerAction('disable_account', account.username)}>Disable</button>
                          {:else}
                            <button class="action-btn action-enable" on:click={() => triggerAction('enable_account', account.username)}>Enable</button>
                          {/if}
                          <button class="action-btn action-delete" on:click={() => openDeleteModal(account)}>Delete</button>
                        </div>
                      {/if}
                    </td>
                  {/if}
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    {/if}
  </main>
</div>

<!-- ── Modals ── -->

{#if showCreateModal}
<div class="modal-overlay" on:click|self={() => showCreateModal = false}>
  <div class="modal">
    <h2 class="modal-title">Create account</h2>
    <p class="modal-sub">On {selectedMachine?.hostname}</p>
    {#if createError}<div class="pw-error">{createError}</div>{/if}
    <div class="field"><label class="field-label" for="c-username">Username</label><input id="c-username" class="field-input" type="text" placeholder="newuser" bind:value={createUsername}/></div>
    <div class="field"><label class="field-label" for="c-password">Password</label><input id="c-password" class="field-input" type="password" placeholder="Min 8 characters" bind:value={createPassword}/></div>
    <label class="checkbox-row"><input type="checkbox" bind:checked={createIsAdmin}/><span>Add to Administrators group</span></label>
    <div class="modal-actions">
      <button class="modal-cancel" on:click={() => showCreateModal = false}>Cancel</button>
      <button class="save-btn" on:click={submitCreateAccount} disabled={createLoading}>{createLoading ? 'Creating…' : 'Create account'}</button>
    </div>
  </div>
</div>
{/if}

{#if showDeleteModal && deleteTargetAccount}
<div class="modal-overlay" on:click|self={() => showDeleteModal = false}>
  <div class="modal">
    <h2 class="modal-title">Delete account</h2>
    <p class="modal-sub" style="margin-bottom:16px">Delete <strong style="color:#e2e4e9">{deleteTargetAccount.username}</strong> from {selectedMachine?.hostname}? This cannot be undone.</p>
    <div class="modal-actions">
      <button class="modal-cancel" on:click={() => showDeleteModal = false}>Cancel</button>
      <button class="action-btn action-disable" style="padding:8px 18px;font-size:13px" on:click={submitDeleteAccount} disabled={deleteLoading}>{deleteLoading ? 'Deleting…' : 'Delete account'}</button>
    </div>
  </div>
</div>
{/if}

{#if showCreateGroupModal}
<div class="modal-overlay" on:click|self={() => showCreateGroupModal = false}>
  <div class="modal">
    <h2 class="modal-title">Create group</h2>
    <p class="modal-sub">On {selectedMachine?.hostname}</p>
    {#if groupFormError}<div class="pw-error">{groupFormError}</div>{/if}
    <div class="field"><label class="field-label" for="g-name">Group name</label><input id="g-name" class="field-input" type="text" placeholder="e.g. ITAdmins" bind:value={newGroupName}/></div>
    <div class="field"><label class="field-label" for="g-desc">Description (optional)</label><input id="g-desc" class="field-input" type="text" placeholder="Description" bind:value={newGroupDesc}/></div>
    <div class="modal-actions">
      <button class="modal-cancel" on:click={() => showCreateGroupModal = false}>Cancel</button>
      <button class="save-btn" on:click={submitCreateGroup} disabled={groupFormLoading}>{groupFormLoading ? 'Creating…' : 'Create group'}</button>
    </div>
  </div>
</div>
{/if}

{#if showAddMemberModal && selectedGroup}
<div class="modal-overlay" on:click|self={() => showAddMemberModal = false}>
  <div class="modal">
    <h2 class="modal-title">Add member</h2>
    <p class="modal-sub">To group: <strong style="color:#e2e4e9">{selectedGroup.name}</strong></p>
    {#if addMemberError}<div class="pw-error">{addMemberError}</div>{/if}
    <div class="field"><label class="field-label" for="m-username">Username</label><input id="m-username" class="field-input" type="text" placeholder="username" bind:value={addMemberUsername}/></div>
    <div class="modal-actions">
      <button class="modal-cancel" on:click={() => showAddMemberModal = false}>Cancel</button>
      <button class="save-btn" on:click={submitAddMember} disabled={addMemberLoading}>{addMemberLoading ? 'Adding…' : 'Add member'}</button>
    </div>
  </div>
</div>
{/if}

{#if showDeleteGroupModal && deleteTargetGroup}
<div class="modal-overlay" on:click|self={() => showDeleteGroupModal = false}>
  <div class="modal">
    <h2 class="modal-title">Delete group</h2>
    <p class="modal-sub" style="margin-bottom:16px">Delete group <strong style="color:#e2e4e9">{deleteTargetGroup.name}</strong>? This cannot be undone.</p>
    <div class="modal-actions">
      <button class="modal-cancel" on:click={() => showDeleteGroupModal = false}>Cancel</button>
      <button class="action-btn action-disable" style="padding:8px 18px;font-size:13px" on:click={submitDeleteGroup}>Delete group</button>
    </div>
  </div>
</div>
{/if}

{/if}

<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  :global(body) { background: #0d0f12; color: #e2e4e9; font-family: 'Inter', system-ui, sans-serif; font-size: 14px; line-height: 1.5; }

  .login-shell { min-height: 100vh; display: flex; align-items: center; justify-content: center; }
  .login-card { width: 360px; background: #111318; border: 1px solid #1e2028; border-radius: 14px; padding: 36px 32px; display: flex; flex-direction: column; gap: 16px; }
  .login-brand { display: flex; align-items: center; gap: 10px; }
  .login-icon { font-size: 24px; color: #6c8fff; }
  .login-name { font-size: 20px; font-weight: 600; color: #f0f1f3; }
  .login-sub { font-size: 13px; color: #4a4f5e; margin-top: -8px; }
  .login-error { background: #2a1010; border: 1px solid #5a2020; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #e55; }
  .field { display: flex; flex-direction: column; gap: 6px; }
  .field-label { font-size: 12px; font-weight: 500; color: #6a7090; text-transform: uppercase; letter-spacing: 0.06em; }
  .field-input { background: #0d0f12; border: 1px solid #1e2028; border-radius: 8px; color: #d0d3e0; font-size: 14px; padding: 10px 14px; outline: none; transition: border-color 0.15s; }
  .field-input:focus { border-color: #6c8fff55; }
  .field-input::placeholder { color: #2e3248; }
  .field-select { cursor: pointer; }
  .login-btn { background: #6c8fff; color: #fff; border: none; border-radius: 8px; padding: 11px; font-size: 14px; font-weight: 600; cursor: pointer; transition: background 0.15s; }
  .login-btn:hover { background: #5a7aee; }
  .login-btn:disabled { opacity: 0.6; cursor: not-allowed; }

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
  .nav-item-btn { display: flex; align-items: center; gap: 8px; width: 100%; padding: 8px 10px; border-radius: 6px; background: none; border: none; color: #8a8fa8; font-size: 13px; cursor: pointer; text-align: left; margin-bottom: 2px; transition: background 0.15s, color 0.15s; }
  .nav-item-btn:hover { background: #1a1d25; color: #d0d3e0; }
  .nav-item-btn.active { background: #1a2240; color: #6c8fff; }
  .nav-item-btn:disabled { opacity: 0.4; cursor: not-allowed; }
  .status-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
  .status-dot.online { background: #3ecf8e; box-shadow: 0 0 6px #3ecf8e88; }
  .status-dot.offline { background: #3a3f52; }
  .machine-hostname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .no-machines { font-size: 12px; color: #3a3f52; padding: 4px 10px; }
  .nav-icon { font-size: 14px; }
  .sidebar-footer { padding: 16px 20px; margin-top: auto; border-top: 1px solid #1e2028; display: flex; flex-direction: column; gap: 10px; }
  .settings-btn { display: flex; align-items: center; gap: 8px; width: 100%; padding: 7px 10px; border-radius: 6px; background: none; border: 1px solid #1e2028; color: #6a7090; font-size: 12px; cursor: pointer; transition: background 0.15s, color 0.15s, border-color 0.15s; text-align: left; }
  .settings-btn:hover { background: #1a1d25; color: #d0d3e0; border-color: #2a2f3e; }
  .settings-btn.active { background: #1a2240; color: #6c8fff; border-color: #6c8fff44; }
  .user-row { display: flex; align-items: center; justify-content: space-between; }
  .user-info { display: flex; align-items: center; gap: 6px; }
  .user-name { font-size: 13px; color: #6a7090; }
  .role-badge { font-size: 10px; font-weight: 600; padding: 1px 6px; border-radius: 4px; text-transform: uppercase; letter-spacing: 0.05em; }
  .role-badge.admin { background: #2e1f08; color: #f5a623; }
  .role-badge.viewer { background: #1a1d25; color: #4a4f5e; }
  .logout-btn { background: none; border: 1px solid #1e2028; border-radius: 5px; color: #4a4f5e; font-size: 11px; padding: 3px 8px; cursor: pointer; transition: color 0.15s, border-color 0.15s; }
  .logout-btn:hover { color: #e55; border-color: #5a2020; }
  .machine-pill { display: flex; align-items: center; gap: 8px; background: #161920; border: 1px solid #1e2028; border-radius: 8px; padding: 8px 12px; }
  .pill-dot { width: 7px; height: 7px; border-radius: 50%; background: #3ecf8e; box-shadow: 0 0 6px #3ecf8e88; flex-shrink: 0; }
  .pill-label { font-size: 12px; color: #6a7090; }

  .main { flex: 1; display: flex; flex-direction: column; min-width: 0; padding: 32px 36px; }
  .topbar { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 28px; gap: 16px; }
  .topbar-right { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
  .page-title { font-size: 22px; font-weight: 600; color: #f0f1f3; letter-spacing: -0.01em; }
  .page-sub { font-size: 12px; color: #4a4f5e; margin-top: 3px; font-family: 'JetBrains Mono', monospace; }
  .search { background: #111318; border: 1px solid #1e2028; border-radius: 8px; color: #d0d3e0; font-size: 13px; padding: 8px 14px; width: 220px; outline: none; transition: border-color 0.15s; }
  .search:focus { border-color: #6c8fff55; }
  .search::placeholder { color: #3a3f52; }
  .create-account-btn { background: #6c8fff; color: #fff; border: none; border-radius: 8px; padding: 8px 14px; font-size: 13px; font-weight: 600; cursor: pointer; white-space: nowrap; transition: background 0.15s; }
  .create-account-btn:hover { background: #5a7aee; }

  /* Groups grid */
  .groups-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }
  .group-card { background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 18px 20px; display: flex; flex-direction: column; gap: 12px; }
  .group-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; }
  .group-info { display: flex; flex-direction: column; gap: 3px; }
  .group-name { font-size: 15px; font-weight: 600; color: #f0f1f3; }
  .member-count { font-size: 11px; color: #4a4f5e; }
  .group-desc { font-size: 12px; color: #4a4f5e; }
  .group-delete-btn { font-size: 11px; padding: 3px 8px; flex-shrink: 0; }
  .member-list { display: flex; flex-direction: column; gap: 4px; min-height: 20px; }
  .member-row { display: flex; align-items: center; justify-content: space-between; padding: 5px 8px; background: #0d0f12; border-radius: 5px; }
  .member-name { font-size: 13px; color: #c8cad4; }
  .member-remove { background: none; border: none; color: #3a3f52; font-size: 16px; cursor: pointer; line-height: 1; padding: 0 2px; transition: color 0.15s; }
  .member-remove:hover { color: #e55; }
  .no-members { font-size: 12px; color: #3a3f52; padding: 4px 0; }
  .add-member-btn { background: none; border: 1px dashed #2a2f3e; border-radius: 6px; color: #4a4f5e; font-size: 12px; padding: 6px; cursor: pointer; text-align: center; transition: border-color 0.15s, color 0.15s; }
  .add-member-btn:hover { border-color: #6c8fff55; color: #6c8fff; }

  .settings-card { background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 28px 32px; max-width: 680px; }
  .settings-section-title { font-size: 16px; font-weight: 600; color: #f0f1f3; margin-bottom: 6px; }
  .settings-section-sub { font-size: 13px; color: #4a4f5e; margin-bottom: 20px; display: flex; align-items: center; }
  .settings-section-sub strong { color: #8a8fa8; }
  .settings-fields { display: flex; flex-direction: column; gap: 14px; }
  .form-row { display: flex; gap: 12px; align-items: flex-end; }
  .pw-success { background: #0d2e1f; border: 1px solid #1a5a3a; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #3ecf8e; margin-bottom: 6px; }
  .pw-error { background: #2a1010; border: 1px solid #5a2020; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #e55; margin-bottom: 6px; }
  .save-btn { background: #6c8fff; color: #fff; border: none; border-radius: 8px; padding: 10px 20px; font-size: 14px; font-weight: 600; cursor: pointer; align-self: flex-start; transition: background 0.15s; margin-top: 4px; }
  .save-btn:hover { background: #5a7aee; }
  .save-btn:disabled { opacity: 0.6; cursor: not-allowed; }

  .stats-row { display: flex; gap: 12px; margin-bottom: 24px; }
  .stat-card { flex: 1; background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 16px 18px; cursor: pointer; text-align: left; transition: border-color 0.15s, background 0.15s; display: flex; flex-direction: column; gap: 4px; }
  .stat-card:hover { border-color: #2a2f3e; background: #13161e; }
  .stat-card.active { border-color: #6c8fff55; background: #111b30; }
  .stat-num { font-size: 26px; font-weight: 600; color: #f0f1f3; line-height: 1; font-family: 'JetBrains Mono', monospace; }
  .stat-num.green { color: #3ecf8e; }
  .stat-num.muted { color: #4a4f5e; }
  .stat-num.amber { color: #f5a623; }
  .stat-label { font-size: 11px; color: #4a4f5e; text-transform: uppercase; letter-spacing: 0.06em; }

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
  .td-mono { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
  .td-sid { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #3a3f52; }
  .td-actions { white-space: nowrap; }
  .td-result { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  .action-group { display: flex; gap: 6px; }
  .action-btn { font-size: 11px; font-weight: 600; padding: 4px 10px; border-radius: 5px; border: none; cursor: pointer; letter-spacing: 0.03em; }
  .action-disable { background: #2a1010; color: #e55; border: 1px solid #5a2020; }
  .action-disable:hover { background: #3a1515; }
  .action-enable { background: #0d2e1f; color: #3ecf8e; border: 1px solid #1a5a3a; }
  .action-enable:hover { background: #0f3824; }
  .action-delete { background: #1a1010; color: #a33; border: 1px solid #3a1515; }
  .action-delete:hover { background: #2a1515; color: #e55; border-color: #5a2020; }
  .action-pending { display: inline-flex; align-items: center; gap: 6px; font-size: 11px; color: #4a4f5e; }
  .action-self { color: #2a2f3e; font-size: 13px; }

  .you-badge { font-size: 10px; background: #1a2240; color: #6c8fff; padding: 1px 6px; border-radius: 4px; margin-left: 6px; font-weight: 600; }

  .badge { display: inline-block; font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 4px; letter-spacing: 0.03em; }
  .badge-green  { background: #0d2e1f; color: #3ecf8e; }
  .badge-gray   { background: #1a1d25; color: #4a4f5e; }
  .badge-amber  { background: #2e1f08; color: #f5a623; }
  .badge-red    { background: #2a1010; color: #e55; }
  .badge-ghost  { background: transparent; color: #3a3f52; border: 1px solid #1e2028; }

  .modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .modal { background: #111318; border: 1px solid #1e2028; border-radius: 14px; padding: 32px; width: 420px; display: flex; flex-direction: column; gap: 16px; }
  .modal-title { font-size: 18px; font-weight: 600; color: #f0f1f3; }
  .modal-sub { font-size: 13px; color: #4a4f5e; margin-top: -8px; }
  .modal-actions { display: flex; gap: 10px; justify-content: flex-end; margin-top: 4px; }
  .modal-cancel { background: none; border: 1px solid #1e2028; border-radius: 8px; color: #6a7090; font-size: 13px; padding: 8px 16px; cursor: pointer; transition: border-color 0.15s, color 0.15s; }
  .modal-cancel:hover { border-color: #2a2f3e; color: #d0d3e0; }
  .checkbox-row { display: flex; align-items: center; gap: 10px; font-size: 13px; color: #8a8fa8; cursor: pointer; }
  .checkbox-row input { width: 15px; height: 15px; cursor: pointer; accent-color: #6c8fff; }

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