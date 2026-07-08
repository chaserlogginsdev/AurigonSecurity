<script>
  import { machines, selectedMachine, page, machineTab } from '../lib/stores.js';
  import { isOnline, formatDateTime } from '../lib/api.js';

  function openMachine(m) {
    selectedMachine.set(m);
    machineTab.set('accounts');
    page.set('machines');
  }

  $: online  = $machines.filter(m => isOnline(m.last_seen));
  $: offline = $machines.filter(m => !isOnline(m.last_seen));
</script>

<div class="page">
  <div class="topbar">
    <div>
      <h1>Overview</h1>
      <p class="sub">Fleet health at a glance</p>
    </div>
  </div>

  <!-- Stats -->
  <div class="stats">
    <div class="stat">
      <div class="stat-num">{$machines.length}</div>
      <div class="stat-label">Total machines</div>
    </div>
    <div class="stat green">
      <div class="stat-num">{online.length}</div>
      <div class="stat-label">Online</div>
    </div>
    <div class="stat muted">
      <div class="stat-num">{offline.length}</div>
      <div class="stat-label">Offline</div>
    </div>
  </div>

  {#if $machines.length === 0}
    <div class="empty">
      <div class="empty-icon">⬡</div>
      <div class="empty-title">No machines registered yet</div>
      <div class="empty-sub">Deploy the agent on a machine using a deploy key to get started.</div>
    </div>
  {:else}
    <!-- Online machines -->
    {#if online.length > 0}
      <div class="section">
        <div class="section-title">
          <span class="dot online"></span> Online
        </div>
        <div class="machine-grid">
          {#each online as machine}
            <button class="machine-card" on:click={() => openMachine(machine)}>
              <div class="card-top">
                <span class="card-hostname">{machine.hostname}</span>
                <span class="badge green">Online</span>
              </div>
              <div class="card-id">{machine.id}</div>
              <div class="card-seen">Last seen {formatDateTime(machine.last_seen)}</div>
            </button>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Offline machines -->
    {#if offline.length > 0}
      <div class="section">
        <div class="section-title">
          <span class="dot offline"></span> Offline
        </div>
        <div class="machine-grid">
          {#each offline as machine}
            <button class="machine-card offline" on:click={() => openMachine(machine)}>
              <div class="card-top">
                <span class="card-hostname">{machine.hostname}</span>
                <span class="badge gray">Offline</span>
              </div>
              <div class="card-id">{machine.id}</div>
              <div class="card-seen">Last seen {formatDateTime(machine.last_seen)}</div>
            </button>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .page { padding: 36px 40px; max-width: 1100px; }
  .topbar { margin-bottom: 32px; }
  h1 { font-size: 22px; font-weight: 700; color: #f0f1f3; letter-spacing: -0.02em; }
  .sub { font-size: 13px; color: #4a4f5e; margin-top: 3px; }

  .stats { display: flex; gap: 14px; margin-bottom: 36px; }
  .stat { background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 20px 24px; min-width: 140px; }
  .stat.green .stat-num { color: #3ecf8e; }
  .stat.muted .stat-num { color: #4a4f5e; }
  .stat-num { font-size: 32px; font-weight: 700; color: #f0f1f3; line-height: 1; font-family: 'JetBrains Mono', monospace; }
  .stat-label { font-size: 12px; color: #4a4f5e; margin-top: 6px; text-transform: uppercase; letter-spacing: 0.06em; }

  .section { margin-bottom: 32px; }
  .section-title { display: flex; align-items: center; gap: 8px; font-size: 12px; font-weight: 600; color: #4a4f5e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 12px; }
  .dot { width: 7px; height: 7px; border-radius: 50%; }
  .dot.online { background: #3ecf8e; box-shadow: 0 0 5px #3ecf8e66; }
  .dot.offline { background: #3a3f52; }

  .machine-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
  .machine-card { background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 18px 20px; text-align: left; cursor: pointer; transition: all 0.15s; border: none; font-family: inherit; width: 100%; border: 1px solid #1e2028; }
  .machine-card:hover { background: #13161e; border-color: #2a2f3e; transform: translateY(-1px); }
  .machine-card.offline { opacity: 0.6; }
  .card-top { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
  .card-hostname { font-size: 14px; font-weight: 600; color: #e2e4e9; }
  .card-id { font-size: 11px; color: #3a3f52; font-family: 'JetBrains Mono', monospace; margin-bottom: 4px; }
  .card-seen { font-size: 12px; color: #4a4f5e; }
  .badge { font-size: 10px; font-weight: 700; padding: 2px 7px; border-radius: 4px; letter-spacing: 0.04em; }
  .badge.green { background: #0d2e1f; color: #3ecf8e; }
  .badge.gray { background: #1a1d25; color: #4a4f5e; }

  .empty { display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 80px 0; gap: 12px; text-align: center; }
  .empty-icon { font-size: 40px; color: #2a2f3e; }
  .empty-title { font-size: 16px; font-weight: 600; color: #4a4f5e; }
  .empty-sub { font-size: 13px; color: #3a3f52; max-width: 340px; line-height: 1.6; }
</style>