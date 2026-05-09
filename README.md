# Fund Tracking

Fund Tracking is a full-stack fund tracking project.

## Project Scope
- Keep existing API behavior stable for the current frontend.
- Serve a fixed default fund list plus fund detail/search.
- Support auth + watchlist management.

## Tech Stack
- Frontend: Next.js 16, React 19, CSS Modules
- Backend: Flask, Flask-CORS, PyMongo, PyJWT, Requests
- Database: MongoDB
- Email: Resend API (verification code)

## Backend Structure
```text
backend/
  app/
    __init__.py
    config.py
    extensions.py
    version.py
    routes/
      auth.py
      funds.py
      watchlist.py
      health.py
    services/
      fund_fetcher.py
      fund_service.py
      auth_service.py
      email_service.py
    models/
      user.py
      fund.py
    utils/
      security.py
      response.py
  run.py
```

Notes:
- `backend/app/__init__.py` is the Flask app package entry (`create_app`).
- Root `app.py` is the compatibility entry for existing deployment commands.
- `backend/run.py` also starts the same app factory.

## Environment Variables
Backend:
- `MONGO_URI` (required)
- `JWT_SECRET` (recommended)
- `EMAIL_SENDER` (optional, required for register/resend email)
- `EMAIL_PASSWORD` (optional, required for register/resend email)
- `UPDATE_API_KEY` (optional)
- `AUTO_REFRESH_INTERVAL_SECONDS` (optional, default `180`)
- `APP_VERSION` (optional, default `dev`)
- `APP_BUILT_AT` (optional)
- `GIT_COMMIT` / `RAILWAY_GIT_COMMIT_SHA` / `SOURCE_VERSION` (optional)
- `PORT` (optional, default `8080`)

Frontend:
- `NEXT_PUBLIC_API_URL` (required)

## Local Run
1. Install dependencies
```bash
pip install -r requirements.txt
cd client && npm install
```

2. Configure environment
- Copy root `.env.example` to `.env` and set values.
- Copy `client/.env.example` to `client/.env.local`.

3. Start Go backend for fund read APIs

```bash
cd backend-go
go run main.go
```

Go backend runs at:

```text
http://127.0.0.1:8081
```

Implemented Go endpoints:

```text
GET /api/health/mongo
GET /api/funds
GET /api/fund/<fund_code>
GET /api/search_proxy?query=...
```

4. Start legacy Flask backend if you need auth/watchlist/update APIs

```bash
python app.py
```

Alternatively:

```bash
python backend/run.py
```

5. Start frontend

```bash
cd client
npm run dev
```

## API Compatibility (frontend-critical)
- `GET /api/funds`
- `GET /api/fund/<fund_code>`
- `GET /api/search_proxy?query=...`
- `GET /api/funds/search?query=...`
- `GET /api/update`
- `GET /api/watchlist`
- `POST /api/watchlist`
- `DELETE /api/watchlist/<fund_code>`
- `PUT /api/watchlist/<fund_code>`
- `POST /api/auth/login`
- `POST /api/auth/register`
- `POST /api/auth/verify`
- `POST /api/auth/resend`
- `GET /api/version`
- `GET /api/health`
- `GET /health`

## Deployment
- Current `Procfile` uses `web: python app.py`.
- This remains valid after modularization.

## Known Runtime Risks
- If environment injects invalid proxy (`127.0.0.1:9`), outbound fund APIs fail.
- If email vars are missing, register/resend returns explainable error.
- Upstream fund APIs may timeout or return partial fields.

