from functools import wraps

import jwt
from flask import current_app, jsonify, request


def require_update_api_key():
    update_api_key = current_app.config.get("UPDATE_API_KEY")
    if not update_api_key:
        return None

    provided = request.headers.get("X-Update-Key") or request.args.get("key")
    if provided != update_api_key:
        return jsonify({"error": "Unauthorized"}), 401

    return None


def extract_bearer_token():
    auth_header = request.headers.get("Authorization")
    if auth_header and auth_header.startswith("Bearer "):
        return auth_header.split(" ")[1]
    return None


def token_required(f):
    @wraps(f)
    def decorated(*args, **kwargs):
        if request.method == "OPTIONS":
            return jsonify({"status": "ok"}), 200

        token = extract_bearer_token()
        if not token:
            return jsonify({"error": "Token is missing"}), 401

        try:
            data = jwt.decode(
                token, current_app.config["JWT_SECRET"], algorithms=["HS256"]
            )
            current_user_id = data["userId"]
        except jwt.ExpiredSignatureError:
            return jsonify({"error": "Token has expired"}), 401
        except jwt.InvalidTokenError:
            return jsonify({"error": "Invalid token"}), 401

        return f(current_user_id, *args, **kwargs)

    return decorated

