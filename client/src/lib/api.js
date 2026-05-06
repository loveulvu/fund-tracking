export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || '';

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

async function fetchJson(path, options = {}) {
  const response = await fetch(apiUrl(path), options);
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
  getFunds: async (options = {}) => normalizeFundsPayload(await fetchJson('/api/funds', options)),
  getFund: (fundCode) => fetch(apiUrl(`/api/fund/${fundCode}`)),
  searchFunds: (query) => fetch(apiUrl(`/api/search_proxy?query=${encodeURIComponent(query)}`)),
  updateFunds: () => fetch(apiUrl('/api/update')),
  
  getWatchlist: (token) => fetch(apiUrl('/api/watchlist'), {
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
  
  login: (email, password) => fetch(apiUrl('/api/auth/login'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  }),
  register: (email, password) => fetch(apiUrl('/api/auth/register'), {
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
