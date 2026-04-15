import os

from flask import Flask
from flask_cors import CORS

from .config import Config
from .extensions import ensure_indexes, init_mongo
from .routes import register_routes
from .services.fund_service import init_seed_funds


def create_app():
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

