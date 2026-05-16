import json
import time

import jwt
import requests
from flask import Blueprint, current_app, jsonify, request

from .. import extensions
from ..models.fund import DEFAULT_FUND_CODES, SEED_FUNDS
from ..services.fund_fetcher import get_fund_info
from ..services.fund_service import (
    is_stale_timestamp,
    refresh_default_funds_if_needed,
)
from ..utils.response import check_db_status
from ..utils.security import require_update_api_key

funds_bp = Blueprint("funds", __name__)


@funds_bp.route("/api/update")
def update_funds():
    auth_error = require_update_api_key()
    if auth_error:
        return auth_error

    db_check = check_db_status()
    if db_check:
        return db_check

    updated = []
    failed = []

    print("=" * 50)
    print(f"Start updating funds, total {len(DEFAULT_FUND_CODES)} funds")
    print("=" * 50)

    for fund_code in DEFAULT_FUND_CODES:
        data = get_fund_info(fund_code)
        if data:
            try:
                extensions.collection.update_one(
                    {"fund_code": fund_code}, {"$set": data}, upsert=True
                )
                updated.append(fund_code)
                print(f"[{fund_code}] database updated successfully")
            except Exception as exc:
                failed.append({"code": fund_code, "reason": f"数据库更新失败: {str(exc)}"})
                print(f"[{fund_code}] 数据库更新失败: {str(exc)}")
        else:
            failed.append({"code": fund_code, "reason": "获取基金信息失败"})

        time.sleep(0.5)

    print("=" * 50)
    print(f"Update complete: success={len(updated)}, failed={len(failed)}")
    print("=" * 50)

    return jsonify(
        {
            "status": "success",
            "count": len(updated),
            "codes": updated,
            "failed": failed,
            "total": len(DEFAULT_FUND_CODES),
        }
    )


@funds_bp.route("/api/update_seeds")
def update_seed_funds():
    auth_error = require_update_api_key()
    if auth_error:
        return auth_error

    db_check = check_db_status()
    if db_check:
        return db_check

    updated = []
    failed = []

    print("=" * 50)
    print(f"Start updating seed funds, total {len(SEED_FUNDS)} funds")
    print("=" * 50)

    for seed in SEED_FUNDS:
        fund_code = seed["code"]
        print(f"[{fund_code}] fetching complete data...")

        data = get_fund_info(fund_code)
        if data:
            try:
                data["is_seed"] = True
                extensions.collection.replace_one({"fund_code": fund_code}, data, upsert=True)
                updated.append(fund_code)
                print(f"[{fund_code}] seed fund updated successfully")
            except Exception as exc:
                failed.append({"code": fund_code, "reason": f"数据库更新失败: {str(exc)}"})
                print(f"[{fund_code}] 数据库更新失败: {str(exc)}")
        else:
            failed.append({"code": fund_code, "reason": "获取基金信息失败"})

        time.sleep(0.5)

    print("=" * 50)
    print(f"Seed update complete: success={len(updated)}, failed={len(failed)}")
    print("=" * 50)

    return jsonify(
        {
            "status": "success",
            "count": len(updated),
            "codes": updated,
            "failed": failed,
            "total": len(SEED_FUNDS),
        }
    )


@funds_bp.route("/api/search_proxy")
@funds_bp.route("/api/funds/search")
def search_proxy():
    query = request.args.get("query", "")
    if not query:
        return jsonify({"error": "Query parameter is required"}), 400

    try:
        headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
        }
        search_url = (
            "http://fundsuggest.eastmoney.com/FundSearch/api/FundSearchAPI.ashx"
            f"?m=1&key={query}"
        )

        response = requests.get(search_url, headers=headers, timeout=5)
        response.encoding = "utf-8"

        if response.status_code == 200:
            data = json.loads(response.text)
            if "Datas" in data:
                results = []
                for item in data["Datas"][:20]:
                    results.append(
                        {
                            "fund_code": item.get("CODE", ""),
                            "fund_name": item.get("NAME", ""),
                            "fund_type": item.get("FUNDTYPE", ""),
                            "fund_family": item.get("FUNDNAME", ""),
                        }
                    )

                print(f"[search_proxy] query '{query}' returned {len(results)} results")
                return jsonify(results)
            return jsonify([])

        return jsonify({"error": "Failed to fetch search results"}), 500
    except requests.exceptions.Timeout:
        print(f"[search_proxy] query '{query}' timed out")
        return jsonify({"error": "Search request timeout"}), 504
    except Exception as exc:
        print(f"[search_proxy] query '{query}' failed: {str(exc)}")
        return jsonify({"error": f"Search failed: {str(exc)}"}), 500


@funds_bp.route("/api/funds")
def get_funds():
    db_check = check_db_status()
    if db_check:
        return db_check

    try:
        refresh_summary = refresh_default_funds_if_needed(force=False)
        if refresh_summary.get("refreshed"):
            print(
                "[auto_refresh] refreshed default funds: "
                f"updated={refresh_summary.get('updated', 0)}, "
                f"failed={len(refresh_summary.get('failed', []))}"
            )

        token = None
        if "Authorization" in request.headers:
            auth_header = request.headers["Authorization"]
            if auth_header.startswith("Bearer "):
                token = auth_header.split(" ")[1]

        watched_fund_codes = []
        if token:
            try:
                data = jwt.decode(token, current_app.config["JWT_SECRET"], algorithms=["HS256"])
                current_user_id = data["userId"]
                watched_items = list(
                    extensions.watchlist_collection.find(
                        {"userId": current_user_id},
                        {"fundCode": 1, "_id": 0},
                    )
                )
                watched_fund_codes = [item["fundCode"] for item in watched_items]
            except Exception:
                pass

        all_funds = list(
            extensions.collection.find({"fund_code": {"$in": DEFAULT_FUND_CODES}}, {"_id": 0})
        )
        for fund in all_funds:
            fund["is_watched"] = fund.get("fund_code") in watched_fund_codes

        sorted_funds = sorted(
            all_funds,
            key=lambda x: (
                not x.get("is_watched", False),
                not x.get("is_seed", False),
                x.get("fund_code", ""),
            ),
        )

        print(
            f"[funds] returning {len(sorted_funds)} funds, "
            f"{len(watched_fund_codes)} watched"
        )
        return jsonify(sorted_funds)
    except Exception as exc:
        print(f"获取基金列表失败: {str(exc)}")
        return jsonify({"status": "error", "message": f"获取基金数据失败: {str(exc)}"}), 500


@funds_bp.route("/api/fund/<fund_code>")
def get_fund(fund_code):
    db_check = check_db_status()
    if db_check:
        return db_check

    try:
        fund_data = extensions.collection.find_one({"fund_code": fund_code}, {"_id": 0})

        if fund_data and not is_stale_timestamp(fund_data.get("update_time")):
            print(f"[{fund_code}] served from cache")
            return jsonify(fund_data)

        if fund_data:
            print(f"[{fund_code}] cache is stale, refreshing from upstream...")
        else:
            print(f"[{fund_code}] not found in cache, fetching from upstream...")

        data = get_fund_info(fund_code)
        if data:
            should_persist = (fund_code in DEFAULT_FUND_CODES) or (fund_data is not None)
            if should_persist:
                extensions.collection.update_one(
                    {"fund_code": fund_code}, {"$set": data}, upsert=True
                )
                print(f"[{fund_code}] fetched from remote and stored successfully")
            else:
                print(f"[{fund_code}] fetched from remote (not persisted, non-default fund)")
            return jsonify(data)

        if fund_data:
            return jsonify(fund_data)

        return jsonify({"error": "Fund not found"}), 404
    except Exception as exc:
        print(f"Failed to fetch fund data: {str(exc)}")
        return jsonify({"error": f"Failed to fetch fund data: {str(exc)}"}), 500
