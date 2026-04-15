import os


class Config:
    CORS_ORIGINS = [
        "https://fundtracking.online",
        "https://www.fundtracking.online",
        "https://fund-tracking-production.up.railway.app",
        "http://localhost:3000",
    ]

    MONGO_URI = os.environ.get("MONGO_URI")
    JWT_SECRET = os.environ.get("JWT_SECRET", "fund_tracking_secret_key_2026")
    EMAIL_SENDER = os.environ.get("EMAIL_SENDER")
    EMAIL_PASSWORD = os.environ.get("EMAIL_PASSWORD")
    UPDATE_API_KEY = os.environ.get("UPDATE_API_KEY")
    AUTO_REFRESH_INTERVAL_SECONDS = int(
        os.environ.get("AUTO_REFRESH_INTERVAL_SECONDS", "180")
    )

    APP_VERSION = os.environ.get("APP_VERSION", "dev")
    APP_COMMIT_SHA = (
        os.environ.get("GIT_COMMIT")
        or os.environ.get("RAILWAY_GIT_COMMIT_SHA")
        or os.environ.get("SOURCE_VERSION")
        or ""
    )
    APP_BUILT_AT = os.environ.get("APP_BUILT_AT")

