<script>
  import { page, machines, selectedMachine, machineTab, currentUser, currentRole, onlineCount, isAdmin, clearSession } from '../lib/stores.js';
  import { isOnline } from '../lib/api.js';

  function nav(p) { page.set(p); selectedMachine.set(null); }

  function selectMachine(m) {
    selectedMachine.set(m);
    machineTab.set('accounts');
    page.set('machines');
  }

  function logout() { clearSession(); }

  const navItems = [
    { id: 'overview',  label: 'Overview',    icon: '⊞' },
    { id: 'machines',  label: 'Machines',    icon: '⬡' },
    { id: 'audit',     label: 'Audit Log',   icon: '≡' },
  ];

  const adminItems = [
    { id: 'users',     label: 'Users',       icon: '◎' },
    { id: 'deploy',    label: 'Deploy Keys', icon: '⚿' },
  ];
</script>

<aside class="sidebar">
  <!-- Brand -->
  <div class="brand">
    <span class="brand-icon">⬡</span>
    <div class="brand-text">
      <span class="brand-name">Aurigon</span>
      <span class="brand-sub">Security</span>
    </div>
  </div>

  <nav class="nav">
    <!-- Main nav -->
    <div class="nav-section">
      <div class="nav-label">Main</div>
      {#each navItems as item}
        <button class="nav-item {$page === item.id && !$selectedMachine ? 'active' : ''}"
          on:click={() => nav(item.id)}>
          <span class="nav-icon">{item.icon}</span>
          <span>{item.label}</span>
          {#if item.id === 'machines' && $machines.length > 0}
            <span class="nav-badge">{$machines.length}</span>
          {/if}
        </button>
      {/each}
    </div>

    <!-- Machines list -->
    {#if $machines.length > 0}
      <div class="nav-section machines-section">
        <div class="nav-label">Machines</div>
        {#each $machines as machine}
          <button class="machine-item {$selectedMachine?.id === machine.id ? 'active' : ''}"
            on:click={() => selectMachine(machine)}>
            <span class="dot {isOnline(machine.last_seen) ? 'online' : 'offline'}"></span>
            <span class="hostname">{machine.hostname}</span>
          </button>
        {/each}
      </div>
    {/if}

    <!-- Admin nav -->
    {#if $isAdmin}
      <div class="nav-section">
        <div class="nav-label">Admin</div>
        {#each adminItems as item}
          <button class="nav-item {$page === item.id && !$selectedMachine ? 'active' : ''}"
            on:click={() => nav(item.id)}>
            <span class="nav-icon">{item.icon}</span>
            <span>{item.label}</span>
          </button>
        {/each}
      </div>
    {/if}
  </nav>

  <!-- Footer -->
  <div class="footer">
    <div class="online-pill">
      <span class="online-dot"></span>
      <span>{$onlineCount} of {$machines.length} online</span>
    </div>
    <button class="footer-item {$page === 'settings' && !$selectedMachine ? 'active' : ''}"
      on:click={() => nav('settings')}>
      <span class="nav-icon">⚙</span>
      <span>Settings</span>
    </button>
    <div class="user-row">
      <div class="user-info">
        <span class="user-name">{$currentUser}</span>
        <span class="user-role">{$currentRole}</span>
      </div>
      <button class="signout" on:click={logout}>Sign out</button>
    </div>
  </div>
</aside>

<style>
  .sidebar { width: 232px; min-height: 100vh; background: #0f1117; border-right: 1px solid #1a1d26; display: flex; flex-direction: column; flex-shrink: 0; }

  .brand { display: flex; align-items: center; gap: 10px; padding: 22px 20px; border-bottom: 1px solid #1a1d26; }
  .brand-icon { font-size: 22px; color: #6c8fff; line-height: 1; }
  .brand-text { display: flex; flex-direction: column; line-height: 1.1; }
  .brand-name { font-size: 15px; font-weight: 700; color: #f0f1f3; letter-spacing: -0.01em; }
  .brand-sub { font-size: 10px; color: #4a4f5e; text-transform: uppercase; letter-spacing: 0.08em; }

  .nav { flex: 1; padding: 16px 10px; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; }
  .nav-section { margin-bottom: 8px; }
  .nav-label { font-size: 10px; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; color: #3a3f52; padding: 0 8px; margin-bottom: 4px; margin-top: 8px; }

  .nav-item { display: flex; align-items: center; gap: 9px; width: 100%; padding: 8px 10px; border-radius: 7px; background: none; border: none; color: #6a7090; font-size: 13px; font-weight: 500; cursor: pointer; text-align: left; transition: all 0.12s; font-family: inherit; position: relative; }
  .nav-item:hover { background: #161921; color: #c8cad4; }
  .nav-item.active { background: #161f38; color: #6c8fff; }
  .nav-icon { font-size: 15px; width: 18px; text-align: center; flex-shrink: 0; }
  .nav-badge { margin-left: auto; background: #1a1d26; color: #4a4f5e; font-size: 10px; font-weight: 600; padding: 1px 6px; border-radius: 10px; }

  .machines-section { border-top: 1px solid #1a1d26; border-bottom: 1px solid #1a1d26; padding: 8px 0; margin: 4px 0; }
  .machine-item { display: flex; align-items: center; gap: 8px; width: 100%; padding: 6px 10px; border-radius: 6px; background: none; border: none; color: #5a6080; font-size: 12px; cursor: pointer; text-align: left; transition: all 0.12s; font-family: inherit; }
  .machine-item:hover { background: #161921; color: #c8cad4; }
  .machine-item.active { background: #161f38; color: #6c8fff; }
  .dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
  .dot.online { background: #3ecf8e; box-shadow: 0 0 5px #3ecf8e66; }
  .dot.offline { background: #2a2f3e; }
  .hostname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  .footer { padding: 14px 10px; border-top: 1px solid #1a1d26; display: flex; flex-direction: column; gap: 6px; }
  .online-pill { display: flex; align-items: center; gap: 7px; padding: 7px 10px; background: #0d1420; border: 1px solid #1a2030; border-radius: 7px; font-size: 11px; color: #4a4f5e; }
  .online-dot { width: 6px; height: 6px; border-radius: 50%; background: #3ecf8e; box-shadow: 0 0 5px #3ecf8e66; flex-shrink: 0; }
  .footer-item { display: flex; align-items: center; gap: 9px; width: 100%; padding: 7px 10px; border-radius: 7px; background: none; border: none; color: #6a7090; font-size: 13px; cursor: pointer; text-align: left; transition: all 0.12s; font-family: inherit; }
  .footer-item:hover { background: #161921; color: #c8cad4; }
  .footer-item.active { background: #161f38; color: #6c8fff; }
  .user-row { display: flex; align-items: center; justify-content: space-between; padding: 6px 8px; }
  .user-info { display: flex; flex-direction: column; gap: 1px; }
  .user-name { font-size: 12px; font-weight: 600; color: #8a8fa8; }
  .user-role { font-size: 10px; color: #3a3f52; text-transform: uppercase; letter-spacing: 0.06em; }
  .signout { background: none; border: 1px solid #1e2028; border-radius: 5px; color: #4a4f5e; font-size: 11px; padding: 3px 8px; cursor: pointer; transition: all 0.15s; font-family: inherit; }
  .signout:hover { color: #e55; border-color: #5a2020; }
</style>