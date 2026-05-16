import time
import traceback

from flask import current_app

from .. import extensions
from ..models.fund import DEFAULT_FUND_CODES, SEED_FUNDS
from .fund_fetcher import get_fund_info, validate_fund_data


def is_stale_timestamp(ts, threshold_seconds=None):
    if threshold_seconds is None:
        threshold_seconds = current_app.config.get("AUTO_REFRESH_INTERVAL_SECONDS", 180)

    if not ts:
        return True

    try:
        ts_int = int(ts)
    except (TypeError, ValueError):
        return True

    return int(time.time()) - ts_int >= int(threshold_seconds)


def get_latest_update_time(codes=None):
    if extensions.collection is None:
        return 0

    query = {}
    if codes:
        query = {"fund_code": {"$in": codes}}

    latest_doc = extensions.collection.find_one(
        query, {"update_time": 1, "_id": 0}, sort=[("update_time", -1)]
    )
    if not latest_doc:
        return 0

    return int(latest_doc.get("update_time", 0) or 0)


def refresh_default_funds_if_needed(force=False):
    if extensions.collection is None:
        return {"refreshed": False, "reason": "db_not_ready", "updated": 0, "failed": []}

    auto_refresh_interval = current_app.config.get("AUTO_REFRESH_INTERVAL_SECONDS", 180)
    now = int(time.time())
    latest_ts = get_latest_update_time(DEFAULT_FUND_CODES)

    if not force and latest_ts and now - latest_ts < auto_refresh_interval:
        return {
            "refreshed": False,
            "reason": "fresh_enough",
            "updated": 0,
            "failed": [],
            "age_seconds": now - latest_ts,
        }

    if not extensions.refresh_lock.acquire(blocking=False):
        return {"refreshed": False, "reason": "refresh_in_progress", "updated": 0, "failed": []}

    updated = 0
    failed = []

    try:
        for fund_code in DEFAULT_FUND_CODES:
            data = get_fund_info(fund_code)
            if not data:
                failed.append({"code": fund_code, "reason": "fetch_failed"})
                continue

            try:
                extensions.collection.update_one(
                    {"fund_code": fund_code}, {"$set": data}, upsert=True
                )
                updated += 1
            except Exception as exc:
                failed.append({"code": fund_code, "reason": f"db_update_failed: {str(exc)}"})

        return {
            "refreshed": True,
            "reason": "updated",
            "updated": updated,
            "failed": failed,
        }
    finally:
        extensions.refresh_lock.release()


def init_seed_funds():
    if extensions.collection is None:
        print("[seed] Database not initialized, skip seed initialization.")
        return

    try:
        print("[seed] Cleaning old seed markers before initialization...")
        result = extensions.collection.delete_many({"is_seed": True})
        print(f"[seed] Removed {result.deleted_count} historical seed-marked docs")

        initialized_count = 0
        failed_count = 0

        for seed in SEED_FUNDS:
            print(f"[seed] Initializing {seed['code']} ({seed['name']}) ...")
            fund_data = get_fund_info(seed["code"])
            is_valid, reason = validate_fund_data(fund_data, seed["code"], seed["name"])

            if is_valid:
                fund_data["is_seed"] = True
                existing = extensions.collection.find_one({"fund_code": seed["code"]})

                if existing:
                    existing_valid, _ = validate_fund_data(existing, seed["code"])
                    if not existing_valid:
                        extensions.collection.replace_one(
                            {"fund_code": seed["code"]},
                            fund_data,
                        )
                        print(f"[seed] {seed['code']} replaced invalid existing data.")
                    else:
                        extensions.collection.update_one(
                            {"fund_code": seed["code"]},
                            {"$set": {"is_seed": True}},
                        )
                        print(f"[seed] {seed['code']} already valid; marked as seed.")
                else:
                    extensions.collection.insert_one(fund_data)
                    print(
                        f"[seed] {seed['code']} inserted: {fund_data.get('fund_name')}"
                    )

                initialized_count += 1
            else:
                failed_count += 1
                print(f"[seed] {seed['code']} validation failed: {reason}")

                existing = extensions.collection.find_one({"fund_code": seed["code"]})
                if existing:
                    extensions.collection.update_one(
                        {"fund_code": seed["code"]},
                        {"$set": {"is_seed": True}},
                    )
                    print(f"[seed] {seed['code']} used existing data and marked seed.")
                else:
                    extensions.collection.insert_one(
                        {
                            "fund_code": seed["code"],
                            "fund_name": seed["name"],
                            "is_seed": True,
                            "update_time": int(time.time()),
                            "week_growth": 0.0,
                            "month_growth": 0.0,
                            "year_growth": 0.0,
                            "day_growth": 0.0,
                            "net_value": 0.0,
                        }
                    )
                    print(f"[seed] {seed['code']} fallback data inserted.")

            time.sleep(0.5)

        print(
            f"[seed] Initialization complete: success={initialized_count}, "
            f"failed={failed_count}"
        )
    except Exception as exc:
        print(f"[seed] Initialization error: {str(exc)}")
        print(traceback.format_exc())

