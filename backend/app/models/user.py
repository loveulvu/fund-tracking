from datetime import datetime, timedelta, timezone


def build_token_payload(user_id, email, expires_hours=24):
    return {
        "userId": str(user_id),
        "email": email,
        "exp": datetime.now(timezone.utc) + timedelta(hours=expires_hours),
    }

