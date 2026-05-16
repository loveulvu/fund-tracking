import threading
import traceback

from pymongo import MongoClient

db = None
collection = None
watchlist_collection = None
users_collection = None
pending_users_collection = None
db_error_message = None
refresh_lock = threading.Lock()


def init_mongo(app):
    global db
    global collection
    global watchlist_collection
    global users_collection
    global pending_users_collection
    global db_error_message

    mongo_uri = app.config.get("MONGO_URI")
    db_error_message = None

    if not mongo_uri:
        db_error_message = (
            "CRITICAL: MONGO_URI is missing. Please set it in backend environment variables."
        )
        print(db_error_message)
        return

    try:
        client = MongoClient(mongo_uri, serverSelectionTimeoutMS=5000)
        db = client["fund_tracking"]
        collection = db["fund_data"]
        watchlist_collection = db["watchlists"]
        users_collection = db["users"]
        pending_users_collection = db["pending_users"]
        client.admin.command("ping")
        print(f"Flask connecting to database: {db.name}")
        print("MongoDB connected.")
    except Exception as exc:
        db_error_message = f"MongoDB 连接失败: {str(exc)}"
        print(db_error_message)
        print(traceback.format_exc())


def ensure_indexes():
    if collection is None:
        return

    try:
        collection.create_index("fund_code", unique=True, name="uniq_fund_code")
        watchlist_collection.create_index(
            [("userId", 1), ("fundCode", 1)],
            unique=True,
            name="uniq_watchlist_user_fund",
        )
        users_collection.create_index("email", unique=True, name="uniq_user_email")
        pending_users_collection.create_index(
            "email",
            unique=True,
            name="uniq_pending_email",
        )
        pending_users_collection.create_index(
            "verification_code_expires",
            expireAfterSeconds=0,
            name="ttl_pending_verification_expiry",
        )
        print("[db] MongoDB indexes ensured.")
    except Exception as exc:
        print(f"[db] Failed to ensure indexes: {str(exc)}")

