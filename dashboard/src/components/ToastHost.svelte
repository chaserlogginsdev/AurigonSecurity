<script>
  import { toasts, dismissToast } from '../lib/toast.js';
</script>

<div class="toast-host">
  {#each $toasts as t (t.id)}
    <div class="toast {t.type}" on:click={() => dismissToast(t.id)}>
      <div class="toast-msg">{t.message}</div>
    </div>
  {/each}
</div>

<style>
  .toast-host {
    position: fixed; bottom: 20px; right: 20px;
    display: flex; flex-direction: column; gap: 8px;
    z-index: 300; max-width: 340px;
  }
  .toast {
    background: #16181f; border: 1px solid #262b38; border-left: 3px solid #6c8fff;
    border-radius: 8px; padding: 12px 14px;
    box-shadow: 0 10px 30px rgba(0,0,0,.4);
    cursor: pointer; animation: slidein 0.2s ease-out;
  }
  .toast.success { border-left-color: #3ecf8e; }
  .toast.error { border-left-color: #e55; }
  .toast-msg { font-size: 13px; color: #d0d3e0; line-height: 1.4; }
  @keyframes slidein { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }
</style>