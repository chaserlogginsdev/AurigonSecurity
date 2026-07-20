import { writable } from 'svelte/store';

export const toasts = writable([]);
let counter = 0;

// showToast(message, { type: 'info'|'success'|'error', duration: ms, progress: bool })
// progress=true shows an animated bar that drains over `duration` — used to
// visually communicate "this is being applied on the machine now" during
// the ~30s window before the agent's next poll cycle picks up the action.
export function showToast(message, opts = {}) {
  const id = ++counter;
  const type = opts.type || 'info';
  const duration = opts.duration ?? 4000;
  const progress = opts.progress ?? false;
  toasts.update(list => [...list, { id, message, type, duration, progress }]);
  if (duration > 0) {
    setTimeout(() => dismissToast(id), duration);
  }
  return id;
}

export function dismissToast(id) {
  toasts.update(list => list.filter(t => t.id !== id));
}