<script>
  import { onMount, onDestroy } from 'svelte';
  import { selectedMachine, machineTab, token } from '../lib/stores.js';
  import { api, formatDate, formatDateTime, isOnline } from '../lib/api.js';
  import { showToast } from '../lib/toast.js';

  let accounts = [];
  let groups = [];
  let actions = [];
  let sessions = [];
  let loading = true;
  let error = null;
  let search = '';
  let filter = 'all';
  let actionStatus = {};
  let pollInterval;

  // Add account modal
  let showAddModal = false;
  let newUsername = '';
  let newPassword = '';
  let newIsAdmin = false;
  let addError = null;
  let addLoading = false;

  // Row action menu
  let openMenuFor = null;
  let menuPos = { top: 0, left: 0 };

  // Reset password modal
  let showResetModal = false;
  let resetTargetUsername = '';
  let resetPassword = '';
  let resetError = null;
  let resetLoading = false;

  // Rename modal
  let showRenameModal = false;
  let renameTargetUsername = '';
  let renameNewUsername = '';
  let renameError = null;
  let renameLoading = false;

  // Edit details modal
  let showDetailsModal = false;
  let detailsTargetUsername = '';
  let detailsFullName = '';
  let detailsDescription = '';
  let detailsError = null;
  let detailsLoading = false;

  // Expiration modal
  let showExpirationModal = false;
  let expirationTargetUsername = '';
  let expirationDate = '';
  let expirationNever = true;
  let expirationError = null;
  let expirationLoading = false;

  // Bulk selection (accounts tab)
  let selected = new Set();

  // Bulk reset password modal
  let showBulkResetModal = false;
  let bulkResetPassword = '';
  let bulkResetError = null;
  let bulkResetLoading = false;

  // Group management
  let showNewGroupModal = false;
  let newGroupName = '';
  let newGroupDescription = '';
  let newGroupError = null;
  let newGroupLoading = false;
  let addMemberInputs = {}; // { [groupName]: usernameBeingAdded }

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
      if (e.status === 401) { return; }
      error = e.message;
    } finally { loading = false; }
  }

  async function loadGroups() {
    if (!machine) return;
    try { groups = await api.getGroups($token, machine.id) || []; }
    catch {}
  }

  async function loadSessions() {
    if (!machine) return;
    try { sessions = await api.getSessions($token, machine.id) || []; }
    catch {}
  }

  const actionLabels = {
    disable_account: 'Disabling',
    enable_account: 'Enabling',
    set_admin: 'Granting admin privileges to',
    remove_admin: 'Removing admin privileges from',
    reset_password: 'Resetting password for',
    require_password_change: 'Requiring password change for',
    unlock_account: 'Unlocking',
    set_password_never_expires: 'Updating password expiration for',
    rename_account: 'Renaming',
    update_account_details: 'Updating details for',
    set_account_expiration: 'Updating expiration for',
    force_logoff: 'Forcing logoff for',
    delete_account: 'Deleting',
    add_to_group: 'Adding',
    remove_from_group: 'Removing',
    create_group: 'Creating group',
    delete_group: 'Deleting group',
  };

  async function triggerAction(type, username, params = {}) {
    actionStatus = { ...actionStatus, [username]: 'pending' };
    try {
      const result = await api.createAction($token, machine.id, type, username, params);
      if (result && result.duplicate) {
        showToast(
          `"${username}" already has a pending action (${(result.existing_type || '').replace(/_/g, ' ')}) — wait for it to finish first.`,
          { type: 'error', duration: 6000 }
        );
        return;
      }
      const label = actionLabels[type] || 'Applying action to';
      showToast(`${label} "${username}"…`, { type: 'info', duration: 3000 });
    } catch (e) {
      actionStatus = { ...actionStatus, [username]: null };
      showToast(`Failed: ${e.message}`, { type: 'error', duration: 6000 });
    }
  }

  // ── Add account ──────────────────────────────────────────────────────
  function openAddModal() {
    showAddModal = true;
    newUsername = ''; newPassword = ''; newIsAdmin = false; addError = null;
  }
  function closeAddModal() { showAddModal = false; }
  async function submitAddAccount() {
    addError = null;
    if (!newUsername.trim()) { addError = 'Username is required.'; return; }
    if (newPassword.length < 8) { addError = 'Password must be at least 8 characters.'; return; }
    addLoading = true;
    const uname = newUsername.trim();
    try {
      await api.createAction($token, machine.id, 'create_account', uname, {
        password: newPassword, is_admin: newIsAdmin ? 'true' : 'false',
      });
      showAddModal = false;
      // Insert a placeholder row immediately so there's something for the
      // "Pending…" state to attach to — otherwise it has nowhere to render
      // until the agent's next sync confirms the account actually exists.
      accounts = [...accounts, {
        username: uname, enabled: false, is_admin: newIsAdmin,
        sid: '', description: '', last_logon: '',
      }];
      actionStatus = { ...actionStatus, [uname]: 'pending' };
      showToast(`Creating account "${uname}"…`, { type: 'info', duration: 3000 });
    } catch (e) { addError = e.message; }
    finally { addLoading = false; }
  }

  // ── Row menu ─────────────────────────────────────────────────────────
  function toggleMenu(username, event) {
    if (openMenuFor === username) { openMenuFor = null; return; }
    const rect = event.currentTarget.getBoundingClientRect();
    const menuHeight = 360; // matches CSS max-height
    const menuWidth = 220;  // matches CSS min-width
    const spaceBelow = window.innerHeight - rect.bottom;
    const top = spaceBelow < menuHeight
      ? Math.max(8, rect.top - menuHeight - 4)
      : rect.bottom + 4;
    const left = Math.max(8, Math.min(rect.right - menuWidth, window.innerWidth - menuWidth - 8));
    menuPos = { top, left };
    openMenuFor = username;
  }
  function closeMenu() { openMenuFor = null; }

  async function deleteAccount(username) {
    if (!confirm(`Delete account "${username}" from ${machine.hostname}? This cannot be undone.`)) return;
    await triggerAction('delete_account', username);
    openMenuFor = null;
  }

  async function toggleAdmin(username, currentlyAdmin) {
    const verb = currentlyAdmin ? 'remove admin privileges from' : 'grant admin privileges to';
    if (!confirm(`Are you sure you want to ${verb} "${username}"?`)) return;
    await triggerAction(currentlyAdmin ? 'remove_admin' : 'set_admin', username);
    openMenuFor = null;
  }

  async function requirePasswordChange(username) {
    if (!confirm(`Require "${username}" to change their password at next logon?`)) return;
    await triggerAction('require_password_change', username);
    openMenuFor = null;
  }

  async function unlockAccount(username) {
    await triggerAction('unlock_account', username);
    openMenuFor = null;
  }

  async function setPasswordNeverExpires(username, neverExpires) {
    await triggerAction('set_password_never_expires', username, { never_expires: neverExpires ? 'true' : 'false' });
    openMenuFor = null;
  }

  async function forceLogoff(username) {
    if (!confirm(`Force log off "${username}" from ${machine.hostname}?`)) return;
    await triggerAction('force_logoff', username);
    openMenuFor = null;
  }

  // ── Reset password modal ────────────────────────────────────────────
  function openResetModal(username) {
    resetTargetUsername = username; resetPassword = ''; resetError = null;
    showResetModal = true; openMenuFor = null;
  }
  function closeResetModal() { showResetModal = false; }
  async function submitResetPassword() {
    resetError = null;
    if (resetPassword.length < 8) { resetError = 'Password must be at least 8 characters.'; return; }
    resetLoading = true;
    try {
      await api.createAction($token, machine.id, 'reset_password', resetTargetUsername, { password: resetPassword });
      showResetModal = false;
      actionStatus = { ...actionStatus, [resetTargetUsername]: 'pending' };
      showToast(`Resetting password for "${resetTargetUsername}"…`, { type: 'info', duration: 3000 });
    } catch (e) { resetError = e.message; }
    finally { resetLoading = false; }
  }

  // ── Rename modal ─────────────────────────────────────────────────────
  function openRenameModal(username) {
    renameTargetUsername = username; renameNewUsername = ''; renameError = null;
    showRenameModal = true; openMenuFor = null;
  }
  function closeRenameModal() { showRenameModal = false; }
  async function submitRename() {
    renameError = null;
    if (!renameNewUsername.trim()) { renameError = 'New username is required.'; return; }
    renameLoading = true;
    try {
      await api.createAction($token, machine.id, 'rename_account', renameTargetUsername, {
        new_username: renameNewUsername.trim(),
      });
      showRenameModal = false;
      actionStatus = { ...actionStatus, [renameTargetUsername]: 'pending' };
      showToast(`Renaming "${renameTargetUsername}"…`, { type: 'info', duration: 3000 });
    } catch (e) { renameError = e.message; }
    finally { renameLoading = false; }
  }

  // ── Edit details modal ──────────────────────────────────────────────
  function openDetailsModal(a) {
    detailsTargetUsername = a.username;
    detailsFullName = '';
    detailsDescription = a.description || '';
    detailsError = null;
    showDetailsModal = true; openMenuFor = null;
  }
  function closeDetailsModal() { showDetailsModal = false; }
  async function submitDetails() {
    detailsError = null;
    detailsLoading = true;
    try {
      await api.createAction($token, machine.id, 'update_account_details', detailsTargetUsername, {
        full_name: detailsFullName, description: detailsDescription,
      });
      showDetailsModal = false;
      actionStatus = { ...actionStatus, [detailsTargetUsername]: 'pending' };
      showToast(`Updating details for "${detailsTargetUsername}"…`, { type: 'info', duration: 3000 });
    } catch (e) { detailsError = e.message; }
    finally { detailsLoading = false; }
  }

  // ── Expiration modal ────────────────────────────────────────────────
  function openExpirationModal(username) {
    expirationTargetUsername = username; expirationDate = ''; expirationNever = true;
    expirationError = null; showExpirationModal = true; openMenuFor = null;
  }
  function closeExpirationModal() { showExpirationModal = false; }
  async function submitExpiration() {
    expirationError = null;
    if (!expirationNever && !expirationDate) { expirationError = 'Pick a date or check "Never expires".'; return; }
    expirationLoading = true;
    try {
      await api.createAction($token, machine.id, 'set_account_expiration', expirationTargetUsername, {
        expires: expirationNever ? 'never' : expirationDate,
      });
      showExpirationModal = false;
      actionStatus = { ...actionStatus, [expirationTargetUsername]: 'pending' };
      showToast(`Updating expiration for "${expirationTargetUsername}"…`, { type: 'info', duration: 3000 });
    } catch (e) { expirationError = e.message; }
    finally { expirationLoading = false; }
  }

  // ── Bulk selection ───────────────────────────────────────────────────
  function toggleSelect(username) {
    const next = new Set(selected);
    if (next.has(username)) next.delete(username); else next.add(username);
    selected = next;
  }
  function toggleSelectAll() {
    if (selected.size === filtered.length) { selected = new Set(); }
    else { selected = new Set(filtered.map(a => a.username)); }
  }
  function clearSelection() { selected = new Set(); }

  async function bulkTrigger(type, params = {}) {
    const targets = [...selected].map(username => ({ machine_id: machine.id, username }));
    if (targets.length === 0) return;
    try {
      const result = await api.bulkCreateAction($token, targets, type, params);
      for (const t of targets) actionStatus[t.username] = 'pending';
      actionStatus = { ...actionStatus };
      clearSelection();
      showToast(
        `Queued for ${result.created} account(s)${result.skipped ? `, ${result.skipped} already pending` : ''}…`,
        { type: 'info', duration: 3000 }
      );
      if (result.failed > 0) {
        showToast(`${result.failed} failed to queue`, { type: 'error', duration: 6000 });
      }
    } catch (e) {
      showToast('Bulk action failed: ' + e.message, { type: 'error', duration: 6000 });
    }
  }

  function bulkDisable() {
    if (!confirm(`Disable ${selected.size} account(s)?`)) return;
    bulkTrigger('disable_account');
  }
  function bulkEnable() {
    if (!confirm(`Enable ${selected.size} account(s)?`)) return;
    bulkTrigger('enable_account');
  }
  function bulkDelete() {
    if (!confirm(`Delete ${selected.size} account(s)? This cannot be undone.`)) return;
    bulkTrigger('delete_account');
  }
  function openBulkReset() {
    bulkResetPassword = ''; bulkResetError = null; showBulkResetModal = true;
  }
  function closeBulkReset() { showBulkResetModal = false; }
  async function submitBulkReset() {
    bulkResetError = null;
    if (bulkResetPassword.length < 8) { bulkResetError = 'Password must be at least 8 characters.'; return; }
    bulkResetLoading = true;
    try {
      await bulkTrigger('reset_password', { password: bulkResetPassword });
      showBulkResetModal = false;
    } catch (e) { bulkResetError = e.message; }
    finally { bulkResetLoading = false; }
  }

  // ── Group management ────────────────────────────────────────────────
  function openNewGroupModal() {
    newGroupName = ''; newGroupDescription = ''; newGroupError = null; showNewGroupModal = true;
  }
  function closeNewGroupModal() { showNewGroupModal = false; }
  async function submitNewGroup() {
    newGroupError = null;
    if (!newGroupName.trim()) { newGroupError = 'Group name is required.'; return; }
    newGroupLoading = true;
    try {
      // Group actions reuse the "username" field to carry the group name
      await api.createAction($token, machine.id, 'create_group', newGroupName.trim(), {
        description: newGroupDescription,
      });
      showNewGroupModal = false;
      showToast(`Creating group "${newGroupName.trim()}"…`, { type: 'info', duration: 3000 });
    } catch (e) { newGroupError = e.message; }
    finally { newGroupLoading = false; }
  }

  async function deleteGroup(groupName) {
    if (!confirm(`Delete group "${groupName}" from ${machine.hostname}?`)) return;
    await triggerAction('delete_group', groupName);
  }

  async function addMember(groupName) {
    const username = (addMemberInputs[groupName] || '').trim();
    if (!username) return;
    await triggerAction('add_to_group', username, { group: groupName });
    addMemberInputs[groupName] = '';
    addMemberInputs = { ...addMemberInputs };
  }

  async function removeMember(groupName, username) {
    if (!confirm(`Remove "${username}" from "${groupName}"?`)) return;
    await triggerAction('remove_from_group', username, { group: groupName });
  }

  // ── Polling ──────────────────────────────────────────────────────────
  // Tracks action IDs we've personally observed as "pending" so we only
  // toast failures we actually watched happen — not the entire failure
  // history re-announced every time this page reloads or remounts.
  let previousPendingIds = new Set();

  function startPolling() {
    clearInterval(pollInterval);
    pollInterval = setInterval(async () => {
      try {
        const acts = await api.getActionStatus($token, machine.id);
        const newStatus = {};
        const currentPendingIds = new Set();
        for (const a of acts) {
          if (a.status === 'pending') {
            newStatus[a.username] = 'pending';
            currentPendingIds.add(a.id);
          }
          // Only toast a failure if we watched it transition from pending
          // (last poll) to failed (this poll) — never for actions that
          // were already failed before we started watching.
          if (a.status === 'failed' && previousPendingIds.has(a.id)) {
            showToast(`Failed — ${a.type.replace(/_/g, ' ')} on "${a.username}": ${a.result || 'unknown error'}`, {
              type: 'error', duration: 8000,
            });
          }
        }
        previousPendingIds = currentPendingIds;
        const wasPending = Object.values(actionStatus).some(s => s === 'pending');
        const stillPending = Object.values(newStatus).some(s => s === 'pending');
        actionStatus = newStatus;
        if (wasPending && !stillPending) {
          await load();
          await loadGroups();
        } else {
          accounts = await api.getAccounts($token, machine.id) || accounts;
        }
        if ($machineTab === 'sessions') await loadSessions();
      } catch {}
    }, 8000);
  }

  onMount(() => { load(); startPolling(); });
  onDestroy(() => clearInterval(pollInterval));

  $: if (machine) { load(); startPolling(); }
  $: if ($machineTab === 'groups') loadGroups();
  $: if ($machineTab === 'sessions') loadSessions();

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

<svelte:window on:click={closeMenu}/>

{#if machine}
<div class="page">
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

  <div class="tabs">
    <button class="tab {$machineTab === 'accounts' ? 'active' : ''}" on:click={() => machineTab.set('accounts')}>Accounts</button>
    <button class="tab {$machineTab === 'groups' ? 'active' : ''}" on:click={() => machineTab.set('groups')}>Groups</button>
    <button class="tab {$machineTab === 'sessions' ? 'active' : ''}" on:click={() => machineTab.set('sessions')}>
      Sessions
      {#if sessions.length > 0}<span class="tab-badge">{sessions.length}</span>{/if}
    </button>
    <button class="tab {$machineTab === 'actions' ? 'active' : ''}" on:click={() => machineTab.set('actions')}>
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

    {#if selected.size > 0}
      <div class="bulk-bar">
        <span class="bulk-count">{selected.size} selected</span>
        <button class="bulk-btn" on:click={bulkEnable}>Enable</button>
        <button class="bulk-btn" on:click={bulkDisable}>Disable</button>
        <button class="bulk-btn" on:click={openBulkReset}>Reset Password</button>
        <button class="bulk-btn danger" on:click={bulkDelete}>Delete</button>
        <button class="bulk-clear" on:click={clearSelection}>Clear</button>
      </div>
    {/if}

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
            <tr>
              <th class="checkbox-col">
                <input type="checkbox" checked={selected.size === filtered.length && filtered.length > 0} on:change={toggleSelectAll}/>
              </th>
              <th>Username</th><th>Status</th><th>Role</th><th>Last logon</th><th>Description</th><th class="actions-col">Actions</th>
            </tr>
          </thead>
          <tbody>
            {#each filtered as a}
              <tr>
                <td class="checkbox-col">
                  <input type="checkbox" checked={selected.has(a.username)} on:change={() => toggleSelect(a.username)}/>
                </td>
                <td class="bold">{a.username}</td>
                <td>
                  {#if a.enabled}<span class="badge green">Enabled</span>{:else}<span class="badge gray">Disabled</span>{/if}
                </td>
                <td>
                  {#if a.is_admin}<span class="badge amber">Admin</span>{:else}<span class="badge ghost">User</span>{/if}
                </td>
                <td class="muted">{formatDate(a.last_logon)}</td>
                <td class="muted">{a.description || '—'}</td>
                <td class="actions-col">
                  {#if actionStatus[a.username] === 'pending'}
                    <span class="pending"><span class="inline-progress"><span class="inline-progress-bar"></span></span> Applying…</span>
                  {:else}
                    <div class="menu-wrap">
                      <button class="menu-trigger" on:click|stopPropagation={(e) => toggleMenu(a.username, e)}>Actions ▾</button>
                      {#if openMenuFor === a.username}
                        <div class="menu" style="top:{menuPos.top}px; left:{menuPos.left}px;" on:click|stopPropagation>
                          {#if a.enabled}
                            <button class="menu-item" on:click={() => { triggerAction('disable_account', a.username); openMenuFor = null; }}>Disable account</button>
                          {:else}
                            <button class="menu-item" on:click={() => { triggerAction('enable_account', a.username); openMenuFor = null; }}>Enable account</button>
                          {/if}
                          <button class="menu-item" on:click={() => toggleAdmin(a.username, a.is_admin)}>
                            {a.is_admin ? 'Remove admin privileges' : 'Make admin'}
                          </button>
                          <div class="menu-divider"></div>
                          <button class="menu-item" on:click={() => openResetModal(a.username)}>Reset password</button>
                          <button class="menu-item" on:click={() => requirePasswordChange(a.username)}>Require password change</button>
                          <button class="menu-item" on:click={() => unlockAccount(a.username)}>Unlock account</button>
                          <button class="menu-item" on:click={() => setPasswordNeverExpires(a.username, true)}>Password never expires</button>
                          <button class="menu-item" on:click={() => setPasswordNeverExpires(a.username, false)}>Password expires normally</button>
                          <div class="menu-divider"></div>
                          <button class="menu-item" on:click={() => openRenameModal(a.username)}>Rename account</button>
                          <button class="menu-item" on:click={() => openDetailsModal(a)}>Edit description</button>
                          <button class="menu-item" on:click={() => openExpirationModal(a.username)}>Set account expiration</button>
                          <button class="menu-item" on:click={() => forceLogoff(a.username)}>Force log off</button>
                          <div class="menu-divider"></div>
                          <button class="menu-item danger" on:click={() => deleteAccount(a.username)}>Delete account</button>
                        </div>
                      {/if}
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
    <div class="tab-controls">
      <div></div>
      <div class="tab-controls-right">
        <button class="add-btn" on:click={openNewGroupModal}>+ New Group</button>
      </div>
    </div>

    {#if groups.length === 0}
      <div class="state"><div class="spinner"></div></div>
    {:else}
      <div class="groups-grid">
        {#each groups as group}
          <div class="group-card">
            <div class="group-card-head">
              <div class="group-name">{group.name}</div>
              <button class="group-delete" on:click={() => deleteGroup(group.name)} title="Delete group">×</button>
            </div>
            {#if group.description}<div class="group-desc">{group.description}</div>{/if}
            <div class="group-members">
              {#each (group.members || []) as member}
                <span class="member">
                  {member}
                  <button class="member-remove" on:click={() => removeMember(group.name, member)} title="Remove">×</button>
                </span>
              {/each}
              {#if !group.members || group.members.length === 0}
                <span class="no-members">No members</span>
              {/if}
            </div>
            <div class="add-member-row">
              <input type="text" placeholder="username" bind:value={addMemberInputs[group.name]}
                on:keydown={(e) => e.key === 'Enter' && addMember(group.name)}/>
              <button on:click={() => addMember(group.name)}>Add</button>
            </div>
          </div>
        {/each}
      </div>
    {/if}

  <!-- ── Sessions tab ── -->
  {:else if $machineTab === 'sessions'}
    {#if sessions.length === 0}
      <div class="state"><p>No active sessions detected on this machine.</p></div>
    {:else}
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>Username</th><th>Session</th><th>State</th><th>Idle</th><th>Logon Time</th><th class="actions-col"></th></tr>
          </thead>
          <tbody>
            {#each sessions as s}
              <tr>
                <td class="bold">{s.username}</td>
                <td class="muted">{s.session_name || '—'}</td>
                <td><span class="badge {s.state === 'Active' ? 'green' : 'gray'}">{s.state || '—'}</span></td>
                <td class="muted">{s.idle_time || '—'}</td>
                <td class="muted">{s.logon_time || '—'}</td>
                <td class="actions-col">
                  {#if actionStatus[s.username] === 'pending'}
                    <span class="pending"><span class="inline-progress"><span class="inline-progress-bar"></span></span> Applying…</span>
                  {:else}
                    <button class="menu-item danger inline-btn" on:click={() => forceLogoff(s.username)}>Force log off</button>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
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
                  {#if a.status === 'completed'}<span class="badge green">Completed</span>
                  {:else if a.status === 'pending'}<span class="badge amber">Pending</span>
                  {:else}<span class="badge red">Failed</span>{/if}
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
        {#if addError}<div class="modal-error">{addError}</div>{/if}
        <div class="modal-fields">
          <div class="field"><label for="new-username">Username</label>
            <input id="new-username" type="text" placeholder="jsmith" bind:value={newUsername}/></div>
          <div class="field"><label for="new-password">Password</label>
            <input id="new-password" type="password" placeholder="Min 8 characters" bind:value={newPassword}/></div>
          <label class="checkbox-row"><input type="checkbox" bind:checked={newIsAdmin}/><span>Grant local administrator privileges</span></label>
        </div>
        <div class="modal-actions">
          <button class="modal-cancel" on:click={closeAddModal}>Cancel</button>
          <button class="modal-submit" on:click={submitAddAccount} disabled={addLoading}>{addLoading ? 'Creating…' : 'Create Account'}</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ── Reset Password Modal ── -->
  {#if showResetModal}
    <div class="modal-overlay" on:click={closeResetModal}>
      <div class="modal" on:click|stopPropagation>
        <div class="modal-title">Reset password</div>
        <p class="modal-sub">Sets a new password for <strong>{resetTargetUsername}</strong> on {machine.hostname}.</p>
        {#if resetError}<div class="modal-error">{resetError}</div>{/if}
        <div class="modal-fields">
          <div class="field"><label for="reset-password">New password</label>
            <input id="reset-password" type="password" placeholder="Min 8 characters" bind:value={resetPassword}/></div>
        </div>
        <div class="modal-actions">
          <button class="modal-cancel" on:click={closeResetModal}>Cancel</button>
          <button class="modal-submit" on:click={submitResetPassword} disabled={resetLoading}>{resetLoading ? 'Resetting…' : 'Reset Password'}</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ── Rename Modal ── -->
  {#if showRenameModal}
    <div class="modal-overlay" on:click={closeRenameModal}>
      <div class="modal" on:click|stopPropagation>
        <div class="modal-title">Rename account</div>
        <p class="modal-sub">Renames <strong>{renameTargetUsername}</strong> on {machine.hostname}.</p>
        {#if renameError}<div class="modal-error">{renameError}</div>{/if}
        <div class="modal-fields">
          <div class="field"><label for="rename-new">New username</label>
            <input id="rename-new" type="text" placeholder="new.username" bind:value={renameNewUsername}/></div>
        </div>
        <div class="modal-actions">
          <button class="modal-cancel" on:click={closeRenameModal}>Cancel</button>
          <button class="modal-submit" on:click={submitRename} disabled={renameLoading}>{renameLoading ? 'Renaming…' : 'Rename'}</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ── Edit Details Modal ── -->
  {#if showDetailsModal}
    <div class="modal-overlay" on:click={closeDetailsModal}>
      <div class="modal" on:click|stopPropagation>
        <div class="modal-title">Edit account details</div>
        <p class="modal-sub">Updates full name and description for <strong>{detailsTargetUsername}</strong>.</p>
        {#if detailsError}<div class="modal-error">{detailsError}</div>{/if}
        <div class="modal-fields">
          <div class="field"><label for="details-fullname">Full name</label>
            <input id="details-fullname" type="text" placeholder="Jane Smith" bind:value={detailsFullName}/></div>
          <div class="field"><label for="details-desc">Description</label>
            <input id="details-desc" type="text" placeholder="e.g. Marketing team" bind:value={detailsDescription}/></div>
        </div>
        <div class="modal-actions">
          <button class="modal-cancel" on:click={closeDetailsModal}>Cancel</button>
          <button class="modal-submit" on:click={submitDetails} disabled={detailsLoading}>{detailsLoading ? 'Saving…' : 'Save'}</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ── Expiration Modal ── -->
  {#if showExpirationModal}
    <div class="modal-overlay" on:click={closeExpirationModal}>
      <div class="modal" on:click|stopPropagation>
        <div class="modal-title">Set account expiration</div>
        <p class="modal-sub">Controls when <strong>{expirationTargetUsername}</strong> automatically becomes disabled.</p>
        {#if expirationError}<div class="modal-error">{expirationError}</div>{/if}
        <div class="modal-fields">
          <label class="checkbox-row"><input type="checkbox" bind:checked={expirationNever}/><span>Never expires</span></label>
          {#if !expirationNever}
            <div class="field"><label for="expiration-date">Expiration date</label>
              <input id="expiration-date" type="date" bind:value={expirationDate}/></div>
          {/if}
        </div>
        <div class="modal-actions">
          <button class="modal-cancel" on:click={closeExpirationModal}>Cancel</button>
          <button class="modal-submit" on:click={submitExpiration} disabled={expirationLoading}>{expirationLoading ? 'Saving…' : 'Save'}</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ── Bulk Reset Password Modal ── -->
  {#if showBulkResetModal}
    <div class="modal-overlay" on:click={closeBulkReset}>
      <div class="modal" on:click|stopPropagation>
        <div class="modal-title">Reset password for {selected.size} account(s)</div>
        <p class="modal-sub">All selected accounts will be set to this same password.</p>
        {#if bulkResetError}<div class="modal-error">{bulkResetError}</div>{/if}
        <div class="modal-fields">
          <div class="field"><label for="bulk-reset-password">New password</label>
            <input id="bulk-reset-password" type="password" placeholder="Min 8 characters" bind:value={bulkResetPassword}/></div>
        </div>
        <div class="modal-actions">
          <button class="modal-cancel" on:click={closeBulkReset}>Cancel</button>
          <button class="modal-submit" on:click={submitBulkReset} disabled={bulkResetLoading}>{bulkResetLoading ? 'Resetting…' : 'Reset All'}</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ── New Group Modal ── -->
  {#if showNewGroupModal}
    <div class="modal-overlay" on:click={closeNewGroupModal}>
      <div class="modal" on:click|stopPropagation>
        <div class="modal-title">Create local group</div>
        {#if newGroupError}<div class="modal-error">{newGroupError}</div>{/if}
        <div class="modal-fields">
          <div class="field"><label for="new-group-name">Group name</label>
            <input id="new-group-name" type="text" placeholder="Finance Team" bind:value={newGroupName}/></div>
          <div class="field"><label for="new-group-desc">Description</label>
            <input id="new-group-desc" type="text" placeholder="Optional" bind:value={newGroupDescription}/></div>
        </div>
        <div class="modal-actions">
          <button class="modal-cancel" on:click={closeNewGroupModal}>Cancel</button>
          <button class="modal-submit" on:click={submitNewGroup} disabled={newGroupLoading}>{newGroupLoading ? 'Creating…' : 'Create Group'}</button>
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

  .tab-controls { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; gap: 16px; flex-wrap: wrap; }
  .tab-controls-right { display: flex; align-items: center; gap: 10px; }
  .add-btn { background: linear-gradient(135deg, #7d9cff, #6c8fff); color: #fff; border: none; border-radius: 8px; padding: 9px 16px; font-size: 13px; font-weight: 600; cursor: pointer; font-family: inherit; transition: all 0.15s; white-space: nowrap; }
  .add-btn:hover { transform: translateY(-1px); box-shadow: 0 4px 14px rgba(108,143,255,.35); }
  .stats-row { display: flex; gap: 10px; }
  .stat { display: flex; flex-direction: column; gap: 2px; background: #111318; border: 1px solid #1e2028; border-radius: 8px; padding: 12px 16px; cursor: pointer; min-width: 80px; text-align: left; font-family: inherit; transition: all 0.12s; }
  .stat:hover { border-color: #2a2f3e; }
  .stat.active { border-color: #6c8fff44; background: #111b30; }
  .n { font-size: 20px; font-weight: 700; color: #f0f1f3; font-family: 'JetBrains Mono', monospace; line-height: 1; }
  .n.green { color: #3ecf8e; } .n.muted { color: #4a4f5e; } .n.amber { color: #f5a623; }
  .l { font-size: 10px; color: #4a4f5e; text-transform: uppercase; letter-spacing: 0.06em; }
  .search { background: #111318; border: 1px solid #1e2028; border-radius: 8px; color: #d0d3e0; font-size: 13px; padding: 8px 14px; width: 240px; outline: none; transition: border-color 0.15s; font-family: inherit; }
  .search:focus { border-color: #6c8fff55; }
  .search::placeholder { color: #3a3f52; }

  .bulk-bar { display: flex; align-items: center; gap: 8px; background: #111b30; border: 1px solid #6c8fff44; border-radius: 8px; padding: 10px 14px; margin-bottom: 14px; }
  .bulk-count { font-size: 12px; font-weight: 600; color: #8a9cd6; margin-right: 6px; }
  .bulk-btn { background: #1a1d25; color: #c8cad4; border: 1px solid #262b38; border-radius: 6px; padding: 6px 12px; font-size: 12px; font-weight: 600; cursor: pointer; font-family: inherit; transition: all 0.15s; }
  .bulk-btn:hover { background: #22262f; }
  .bulk-btn.danger { color: #e55; }
  .bulk-btn.danger:hover { background: #2a1010; }
  .bulk-clear { background: none; border: none; color: #4a4f5e; font-size: 12px; cursor: pointer; margin-left: auto; font-family: inherit; }
  .bulk-clear:hover { color: #8a8fa8; }

  .table-wrap { background: #111318; border: 1px solid #1e2028; border-radius: 10px; overflow: hidden; }
  table { width: 100%; border-collapse: collapse; }
  thead tr { border-bottom: 1px solid #1e2028; }  th { text-align: left; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.07em; color: #4a4f5e; padding: 12px 16px; }
  th.checkbox-col, td.checkbox-col { width: 34px; padding-left: 16px; padding-right: 0; }
  th.actions-col, td.actions-col { width: 190px; white-space: nowrap; text-align: right; }
  tbody tr { border-bottom: 1px solid #171921; transition: background 0.1s; }
  tbody tr:last-child { border-bottom: none; }
  tbody tr:hover { background: #13161e; }
  td { padding: 12px 16px; font-size: 13px; color: #c8cad4; vertical-align: middle; }
  .bold { font-weight: 600; color: #e2e4e9; }
  .muted { color: #4a4f5e; font-size: 12px; }
  input[type="checkbox"] { width: 14px; height: 14px; accent-color: #6c8fff; cursor: pointer; }

  .badge { display: inline-block; font-size: 10px; font-weight: 700; padding: 2px 7px; border-radius: 4px; letter-spacing: 0.04em; }
  .badge.green  { background: #0d2e1f; color: #3ecf8e; }
  .badge.gray   { background: #1a1d25; color: #4a4f5e; }
  .badge.amber  { background: #2e1f08; color: #f5a623; }
  .badge.red    { background: #2a1010; color: #e55; }
  .badge.ghost  { background: transparent; color: #3a3f52; border: 1px solid #1e2028; }

  .menu-wrap { position: relative; display: inline-block; }
  .menu-trigger { background: #1a1d25; color: #8a8fa8; border: 1px solid #262b38; border-radius: 6px; padding: 5px 12px; font-size: 12px; font-weight: 600; cursor: pointer; font-family: inherit; transition: all 0.15s; }
  .menu-trigger:hover { background: #22262f; color: #c8cad4; border-color: #333a4a; }
  .menu { position: fixed; min-width: 220px; max-height: 360px; overflow-y: auto; background: #16181f; border: 1px solid #262b38; border-radius: 8px; padding: 6px; box-shadow: 0 12px 30px rgba(0,0,0,.45); z-index: 120; }
  .menu-item { display: block; width: 100%; text-align: left; background: none; border: none; color: #c8cad4; font-size: 13px; padding: 8px 10px; border-radius: 6px; cursor: pointer; font-family: inherit; transition: background 0.12s; }
  .menu-item:hover { background: #1e2230; }
  .menu-item.danger { color: #e55; }
  .menu-item.danger:hover { background: #2a1010; }
  .menu-item.inline-btn { display: inline-block; width: auto; padding: 4px 10px; font-size: 11px; border: 1px solid #5a2020; }
  .menu-divider { height: 1px; background: #262b38; margin: 5px 2px; }
  .pending { display: inline-flex; align-items: center; gap: 8px; font-size: 11px; color: #4a4f5e; }
  .inline-progress { display: inline-block; width: 70px; height: 4px; background: #1e2028; border-radius: 2px; overflow: hidden; position: relative; }
  .inline-progress-bar {
    position: absolute; top: 0; left: -40%; height: 100%; width: 40%;
    background: linear-gradient(90deg, #6c8fff88, #6c8fff);
    border-radius: 2px;
    animation: indeterminate 1.1s ease-in-out infinite;
  }
  @keyframes indeterminate {
    0%   { left: -40%; }
    100% { left: 100%; }
  }
  .self { color: #2a2f3e; }
  .action-type { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #6a7090; background: #1a1d26; padding: 2px 6px; border-radius: 4px; }

  .groups-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 12px; }
  .group-card { background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 18px 20px; }
  .group-card-head { display: flex; align-items: center; justify-content: space-between; }
  .group-name { font-size: 14px; font-weight: 600; color: #e2e4e9; margin-bottom: 4px; }
  .group-delete { background: none; border: none; color: #3a3f52; font-size: 16px; cursor: pointer; line-height: 1; padding: 0 4px; }
  .group-delete:hover { color: #e55; }
  .group-desc { font-size: 12px; color: #4a4f5e; margin-bottom: 10px; }
  .group-members { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 10px; }
  .member { display: inline-flex; align-items: center; gap: 5px; font-size: 11px; background: #1a1d26; color: #6a7090; border-radius: 4px; padding: 2px 4px 2px 8px; font-family: 'JetBrains Mono', monospace; }
  .member-remove { background: none; border: none; color: #4a4f5e; cursor: pointer; font-size: 12px; padding: 0 3px; line-height: 1; }
  .member-remove:hover { color: #e55; }
  .no-members { font-size: 11px; color: #3a3f52; font-style: italic; }
  .add-member-row { display: flex; gap: 6px; margin-top: 14px; }
  .add-member-row input { flex: 1; background: #0d0f12; border: 1px solid #1e2028; border-radius: 6px; color: #d0d3e0; font-size: 12px; padding: 6px 10px; outline: none; font-family: inherit; }
  .add-member-row input:focus { border-color: #6c8fff55; }
  .add-member-row button { background: #1a2240; color: #6c8fff; border: 1px solid #6c8fff44; border-radius: 6px; padding: 6px 12px; font-size: 12px; font-weight: 600; cursor: pointer; font-family: inherit; }
  .add-member-row button:hover { background: #24306a; }

  .state { display: flex; align-items: center; justify-content: center; padding: 60px; color: #4a4f5e; font-size: 13px; }
  .state.error { color: #e55; }
  .spinner { width: 22px; height: 22px; border: 2px solid #1e2028; border-top-color: #6c8fff; border-radius: 50%; animation: spin 0.7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }

  .modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,.6); display: flex; align-items: center; justify-content: center; z-index: 100; backdrop-filter: blur(2px); }
  .modal { width: 400px; background: #14161c; border: 1px solid #262b38; border-radius: 12px; padding: 26px; box-shadow: 0 20px 60px rgba(0,0,0,.5); max-height: 85vh; overflow-y: auto; }
  .modal-title { font-size: 16px; font-weight: 700; color: #f0f1f3; margin-bottom: 4px; }
  .modal-sub { font-size: 12px; color: #4a4f5e; margin-bottom: 18px; line-height: 1.5; }
  .modal-sub strong { color: #8a8fa8; }
  .modal-error { background: #2a1010; border: 1px solid #5a2020; border-radius: 7px; padding: 9px 12px; font-size: 12px; color: #e55; margin-bottom: 14px; }
  .modal-fields { display: flex; flex-direction: column; gap: 14px; margin-bottom: 20px; }
  .modal-fields .field { display: flex; flex-direction: column; gap: 5px; }
  .modal-fields label { font-size: 11px; font-weight: 600; color: #6a7090; text-transform: uppercase; letter-spacing: 0.06em; }
  .modal-fields input[type="text"], .modal-fields input[type="password"], .modal-fields input[type="date"] { background: #0d0f12; border: 1px solid #1e2028; border-radius: 7px; color: #d0d3e0; font-size: 13px; padding: 9px 12px; outline: none; font-family: inherit; transition: border-color 0.15s; }
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