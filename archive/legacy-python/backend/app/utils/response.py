from flask import jsonify

from .. import extensions


def check_db_status():
    if extensions.db_error_message:
        return jsonify({"status": "error", "message": extensions.db_error_message}), 500

    if extensions.collection is None:
        return jsonify({"status": "error", "message": "Database not initialized"}), 500

    return None


def options_ok():
    return jsonify({"status": "ok"}), 200

