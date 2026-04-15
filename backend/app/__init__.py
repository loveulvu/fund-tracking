import os
import sys

from flask import Flask
from flask_cors import CORS

from .config import Config
from .extensions import ensure_indexes, init_mongo
from .routes import register_routes
from .services.fund_service import init_seed_funds


def _configure_stdio_for_unicode():
    """Avoid UnicodeEncodeError on Windows console/log sinks."""
    for stream_name in ("stdout", "stderr"):
        stream = getattr(sys, stream_name, None)
        if stream and hasattr(stream, "reconfigure"):
            try:
                stream.reconfigure(encoding="utf-8", errors="replace")
            except Exception:
                pass


def _sanitize_invalid_proxy_env():
    """
    Remove known-bad loopback proxy values that break outbound HTTP requests.
    Only strips explicit :9 loopback proxies to avoid touching valid proxies.
    """
    proxy_keys = [
        "HTTP_PROXY",
        "HTTPS_PROXY",
        "ALL_PROXY",
        "http_proxy",
        "https_proxy",
        "all_proxy",
    ]
    removed_keys = []
    for key in proxy_keys:
        value = os.environ.get(key)
        if not value:
            continue
        lower_value = value.lower()
        if "127.0.0.1:9" in lower_value or "localhost:9" in lower_value:
            os.environ.pop(key, None)
            removed_keys.append(key)

    if removed_keys:
        print(
            "[runtime] Removed invalid proxy env vars: "
            + ", ".join(sorted(removed_keys))
        )


def create_app():
    _configure_stdio_for_unicode()
    _sanitize_invalid_proxy_env()

    app = Flask(__name__)
    app.config.from_object(Config)

    CORS(
        app,
        resources={r"/api/*": {"origins": app.config.get("CORS_ORIGINS", [])}},
        supports_credentials=True,
    )

    if "JWT_SECRET" not in os.environ:
        print(
            "[security] WARNING: JWT_SECRET is using fallback value. "
            "Please set JWT_SECRET in Railway."
        )

    init_mongo(app)
    ensure_indexes()

    with app.app_context():
        init_seed_funds()

    register_routes(app)
    return app
