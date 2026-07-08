const BASE = '';

function authHeaders(token) {
  return { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' };
}

async function request(method, path, token, body) {
  const res = await fetch(BASE + path, {
    method,
    headers: authHeaders(token),
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) throw { status: 401, message: 'Session expired' };
  if (!res.ok) throw { status: res.status, message: await res.text() };
  const text = await res.text();
  return text ? JSON.parse(text) : null;
}

export const api = {
  // Auth
  login: (tenantSlug, username, password) =>
    fetch(BASE + '/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenant_slug: tenantSlug, username, password }),
    }).then(async res => {
      if (!res.ok) throw new Error('Invalid username or password');
      return res.json();
    }),

  changePassword: (token, current, next) =>
    request('POST', '/change-password', token, { current_password: current, new_password: next }),

  // Machines
  getMachines: (token) => request('GET', '/machines', token),

  // Accounts
  getAccounts: (token, machineId) =>
    request('GET', `/accounts?machine_id=${machineId}`, token),

  // Actions
  createAction: (token, machineId, type, username, params = {}) =>
    request('POST', '/actions/create', token, { machine_id: machineId, type, username, params }),

  getActionStatus: (token, machineId) =>
    request('GET', `/actions/status?machine_id=${machineId}`, token),

  // Audit
  getAuditLog: (token) => request('GET', '/audit', token),

  // Users
  getUsers: (token) => request('GET', '/users', token),

  createUser: (token, username, password, role) =>
    request('POST', '/users/create', token, { username, password, role }),

  deleteUser: (token, username) =>
    request('POST', '/users/delete', token, { username }),

  // Deploy keys
  getDeployKeys: (token) => request('GET', '/deploy-keys', token),

  generateDeployKey: (token, label, backendURL) =>
    request('POST', '/deploy-keys/generate', token, { label, backend_url: backendURL }),

  revokeDeployKey: (token, id) =>
    request('POST', '/deploy-keys/revoke', token, { id }),

  // Groups
  getGroups: (token, machineId) =>
    request('GET', `/groups?machine_id=${machineId}`, token),
};

export function isOnline(lastSeen) {
  if (!lastSeen) return false;
  return (Date.now() - new Date(lastSeen).getTime()) < 5 * 60 * 1000;
}

export function formatDate(d) {
  if (!d) return '—';
  try {
    return new Date(d).toLocaleDateString('en-US', {
      year: 'numeric', month: 'short', day: 'numeric'
    });
  } catch { return '—'; }
}

export function formatDateTime(d) {
  if (!d) return '—';
  try {
    return new Date(d).toLocaleString('en-US', {
      month: 'short', day: 'numeric',
      hour: '2-digit', minute: '2-digit'
    });
  } catch { return '—'; }
}