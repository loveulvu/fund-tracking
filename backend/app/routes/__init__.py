from .auth import auth_bp
from .funds import funds_bp
from .health import health_bp
from .watchlist import watchlist_bp


def register_routes(app):
    app.register_blueprint(funds_bp)
    app.register_blueprint(watchlist_bp)
    app.register_blueprint(auth_bp)
    app.register_blueprint(health_bp)

