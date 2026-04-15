import time

from flask import Blueprint, current_app, jsonify

from .. import extensions
from ..models.fund import DEFAULT_FUND_CODES
from ..services.fund_service import get_latest_update_time
from ..version import get_version_payload

health_bp = Blueprint("health", __name__)


@health_bp.route("/")
def index():
    version = get_version_payload()
    version_text = f"version={version['version']}, commit={version['commit'] or 'unknown'}"
    if extensions.db_error_message:
        return (
            f"API is Running, but Database Error: "
            f"{extensions.db_error_message} ({version_text})",
            200,
        )
    return (
        "Fund Tracking API is Running successfully with DB connected. "
        f"({version_text})",
        200,
    )


@health_bp.route("/api/version")
def api_version():
    return jsonify(get_version_payload())


@health_bp.route("/health")
def health():
    latest_update_time = get_latest_update_time(DEFAULT_FUND_CODES)
    latest_update_age_seconds = None
    if latest_update_time:
        latest_update_age_seconds = max(0, int(time.time()) - int(latest_update_time))

    return jsonify(
        {
            "status": "ok",
            "version": get_version_payload(),
            "db_connected": extensions.collection is not None,
            "db_error": extensions.db_error_message,
            "latest_update_time": latest_update_time,
            "latest_update_age_seconds": latest_update_age_seconds,
            "auto_refresh_interval_seconds": current_app.config.get(
                "AUTO_REFRESH_INTERVAL_SECONDS", 180
            ),
        }
    )


@health_bp.route("/api/health")
def api_health():
    return jsonify({"status": "ok", "version": get_version_payload()})
