<script>
  import { onMount } from 'svelte';
  import { token } from '../lib/stores.js';
  import { api, formatDateTime } from '../lib/api.js';

  let actions = [];
  let loading = true;
  let error = null;
  let search = '';
  let statusFilter = 'all';

  onMount(async () => {
    try { actions = await api.getAuditLog($token) || []; }
    catch (e) { error = e.message; }
    finally { loading = false; }
  });

  $: filtered = actions.filter(a => {
    const s = search.toLowerCase();
    const matchSearch =
      a.username.toLowerCase().includes(s) ||
      a.hostname?.toLowerCase().includes(s) ||
      a.created_by?.toLowerCase().includes(s) ||
      a.type.toLowerCase().includes(s);
    const matchStatus = statusFilter === 'all' ? true : a.status === statusFilter;
    return matchSearch && matchStatus;
  });
</script>

<div class="page">
  <div class="topbar">
    <div>
      <h1>Audit Log</h1>
      <p class="sub">Every action taken across your fleet</p>
    </div>
    <div class="controls">
      <select bind:value={statusFilter}>
        <option value="all">All statuses</option>
        <option value="completed">Completed</option>
        <option value="pending">Pending</option>
        <option value="failed">Failed</option>
      </select>
      <input class="search" type="text" placeholder="Search…" bind:value={search}/>
    </div>
  </div>

  {#if loading}
    <div class="state"><div class="spinner"></div></div>
  {:else if error}
    <div class="state error"><p>{error}</p></div>
  {:else if filtered.length === 0}
    <div class="state"><p>No actions found.</p></div>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Type</th><th>Username</th><th>Machine</th>
            <th>Status</th><th>By</th><th>Created</th><th>Result</th>
          </tr>
        </thead>
        <tbody>
          {#each filtered as a}
            <tr>
              <td><code class="type">{a.type}</code></td>
              <td class="bold">{a.username}</td>
              <td class="muted">{a.hostname || a.machine_id}</td>
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
              <td class="muted">{a.result || '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .page { padding: 36px 40px; max-width: 1200px; }
  .topbar { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 28px; gap: 16px; }
  h1 { font-size: 22px; font-weight: 700; color: #f0f1f3; letter-spacing: -0.02em; }
  .sub { font-size: 13px; color: #4a4f5e; margin-top: 3px; }
  .controls { display: flex; gap: 10px; align-items: center; }
  select { background: #111318; border: 1px solid #1e2028; border-radius: 8px; color: #d0d3e0; font-size: 13px; padding: 8px 14px; outline: none; font-family: inherit; cursor: pointer; }
  .search { background: #111318; border: 1px solid #1e2028; border-radius: 8px; color: #d0d3e0; font-size: 13px; padding: 8px 14px; width: 220px; outline: none; font-family: inherit; }
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
  .type { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #6a7090; background: #1a1d26; padding: 2px 6px; border-radius: 4px; }
  .badge { display: inline-block; font-size: 10px; font-weight: 700; padding: 2px 7px; border-radius: 4px; }
  .badge.green { background: #0d2e1f; color: #3ecf8e; }
  .badge.amber { background: #2e1f08; color: #f5a623; }
  .badge.red { background: #2a1010; color: #e55; }
  .state { display: flex; align-items: center; justify-content: center; padding: 80px; color: #4a4f5e; font-size: 13px; }
  .spinner { width: 22px; height: 22px; border: 2px solid #1e2028; border-top-color: #6c8fff; border-radius: 50%; animation: spin 0.7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>