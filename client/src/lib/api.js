const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.fundtracking.online';

export const api = {
  getFunds: (options = {}) => fetch(`${API_BASE_URL}/api/funds`, options),
  getFund: (fundCode) => fetch(`${API_BASE_URL}/api/fund/${fundCode}`),
  searchFunds: (query) => fetch(`${API_BASE_URL}/api/search_proxy?query=${encodeURIComponent(query)}`),
  updateFunds: () => fetch(`${API_BASE_URL}/api/update`),
  
  getWatchlist: (token) => fetch(`${API_BASE_URL}/api/watchlist`, {
    headers: { 'Authorization': `Bearer ${token}` }
  }),
  addToWatchlist: (token, data) => fetch(`${API_BASE_URL}/api/watchlist`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify(data)
  }),
  removeFromWatchlist: (token, fundCode) => fetch(`${API_BASE_URL}/api/watchlist/${fundCode}`, {
    method: 'DELETE',
    headers: { 'Authorization': `Bearer ${token}` }
  }),
  updateWatchlistThreshold: (token, fundCode, threshold) => fetch(`${API_BASE_URL}/api/watchlist/${fundCode}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify({ alertThreshold: threshold })
  }),
  
  login: (email, password) => fetch(`${API_BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  }),
  register: (email, password) => fetch(`${API_BASE_URL}/api/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  }),
  verifyEmail: (email, code, password) => fetch(`${API_BASE_URL}/api/auth/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code, password })
  }),
  resendVerification: (email) => fetch(`${API_BASE_URL}/api/auth/resend`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email })
  })
};

export default api;
