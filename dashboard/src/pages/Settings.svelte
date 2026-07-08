<script>
  import { token, currentUser, currentRole } from '../lib/stores.js';
  import { api } from '../lib/api.js';

  let current = '';
  let next = '';
  let confirm = '';
  let error = null;
  let success = false;
  let loading = false;

  async function changePassword() {
    error = null; success = false;
    if (next.length < 8) { error = 'New password must be at least 8 characters.'; return; }
    if (next !== confirm) { error = 'Passwords do not match.'; return; }
    loading = true;
    try {
      await api.changePassword($token, current, next);
      success = true;
      current = ''; next = ''; confirm = '';
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }
</script>

<div class="page">
  <div class="topbar">
    <div>
      <h1>Settings</h1>
      <p class="sub">Account preferences</p>
    </div>
  </div>

  <div class="card">
    <div class="who">
      <div class="avatar">{($currentUser || 'A')[0].toUpperCase()}</div>
      <div>
        <div class="username">{$currentUser}</div>
        <div class="role">{$currentRole}</div>
      </div>
    </div>

    <div class="divider"></div>

    <div class="section-title">Change password</div>

    {#if success}
      <div class="success">Password updated successfully.</div>
    {/if}
    {#if error}
      <div class="err">{error}</div>
    {/if}

    <div class="fields">
      <div class="field">
        <label>Current password</label>
        <input type="password" placeholder="••••••••" bind:value={current} autocomplete="current-password"/>
      </div>
      <div class="field">
        <label>New password</label>
        <input type="password" placeholder="Min 8 characters" bind:value={next} autocomplete="new-password"/>
      </div>
      <div class="field">
        <label>Confirm new password</label>
        <input type="password" placeholder="••••••••" bind:value={confirm} autocomplete="new-password"/>
      </div>
      <button on:click={changePassword} disabled={loading}>
        {loading ? 'Saving…' : 'Update password'}
      </button>
    </div>
  </div>
</div>

<style>
  .page { padding: 36px 40px; max-width: 480px; }
  .topbar { margin-bottom: 28px; }
  h1 { font-size: 22px; font-weight: 700; color: #f0f1f3; letter-spacing: -0.02em; }
  .sub { font-size: 13px; color: #4a4f5e; margin-top: 3px; }
  .card { background: #111318; border: 1px solid #1e2028; border-radius: 10px; padding: 28px; }
  .who { display: flex; align-items: center; gap: 14px; margin-bottom: 24px; }
  .avatar { width: 42px; height: 42px; border-radius: 50%; background: #161f38; border: 1px solid #6c8fff44; display: flex; align-items: center; justify-content: center; font-size: 18px; font-weight: 700; color: #6c8fff; flex-shrink: 0; }
  .username { font-size: 15px; font-weight: 600; color: #e2e4e9; }
  .role { font-size: 11px; color: #4a4f5e; text-transform: uppercase; letter-spacing: 0.07em; margin-top: 2px; }
  .divider { height: 1px; background: #1e2028; margin-bottom: 24px; }
  .section-title { font-size: 12px; font-weight: 600; color: #6a7090; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 16px; }
  .fields { display: flex; flex-direction: column; gap: 14px; }
  .field { display: flex; flex-direction: column; gap: 5px; }
  label { font-size: 11px; font-weight: 600; color: #6a7090; text-transform: uppercase; letter-spacing: 0.07em; }
  input { background: #0d0f12; border: 1px solid #1e2028; border-radius: 7px; color: #d0d3e0; font-size: 13px; padding: 9px 12px; outline: none; font-family: inherit; transition: border-color 0.15s; }
  input:focus { border-color: #6c8fff55; }
  input::placeholder { color: #2e3248; }
  button { background: #6c8fff; color: #fff; border: none; border-radius: 7px; padding: 10px; font-size: 13px; font-weight: 600; cursor: pointer; font-family: inherit; transition: background 0.15s; margin-top: 4px; }
  button:hover { background: #5a7aee; }
  button:disabled { opacity: 0.6; cursor: not-allowed; }
  .success { background: #0d2e1f; border: 1px solid #1a5a3a; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #3ecf8e; margin-bottom: 14px; }
  .err { background: #2a1010; border: 1px solid #5a2020; border-radius: 7px; padding: 10px 14px; font-size: 13px; color: #e55; margin-bottom: 14px; }
</style>