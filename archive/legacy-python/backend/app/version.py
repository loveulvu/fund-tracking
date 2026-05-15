import time

from flask import current_app


def get_version_payload():
    commit_full = current_app.config.get("APP_COMMIT_SHA") or ""
    short_commit = commit_full[:7] if commit_full else None
    return {
        "service": "fund-tracking-api",
        "version": current_app.config.get("APP_VERSION", "dev"),
        "commit": short_commit,
        "commit_full": commit_full or None,
        "built_at": current_app.config.get("APP_BUILT_AT"),
        "server_time": int(time.time()),
    }

