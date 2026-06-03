export const GO_API_BASE_URL = process.env.NEXT_PUBLIC_GO_API_URL || 'http://127.0.0.1:8081';

export function goApiUrl(path) {
  return `${GO_API_BASE_URL}${path}`;
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

  getFundHistory: (fundCode, range = '7d') =>
    fetchGoJson(`/api/funds/${encodeURIComponent(fundCode)}/history?range=${encodeURIComponent(range)}`),

  searchFunds: (query) =>
    fetch(goApiUrl(`/api/search_proxy?query=${encodeURIComponent(query)}`)),

  updateFunds: () => fetch(goApiUrl('/api/update')),

  startAsyncUpdate: () =>
    fetch('/api/update/async-client', {
      method: 'POST',
    }),

  getUpdateTask: (taskId) =>
    fetch(`/api/update/tasks-client/${encodeURIComponent(taskId)}`),

  getWatchlist: (token) => fetch(goApiUrl('/api/watchlist'), {
    headers: { 'Authorization': `Bearer ${token}` }
  }),

  addToWatchlist: (token, data) => fetch(goApiUrl('/api/watchlist'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify(data)
  }),

  removeFromWatchlist: (token, fundCode) => fetch(goApiUrl(`/api/watchlist/${fundCode}`), {
    method: 'DELETE',
    headers: { 'Authorization': `Bearer ${token}` }
  }),

  updateWatchlist: (token, fundCode, data) => fetch(goApiUrl(`/api/watchlist/${fundCode}`), {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify(data)
  }),

  updateWatchlistThreshold: (token, fundCode, threshold) =>
    api.updateWatchlist(token, fundCode, { alertThreshold: threshold }),

  updateWatchlistPurchaseDate: (token, fundCode, purchaseDate) =>
    api.updateWatchlist(token, fundCode, { purchase_date: purchaseDate }),

  importFund: (token, fundCode) => fetch(goApiUrl('/api/funds/import'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify({ fundCode })
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

  verifyEmailCode: (email, code) => fetch(goApiUrl('/api/auth/verify-email-code'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code })
  }),

  resendEmailCode: (email) => fetch(goApiUrl('/api/auth/resend-email-code'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email })
  })
};

export default api;
