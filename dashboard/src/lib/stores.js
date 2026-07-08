import { writable, derived } from 'svelte/store';

// Auth
export const token = writable(null);
export const currentUser = writable(null);
export const currentRole = writable(null);
export const tenantId = writable(null);
export const tenantName = writable(null);

// Navigation
export const page = writable('overview'); // overview | machines | audit | users | deploy | settings
export const selectedMachine = writable(null);
export const machineTab = writable('accounts'); // accounts | groups | actions

// Fleet data
export const machines = writable([]);

// Derived
export const isAdmin = derived(currentRole, $role => $role === 'admin');
export const onlineCount = derived(machines, $machines => {
  const cutoff = Date.now() - 5 * 60 * 1000;
  return $machines.filter(m => new Date(m.last_seen).getTime() > cutoff).length;
});

// Persist session
export function saveSession(t, user, role, tId, tName) {
  token.set(t);
  currentUser.set(user);
  currentRole.set(role);
  tenantId.set(tId);
  tenantName.set(tName);
  sessionStorage.setItem('aurigon_token', t);
  sessionStorage.setItem('aurigon_user', user);
  sessionStorage.setItem('aurigon_role', role);
  sessionStorage.setItem('aurigon_tenant_id', tId || '');
  sessionStorage.setItem('aurigon_tenant_name', tName || '');
}

export function clearSession() {
  token.set(null);
  currentUser.set(null);
  currentRole.set(null);
  tenantId.set(null);
  tenantName.set(null);
  machines.set([]);
  selectedMachine.set(null);
  page.set('overview');
  sessionStorage.removeItem('aurigon_token');
  sessionStorage.removeItem('aurigon_user');
  sessionStorage.removeItem('aurigon_role');
  sessionStorage.removeItem('aurigon_tenant_id');
  sessionStorage.removeItem('aurigon_tenant_name');
}

export function restoreSession() {
  const t = sessionStorage.getItem('aurigon_token');
  const u = sessionStorage.getItem('aurigon_user');
  const r = sessionStorage.getItem('aurigon_role');
  const tId = sessionStorage.getItem('aurigon_tenant_id');
  const tName = sessionStorage.getItem('aurigon_tenant_name');
  if (t) {
    token.set(t); currentUser.set(u); currentRole.set(r);
    tenantId.set(tId); tenantName.set(tName);
    return true;
  }
  return false;
}