<script>
  import { onMount } from 'svelte';
  import { token, page, selectedMachine, machines, restoreSession } from './lib/stores.js';
  import { api } from './lib/api.js';

  import Login       from './components/Login.svelte';
  import Sidebar     from './components/Sidebar.svelte';
  import Overview    from './pages/Overview.svelte';
  import MachineDetail from './pages/MachineDetail.svelte';
  import AuditLog    from './pages/AuditLog.svelte';
  import Users       from './pages/Users.svelte';
  import DownloadAgent from './pages/DownloadAgent.svelte';
  import Settings    from './pages/Settings.svelte';
  import ToastHost   from './components/ToastHost.svelte';

  let ready = false;

  onMount(async () => {
    const restored = restoreSession();
    if (restored) await loadMachines();
    ready = true;
  });

  async function onLogin() {
    await loadMachines();
    page.set('overview');
  }

  async function loadMachines() {
    try {
      const data = await api.getMachines($token);
      machines.set(data || []);
    } catch (e) {
      if (e.status === 401) { token.set(null); }
    }
  }

  // Refresh machine list every 30s
  let machineTimer;
  $: if ($token) {
    clearInterval(machineTimer);
    machineTimer = setInterval(loadMachines, 30000);
  }

  $: isLoggedIn = !!$token;
</script>

{#if !ready}
  <div class="loading"></div>
{:else if !isLoggedIn}
  <Login onLogin={onLogin}/>
{:else}
  <div class="shell">
    <Sidebar/>
    <main class="main">
      {#if $selectedMachine}
        <MachineDetail/>
      {:else if $page === 'overview'}
        <Overview/>
      {:else if $page === 'machines'}
        <Overview/>
      {:else if $page === 'audit'}
        <AuditLog/>
      {:else if $page === 'users'}
        <Users/>
      {:else if $page === 'deploy'}
        <DownloadAgent/>
      {:else if $page === 'settings'}
        <Settings/>
      {/if}
    </main>
  </div>
{/if}

<ToastHost/>

<style>
  :global(*, *::before, *::after) { box-sizing: border-box; margin: 0; padding: 0; }
  :global(body) {
    background: #0d0f12;
    color: #e2e4e9;
    font-family: 'Inter', system-ui, sans-serif;
    font-size: 14px;
    line-height: 1.5;
    -webkit-font-smoothing: antialiased;
  }
  :global(a) { text-decoration: none; color: inherit; }

  .loading { min-height: 100vh; background: #0d0f12; }
  .shell { display: flex; min-height: 100vh; }
  .main { flex: 1; min-width: 0; overflow-y: auto; }
</style>