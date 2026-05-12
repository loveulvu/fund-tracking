export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || '';
export const GO_API_BASE_URL = process.env.NEXT_PUBLIC_GO_API_URL || 'http://127.0.0.1:8081';

const LOCAL_HOSTNAMES = new Set(['localhost', '127.0.0.1', '::1']);

export function getApiBaseUrl() {
  if (typeof window !== 'undefined' && LOCAL_HOSTNAMES.has(window.location.hostname)) {
    return '';
  }

  return API_BASE_URL;
}

export function apiUrl(path) {
  return `${getApiBaseUrl()}${path}`;
}

export function goApiUrl(path) {
  return `${GO_API_BASE_URL}${path}`;
}

async function fetchJson(path, options = {}) {
  const response = await fetch(apiUrl(path), options);
  if (!response.ok) {
    throw new Error(`接口返回 ${response.status}`);
  }

  return response.json();
}

async function fetchGoJson(path, options = {}) {
  const response = await fetch(goApiUrl(path), options);
  if (!response.ok) {
    throw new Error(`接口返回 ${response.status}`);
  }

  return response.json();
}

function normalizeFundsPayload(payload) {
  if (Array.isArray(payload)) return payload;
  if (Array.isArray(payload?.data)) return payload.data;
  throw new Error('接口返回格式不是基金数组');
}

export const api = {
  getFunds: async (options = {}) =>
    normalizeFundsPayload(await fetchGoJson('/api/funds', options)),

  getFund: (fundCode) =>
    fetch(goApiUrl(`/api/fund/${fundCode}`)),

  searchFunds: (query) =>
    fetch(goApiUrl(`/api/search_proxy?query=${encodeURIComponent(query)}`)),

  updateFunds: () => fetch(goApiUrl('/api/update')),

  getWatchlist: (token) => fetch(goApiUrl('/api/watchlist'), {
    headers: { 'Authorization': `Bearer ${token}` }
  }),

  addToWatchlist: (token, data) => fetch(apiUrl('/api/watchlist'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify(data)
  }),

  removeFromWatchlist: (token, fundCode) => fetch(apiUrl(`/api/watchlist/${fundCode}`), {
    method: 'DELETE',
    headers: { 'Authorization': `Bearer ${token}` }
  }),

  updateWatchlistThreshold: (token, fundCode, threshold) => fetch(apiUrl(`/api/watchlist/${fundCode}`), {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify({ alertThreshold: threshold })
  }),

  login: (email, password) => fetch(goApiUrl('/api/auth/login'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  }),

  register: (email, password) => fetch(goApiUrl('/api/auth/register'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  }),

  verifyEmail: (email, code, password) => fetch(apiUrl('/api/auth/verify'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code, password })
  }),

  resendVerification: (email) => fetch(apiUrl('/api/auth/resend'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email })
  })
};

export default api;