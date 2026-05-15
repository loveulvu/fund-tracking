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

  searchFunds: (query) =>
    fetch(goApiUrl(`/api/search_proxy?query=${encodeURIComponent(query)}`)),

  updateFunds: () => fetch(goApiUrl('/api/update')),

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

  updateWatchlistThreshold: (token, fundCode, threshold) => fetch(goApiUrl(`/api/watchlist/${fundCode}`), {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify({ alertThreshold: threshold })
  }),

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
  })
};

export default api;
