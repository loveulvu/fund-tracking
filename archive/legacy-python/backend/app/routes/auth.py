from datetime import datetime, timedelta, timezone

from flask import Blueprint, jsonify, request

from .. import extensions
from ..services.auth_service import (
    encode_login_token,
    generate_verification_code,
    hash_password,
    verify_password,
)
from ..services.email_service import (
    email_config_missing_message,
    is_email_configured,
    send_verification_email,
)
from ..utils.response import check_db_status, options_ok

auth_bp = Blueprint("auth", __name__)


@auth_bp.route("/api/auth/register", methods=["POST", "OPTIONS"])
def register():
    if request.method == "OPTIONS":
        return options_ok()

    db_check = check_db_status()
    if db_check:
        return db_check

    try:
        data = request.get_json() or {}
        email = data.get("email")
        password = data.get("password")

        if not email or not password:
            return jsonify({"error": "Email and password are required"}), 400

        existing_user = extensions.users_collection.find_one({"email": email})
        if existing_user and existing_user.get("is_verified"):
            print(f"[register] user already exists and verified: {email}")
            return jsonify({"error": "User already exists"}), 409

        if not is_email_configured():
            return jsonify({"error": email_config_missing_message()}), 503

        verification_code = generate_verification_code()
        success = send_verification_email(email, verification_code)
        if not success:
            print(f"[register] failed to send verification email: {email}")
            return jsonify({"error": "Failed to send verification code"}), 500

        expires_at = datetime.now(timezone.utc) + timedelta(minutes=10)
        pending_user = {
            "email": email,
            "password": hash_password(password),
            "verification_code": verification_code,
            "verification_code_expires": expires_at,
            "createdAt": datetime.now(timezone.utc),
        }

        extensions.pending_users_collection.update_one(
            {"email": email},
            {"$set": pending_user},
            upsert=True,
        )

        print(f"[register] verification code sent and saved for {email}")
        return (
            jsonify({"status": "success", "message": "Verification code sent to your email"}),
            200,
        )
    except Exception as exc:
        print(f"[register] registration failed: {str(exc)}")
        return jsonify({"error": "Registration failed"}), 500


@auth_bp.route("/api/auth/verify", methods=["POST", "OPTIONS"])
def verify():
    if request.method == "OPTIONS":
        return options_ok()

    db_check = check_db_status()
    if db_check:
        return db_check

    try:
        data = request.get_json() or {}
        email = data.get("email")
        code = data.get("code")

        print(f"[verify] received verification request: email={email}, code={code}")

        if not email or not code:
            print(f"[verify] missing required params: email={email}, code={code}")
            return jsonify({"error": "Email and verification code are required"}), 400

        pending_user = extensions.pending_users_collection.find_one({"email": email})
        if not pending_user:
            print(f"[verify] pending user not found: {email}")
            return jsonify({"error": "User not found or registration expired"}), 404

        stored_code = pending_user.get("verification_code")
        expires_at = pending_user.get("verification_code_expires")

        code_str = str(code).strip()
        stored_code_str = str(stored_code).strip() if stored_code else ""
        if code_str != stored_code_str:
            print("[verify] invalid verification code")
            return jsonify({"error": "Invalid verification code"}), 400

        if expires_at and isinstance(expires_at, datetime):
            now = datetime.now(timezone.utc)
            if expires_at.tzinfo is None:
                expires_at = expires_at.replace(tzinfo=timezone.utc)
            if now > expires_at:
                print("[verify] verification code expired")
                return jsonify({"error": "Verification code expired"}), 400

        verified_user = {
            "email": email,
            "password": pending_user.get("password"),
            "is_verified": True,
            "verified_at": datetime.now(timezone.utc),
            "createdAt": pending_user.get("createdAt", datetime.now(timezone.utc)),
        }

        extensions.users_collection.update_one(
            {"email": email},
            {"$set": verified_user},
            upsert=True,
        )
        saved_user = extensions.users_collection.find_one({"email": email})
        user_id = str(saved_user["_id"])

        extensions.pending_users_collection.delete_one({"email": email})
        token = encode_login_token(user_id, email)

        print(f"[verify] user verified successfully and moved to users: {email}")
        return (
            jsonify(
                {
                    "status": "success",
                    "message": "Verification successful",
                    "token": token,
                    "email": email,
                }
            ),
            200,
        )
    except Exception as exc:
        print(f"[verify] verification failed: {str(exc)}")
        return jsonify({"error": "Verification failed"}), 500


@auth_bp.route("/api/auth/resend", methods=["POST", "OPTIONS"])
def resend_verification():
    if request.method == "OPTIONS":
        return options_ok()

    db_check = check_db_status()
    if db_check:
        return db_check

    try:
        data = request.get_json() or {}
        email = data.get("email")
        if not email:
            return jsonify({"error": "Email is required"}), 400

        pending_user = extensions.pending_users_collection.find_one({"email": email})
        if not pending_user:
            return jsonify({"error": "User not found or registration expired"}), 404

        if not is_email_configured():
            return jsonify({"error": email_config_missing_message()}), 503

        verification_code = generate_verification_code()
        success = send_verification_email(email, verification_code)
        if not success:
            return jsonify({"error": "Failed to send verification code"}), 500

        extensions.pending_users_collection.update_one(
            {"email": email},
            {
                "$set": {
                    "verification_code": verification_code,
                    "verification_code_expires": datetime.now(timezone.utc)
                    + timedelta(minutes=10),
                }
            },
        )

        return jsonify({"status": "success", "message": "Verification code resent"}), 200
    except Exception as exc:
        print(f"[resend] resend failed: {str(exc)}")
        return jsonify({"error": "Resend failed"}), 500


@auth_bp.route("/api/auth/login", methods=["POST", "OPTIONS"])
def login():
    if request.method == "OPTIONS":
        return options_ok()

    db_check = check_db_status()
    if db_check:
        return db_check

    try:
        data = request.get_json() or {}
        email = data.get("email")
        password = data.get("password")

        if not email or not password:
            return jsonify({"error": "Email and password are required"}), 400

        user = extensions.users_collection.find_one({"email": email})
        if not user or not verify_password(user.get("password"), password):
            return jsonify({"error": "Invalid email or password"}), 401

        if user.get("password") == password:
            extensions.users_collection.update_one(
                {"_id": user["_id"]},
                {"$set": {"password": hash_password(password)}},
            )

        token = encode_login_token(user["_id"], email)
        print(f"[login] user logged in successfully: {email}")
        return jsonify({"status": "success", "token": token, "email": email}), 200
    except Exception as exc:
        print(f"[login] login failed: {str(exc)}")
        return jsonify({"error": "Login failed"}), 500
