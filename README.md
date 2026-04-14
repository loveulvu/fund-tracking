# Fund Tracking System

A full-stack fund tracking project with:
- real-time fund data retrieval
- search by fund code/name
- user registration/login with email verification
- personal watchlist and alert threshold management

Frontend is built with Next.js, backend with Flask + MongoDB.

## Highlights

- Default market list is fixed to 10 seed funds (`/api/funds`).
- Search results do not pollute the default list anymore.
- Watchlist supports add/remove/update threshold.
- Auth uses JWT.
- Email verification is sent through Resend API.

## Tech Stack

- Frontend: Next.js 16, React 19, CSS Modules
- Backend: Flask, PyMongo, JWT
- Data fetch: requests + BeautifulSoup/lxml
- Database: MongoDB

## Project Structure

```text
.
|- app.py
|- requirements.txt
|- Procfile
|- client/
|  |- package.json
|  |- .env.example
|  `- src/
|     |- pages/
|     |- components/
|     `- lib/
`- python/
   `- funds.json
```

## Quick Start (Local)

### 1. Prerequisites

- Node.js 18+
- Python 3.9+
- MongoDB

### 2. Install dependencies

```bash
pip install -r requirements.txt
cd client
npm install
```

### 3. Configure environment

Create backend `.env` in project root:

```env
MONGO_URI=mongodb://localhost:27017/fund_tracking
JWT_SECRET=replace_with_a_strong_secret
EMAIL_SENDER=your_resend_sender
EMAIL_PASSWORD=your_resend_api_key
UPDATE_API_KEY=optional_update_key
AUTO_REFRESH_INTERVAL_SECONDS=180
APP_VERSION=2026.04.14
APP_BUILT_AT=2026-04-14T00:00:00Z
PORT=8080
```

Create frontend `client/.env.local`:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
```

### 4. Run services

Backend:

```bash
python app.py
```

Frontend:

```bash
cd client
npm run dev
```

Open:
- Frontend: `http://localhost:3000`
- Backend health: `http://localhost:8080/health`

## API Overview

Auth:
- `POST /api/auth/register`
- `POST /api/auth/verify`
- `POST /api/auth/resend`
- `POST /api/auth/login`

Funds:
- `GET /api/funds`
- `GET /api/fund/<fund_code>`
- `GET /api/search_proxy?query=...`
- `GET /api/funds/search?query=...`
- `GET /api/update` (requires `UPDATE_API_KEY` when configured)
- `GET /api/update_seeds` (requires `UPDATE_API_KEY` when configured)

Watchlist:
- `GET /api/watchlist`
- `POST /api/watchlist`
- `DELETE /api/watchlist/<fund_code>`
- `PUT /api/watchlist/<fund_code>`

Health:
- `GET /`
- `GET /health`
- `GET /api/health`
- `GET /api/version`

## Deployment Notes

- Backend is ready for Railway (`Procfile`: `web: python app.py`).
- CORS currently allows:
  - `https://fundtracking.online`
  - `https://www.fundtracking.online`
  - `http://localhost:3000`

## Troubleshooting

- `CRITICAL: MONGO_URI is missing`: set `MONGO_URI` in env.
- Email verify fails: check `EMAIL_SENDER` and `EMAIL_PASSWORD` (Resend API key).
- Empty or stale fund data: call `/api/update_seeds` with update key.
- Release verification:
  - Backend should expose `GET /api/version` (non-404).
  - Frontend bottom-right badge should show both `FE` and `BE` versions.

## License

MIT
