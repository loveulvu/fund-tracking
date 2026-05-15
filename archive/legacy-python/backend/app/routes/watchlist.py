from datetime import datetime, timezone

import jwt
from flask import Blueprint, current_app, jsonify, request

from .. import extensions
from ..utils.response import check_db_status, options_ok
from ..utils.security import extract_bearer_token

watchlist_bp = Blueprint("watchlist", __name__)


@watchlist_bp.route("/api/watchlist", methods=["GET", "OPTIONS"])
def get_watchlist():
    print(f"[DEBUG] get_watchlist called, method: {request.method}")
    if request.method == "OPTIONS":
        print("[DEBUG] OPTIONS preflight responded 200")
        return options_ok()

    db_check = check_db_status()
    if db_check:
        print(f"[DEBUG] DB status error: {db_check}")
        return db_check

    token = extract_bearer_token()
    if not token:
        print("[DEBUG] Token missing")
        return jsonify({"error": "Token is missing"}), 401

    try:
        data = jwt.decode(token, current_app.config["JWT_SECRET"], algorithms=["HS256"])
        current_user_id = data["userId"]
        print(f"[DEBUG] Token decoded, userId: {current_user_id}")
    except jwt.ExpiredSignatureError:
        print("[DEBUG] Token expired")
        return jsonify({"error": "Token has expired"}), 401
    except jwt.InvalidTokenError as exc:
        print(f"[DEBUG] Invalid token: {str(exc)}")
        return jsonify({"error": "Invalid token"}), 401

    try:
        watchlist = list(
            extensions.watchlist_collection.find({"userId": current_user_id}, {"_id": 0})
        )
        print(f"[watchlist] user {current_user_id} has {len(watchlist)} items")
        return jsonify(watchlist)
    except Exception as exc:
        print(f"Failed to fetch watchlist: {str(exc)}")
        return jsonify({"error": "Failed to fetch watchlist"}), 500


@watchlist_bp.route("/api/watchlist", methods=["POST", "OPTIONS"])
def add_to_watchlist():
    print(f"[DEBUG] add_to_watchlist called, method: {request.method}")
    if request.method == "OPTIONS":
        print("[DEBUG] OPTIONS preflight responded 200")
        return options_ok()

    db_check = check_db_status()
    if db_check:
        print(f"[DEBUG] DB status error: {db_check}")
        return db_check

    token = extract_bearer_token()
    if not token:
        print("[DEBUG] Token missing")
        return jsonify({"error": "Token is missing"}), 401

    try:
        data = jwt.decode(token, current_app.config["JWT_SECRET"], algorithms=["HS256"])
        current_user_id = data["userId"]
        print(f"[DEBUG] Token decoded, userId: {current_user_id}")
    except jwt.ExpiredSignatureError:
        print("[DEBUG] Token expired")
        return jsonify({"error": "Token has expired"}), 401
    except jwt.InvalidTokenError as exc:
        print(f"[DEBUG] Invalid token: {str(exc)}")
        return jsonify({"error": "Invalid token"}), 401

    try:
        req_data = request.get_json() or {}
        print(f"[DEBUG] request body: {req_data}")
        fund_code = req_data.get("fundCode")
        fund_name = req_data.get("fundName")
        threshold = req_data.get("alertThreshold", 5)

        if not fund_code or not fund_name:
            print(
                f"[DEBUG] missing required params: "
                f"fundCode={fund_code}, fundName={fund_name}"
            )
            return jsonify({"error": "fundCode and fundName are required"}), 400

        existing = extensions.watchlist_collection.find_one(
            {"userId": current_user_id, "fundCode": fund_code}
        )
        if existing:
            print(f"[DEBUG] Fund {fund_code} already in watchlist")
            return jsonify({"error": "Fund already in watchlist"}), 400

        watchlist_item = {
            "userId": current_user_id,
            "fundCode": fund_code,
            "fundName": fund_name,
            "alertThreshold": threshold,
            "addedAt": datetime.now(timezone.utc),
        }

        extensions.watchlist_collection.insert_one(watchlist_item)
        watchlist_item.pop("_id", None)
        print(f"[watchlist] user {current_user_id} added {fund_code}")
        return jsonify(watchlist_item), 201
    except Exception as exc:
        print(f"Failed to add to watchlist: {str(exc)}")
        return jsonify({"error": "Failed to add to watchlist"}), 500


@watchlist_bp.route("/api/watchlist/<fund_code>", methods=["DELETE", "OPTIONS"])
def remove_from_watchlist(fund_code):
    if request.method == "OPTIONS":
        return options_ok()

    db_check = check_db_status()
    if db_check:
        return db_check

    token = extract_bearer_token()
    if not token:
        return jsonify({"error": "Token is missing"}), 401

    try:
        data = jwt.decode(token, current_app.config["JWT_SECRET"], algorithms=["HS256"])
        current_user_id = data["userId"]
    except jwt.ExpiredSignatureError:
        return jsonify({"error": "Token has expired"}), 401
    except jwt.InvalidTokenError:
        return jsonify({"error": "Invalid token"}), 401

    try:
        result = extensions.watchlist_collection.delete_one(
            {"userId": current_user_id, "fundCode": fund_code}
        )
        if result.deleted_count == 0:
            return jsonify({"error": "Fund not found in watchlist"}), 404

        print(f"[watchlist] user {current_user_id} removed {fund_code}")
        return jsonify({"message": "Successfully removed from watchlist"})
    except Exception as exc:
        print(f"Failed to remove from watchlist: {str(exc)}")
        return jsonify({"error": "Failed to remove from watchlist"}), 500


@watchlist_bp.route("/api/watchlist/<fund_code>", methods=["PUT", "OPTIONS"])
def update_watchlist_threshold(fund_code):
    if request.method == "OPTIONS":
        return options_ok()

    db_check = check_db_status()
    if db_check:
        return db_check

    token = extract_bearer_token()
    if not token:
        return jsonify({"error": "Token is missing"}), 401

    try:
        data = jwt.decode(token, current_app.config["JWT_SECRET"], algorithms=["HS256"])
        current_user_id = data["userId"]
    except jwt.ExpiredSignatureError:
        return jsonify({"error": "Token has expired"}), 401
    except jwt.InvalidTokenError:
        return jsonify({"error": "Invalid token"}), 401

    try:
        req_data = request.get_json() or {}
        new_threshold = req_data.get("alertThreshold")
        if new_threshold is None:
            return jsonify({"error": "alertThreshold is required"}), 400

        result = extensions.watchlist_collection.update_one(
            {"userId": current_user_id, "fundCode": fund_code},
            {"$set": {"alertThreshold": new_threshold}},
        )
        if result.matched_count == 0:
            return jsonify({"error": "Fund not found in watchlist"}), 404

        updated_item = extensions.watchlist_collection.find_one(
            {"userId": current_user_id, "fundCode": fund_code},
            {"_id": 0},
        )

        print(
            f"[watchlist] user {current_user_id} updated "
            f"{fund_code} threshold to {new_threshold}"
        )
        return jsonify(updated_item)
    except Exception as exc:
        print(f"Failed to update threshold: {str(exc)}")
        return jsonify({"error": "Failed to update threshold"}), 500

