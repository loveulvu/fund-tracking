import random

import jwt
from flask import current_app
from werkzeug.security import check_password_hash, generate_password_hash

from ..models.user import build_token_payload


def hash_password(password):
    return generate_password_hash(password)


def verify_password(stored_password, candidate_password):
    if not stored_password or not candidate_password:
        return False

    try:
        if check_password_hash(stored_password, candidate_password):
            return True
    except Exception:
        pass

    return stored_password == candidate_password


def generate_verification_code():
    return str(random.randint(100000, 999999))


def encode_login_token(user_id, email):
    payload = build_token_payload(user_id, email, expires_hours=24)
    return jwt.encode(payload, current_app.config["JWT_SECRET"], algorithm="HS256")


def decode_token(token):
    return jwt.decode(token, current_app.config["JWT_SECRET"], algorithms=["HS256"])

