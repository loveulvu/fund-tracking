import os
import time
import threading
import requests
import traceback
import json
import re
from flask import Flask, jsonify, request
from flask_cors import CORS
from pymongo import MongoClient
from functools import wraps
import jwt
from datetime import datetime, timedelta, timezone
from werkzeug.security import generate_password_hash, check_password_hash

app = Flask(__name__)
CORS(app, resources={r"/api/*": {"origins": ["https://fundtracking.online", "https://www.fundtracking.online", "http://localhost:3000"]}}, supports_credentials=True)

# 1. 鐜鍙橀噺璇诲彇
MONGO_URI = os.environ.get("MONGO_URI")
JWT_SECRET = os.environ.get("JWT_SECRET", "fund_tracking_secret_key_2026")
EMAIL_SENDER = os.environ.get("EMAIL_SENDER")
EMAIL_PASSWORD = os.environ.get("EMAIL_PASSWORD")
UPDATE_API_KEY = os.environ.get("UPDATE_API_KEY")
AUTO_REFRESH_INTERVAL_SECONDS = int(os.environ.get("AUTO_REFRESH_INTERVAL_SECONDS", "180"))
APP_VERSION = os.environ.get("APP_VERSION", "dev")
APP_COMMIT_SHA = (
    os.environ.get("GIT_COMMIT")
    or os.environ.get("RAILWAY_GIT_COMMIT_SHA")
    or os.environ.get("SOURCE_VERSION")
    or ""
)
APP_BUILT_AT = os.environ.get("APP_BUILT_AT")
refresh_lock = threading.Lock()

if "JWT_SECRET" not in os.environ:
    print("[security] WARNING: JWT_SECRET is using fallback value. Please set JWT_SECRET in Railway.")


def get_version_payload():
    short_commit = APP_COMMIT_SHA[:7] if APP_COMMIT_SHA else None
    return {
        "service": "fund-tracking-api",
        "version": APP_VERSION,
        "commit": short_commit,
        "commit_full": APP_COMMIT_SHA or None,
        "built_at": APP_BUILT_AT,
        "server_time": int(time.time())
    }


def require_update_api_key():
    """
    Optional guard for update endpoints.
    If UPDATE_API_KEY is configured, requests must provide matching key.
    """
    if not UPDATE_API_KEY:
        return None
    provided = request.headers.get("X-Update-Key") or request.args.get("key")
    if provided != UPDATE_API_KEY:
        return jsonify({"error": "Unauthorized"}), 401
    return None


def hash_password(password):
    return generate_password_hash(password)


def verify_password(stored_password, candidate_password):
    """
    Supports both hashed and legacy plaintext passwords for backward compatibility.
    """
    if not stored_password or not candidate_password:
        return False
    try:
        if check_password_hash(stored_password, candidate_password):
            return True
    except Exception:
        pass
    return stored_password == candidate_password

def send_verification_email(email, code):
    """
    浣跨敤 Resend HTTP API 鍙戦€侀獙璇佺爜閭欢
    - 杩斿洖 True/False
    """
    if not EMAIL_PASSWORD or not EMAIL_SENDER:
        print(f"[閭欢] 鈿狅笍 閭欢鏈嶅姟閰嶇疆涓嶅畬锟? EMAIL_SENDER 锟?EMAIL_PASSWORD 缂哄け")
        return False
    
    try:
        html_content = f"""
        <div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
            <h2 style="color: #333;">楠岃瘉锟?/h2>
            <p style="font-size: 16px; color: #666;">鎮ㄧ殑楠岃瘉鐮佹槸锟?/p>
            <div style="background: #f5f5f5; padding: 20px; text-align: center; font-size: 32px; font-weight: bold; letter-spacing: 5px; margin: 20px 0;">
                {code}
            </div>
            <p style="font-size: 14px; color: #999;">楠岃瘉鐮佹湁鏁堟湡锟?0鍒嗛挓锛岃灏藉揩浣跨敤锟?/p>
            <p style="font-size: 14px; color: #999;">濡傛灉鎮ㄦ病鏈夎姹傛楠岃瘉鐮侊紝璇峰拷鐣ユ閭欢锟?/p>
        </div>
        """
        
        response = requests.post(
            "https://api.resend.com/emails",
            headers={
                "Authorization": f"Bearer {EMAIL_PASSWORD}",
                "Content-Type": "application/json"
            },
            json={
                "from": EMAIL_SENDER,
                "to": [email],
                "subject": "鎮ㄧ殑楠岃瘉锟?- Fund Tracking",
                "html": html_content
            },
            timeout=10
        )
        
        if 200 <= response.status_code < 300:
            result = response.json()
            print(f"[閭欢] 锟?楠岃瘉鐮佸凡鍙戦€佽嚦 {email}, Resend ID: {result.get('id')}")
            return True
        else:
            print(f"[閭欢] 锟?鍙戦€佸け锟? HTTP {response.status_code} - {response.text}")
            return False
    except Exception as e:
        print(f"[閭欢] 锟?褰诲簳澶辫触: {str(e)}")
        return False

# 2. 鍏ㄥ眬鏁版嵁搴撳璞″崰锟?
db = None
collection = None
watchlist_collection = None
users_collection = None
pending_users_collection = None
db_error_message = None

# 3. 鏌旀€ц繛鎺ラ€昏緫锛氬嵆浣垮け璐ヤ篃涓嶈Е鍙戣繘绋嬪穿婧冿紝淇濊瘉 Railway 鑳介『鍒╁惎锟?
if not MONGO_URI:
    db_error_message = "CRITICAL: MONGO_URI is missing. Please set it in Railway Variables."
    print(db_error_message)
else:
    try:
        client = MongoClient(MONGO_URI, serverSelectionTimeoutMS=5000)
        db = client['fund_tracking']
        collection = db['fund_data']
        watchlist_collection = db['watchlists']
        users_collection = db['users']
        pending_users_collection = db['pending_users']
        client.admin.command('ping')
        print(f"Flask connecting to database: {db.name}")
        print("MongoDB connected.")
    except Exception as e:
        db_error_message = f"MongoDB 杩炴帴澶辫触: {str(e)}"
        print(db_error_message)
        print("瀹屾暣閿欒鍫嗘爤:")
        print(traceback.format_exc())

def ensure_indexes():
    if collection is None:
        return
    try:
        collection.create_index("fund_code", unique=True, name="uniq_fund_code")
        watchlist_collection.create_index(
            [("userId", 1), ("fundCode", 1)],
            unique=True,
            name="uniq_watchlist_user_fund"
        )
        users_collection.create_index("email", unique=True, name="uniq_user_email")
        pending_users_collection.create_index("email", unique=True, name="uniq_pending_email")
        pending_users_collection.create_index(
            "verification_code_expires",
            expireAfterSeconds=0,
            name="ttl_pending_verification_expiry"
        )
        print("[db] MongoDB indexes ensured.")
    except Exception as e:
        print(f"[db] Failed to ensure indexes: {str(e)}")


SEED_FUNDS = [
    {"code": "008540", "name": "鍗庡绉戞妧鍒涙柊A"},
    {"code": "012414", "name": "鍗庡涓瘉鏂拌兘婧愭苯杞TF鑱旀帴A"},
    {"code": "001887", "name": "鍗庡娌繁300鎸囨暟澧炲己A"},
    {"code": "005303", "name": "鍗庡涓瘉500鎸囨暟澧炲己A"},
    {"code": "588000", "name": "鍗庡涓婅瘉绉戝垱锟?0鎴愪唤ETF"},
    {"code": "161128", "name": "鏄撴柟杈句腑璇佹捣澶栦腑鍥戒簰鑱旂綉50ETF"},
    {"code": "510300", "name": "鍗庢嘲鏌忕憺娌繁300ETF"},
    {"code": "161725", "name": "鎷涘晢涓瘉鐧介厭鎸囨暟(LOF)A"},
    {"code": "001607", "name": "涓鍖荤枟鍋ュ悍娣峰悎A"},
    {"code": "004243", "name": "Fund 004243"}
]

DEFAULT_FUND_CODES = [fund["code"] for fund in SEED_FUNDS]


def is_stale_timestamp(ts, threshold_seconds=AUTO_REFRESH_INTERVAL_SECONDS):
    if not ts:
        return True
    try:
        ts_int = int(ts)
    except (TypeError, ValueError):
        return True
    return int(time.time()) - ts_int >= threshold_seconds


def get_latest_update_time(codes=None):
    if collection is None:
        return 0

    query = {}
    if codes:
        query = {"fund_code": {"$in": codes}}

    latest_doc = collection.find_one(
        query,
        {"update_time": 1, "_id": 0},
        sort=[("update_time", -1)]
    )
    if not latest_doc:
        return 0
    return int(latest_doc.get("update_time", 0) or 0)


def refresh_default_funds_if_needed(force=False):
    """
    Refresh default funds when cache is stale.
    Returns summary dict for logging/diagnostics.
    """
    if collection is None:
        return {"refreshed": False, "reason": "db_not_ready", "updated": 0, "failed": []}

    now = int(time.time())
    latest_ts = get_latest_update_time(DEFAULT_FUND_CODES)

    if not force and latest_ts and now - latest_ts < AUTO_REFRESH_INTERVAL_SECONDS:
        return {
            "refreshed": False,
            "reason": "fresh_enough",
            "updated": 0,
            "failed": [],
            "age_seconds": now - latest_ts
        }

    if not refresh_lock.acquire(blocking=False):
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
                collection.update_one({"fund_code": fund_code}, {"$set": data}, upsert=True)
                updated += 1
            except Exception as e:
                failed.append({"code": fund_code, "reason": f"db_update_failed: {str(e)}"})

        return {
            "refreshed": True,
            "reason": "updated",
            "updated": updated,
            "failed": failed
        }
    finally:
        refresh_lock.release()

def validate_fund_data(fund_data, expected_code, expected_name_hint=None):
    """
    涓ユ牸楠岃瘉鍩洪噾鏁版嵁鐨勬湁鏁堬拷?
    杩斿洖: (is_valid, reason)
    """
    if not fund_data:
        return False, "鏁版嵁涓虹┖"
    
    fund_name = fund_data.get('fund_name', '')
    
    if fund_name == '鏈煡' or not fund_name:
        return False, "Fund name is empty or unknown"
    
    if fund_data.get('fund_code') != expected_code:
        return False, f"鍩洪噾浠ｇ爜涓嶅尮锟? 鏈熸湜 {expected_code}, 瀹為檯 {fund_data.get('fund_code')}"
    
    critical_fields = ['net_value', 'day_growth', 'week_growth', 'month_growth']
    missing_fields = [f for f in critical_fields if f not in fund_data]
    if missing_fields:
        return False, f"缂哄皯鍏抽敭瀛楁: {missing_fields}"
    
    all_zero = all(
        fund_data.get(field, 0) == 0 
        for field in ['week_growth', 'month_growth', 'year_growth']
    )
    
    if all_zero and fund_name == '鏈煡':
        return False, "鎵€鏈夋定骞呬负0涓斿熀閲戝悕绉版湭鐭ワ紝鏁版嵁鍙兘鏃犳晥"
    
    return True, "鏁版嵁鏈夋晥"

def init_seed_funds():
    """
    鍒濆鍖栫瀛愬熀閲戯紝鏍囪锟?is_seed=True锛屽苟鑾峰彇瀹屾暣鏁版嵁
    鍖呭惈涓ユ牸鐨勬暟鎹獙璇侊紝闃叉閿欒鏁版嵁瑕嗙洊姝ｇ‘鏁版嵁
    """
    if collection is None:
        print("[seed] Database not initialized, skip seed initialization.")
        return
    
    try:
        print("[绉嶅瓙鍩洪噾] 寮€濮嬫竻鐞嗘棫鐨勭瀛愬熀閲戞暟锟?..")
        result = collection.delete_many({"is_seed": True})
        print(f"[绉嶅瓙鍩洪噾] 宸插垹锟?{result.deleted_count} 鏉℃棫鏁版嵁锛屽噯澶囬噸鏂板垵濮嬪寲")
        
        initialized_count = 0
        failed_count = 0
        
        for seed in SEED_FUNDS:
            print(f"[绉嶅瓙鍩洪噾] 姝ｅ湪鑾峰彇 {seed['code']} ({seed['name']}) 鐨勬暟锟?..")
            fund_data = get_fund_info(seed["code"])
            
            is_valid, reason = validate_fund_data(fund_data, seed["code"], seed["name"])
            
            if is_valid:
                fund_data["is_seed"] = True
                
                existing = collection.find_one({"fund_code": seed["code"]})
                
                if existing:
                    existing_valid, _ = validate_fund_data(existing, seed["code"])
                    
                    if not existing_valid:
                        collection.replace_one(
                            {"fund_code": seed["code"]},
                            fund_data
                        )
                        print(f"[seed] {seed['code']} replaced invalid existing data.")
                    else:
                        collection.update_one(
                            {"fund_code": seed["code"]},
                            {"$set": {"is_seed": True}}
                        )
                        print(f"[绉嶅瓙鍩洪噾] 锟?{seed['code']} 宸叉湁鏈夋晥鏁版嵁锛屼粎鏇存柊鏍囪")
                else:
                    collection.insert_one(fund_data)
                    print(f"[绉嶅瓙鍩洪噾] 锟?{seed['code']} 鏁版嵁鎻掑叆鎴愬姛: {fund_data.get('fund_name')}")
                
                initialized_count += 1
            else:
                failed_count += 1
                print(f"[绉嶅瓙鍩洪噾] 锟?{seed['code']} 鏁版嵁楠岃瘉澶辫触: {reason}")
                
                existing = collection.find_one({"fund_code": seed["code"]})
                if existing:
                    collection.update_one(
                        {"fund_code": seed["code"]},
                        {"$set": {"is_seed": True}}
                    )
                    print(f"[绉嶅瓙鍩洪噾] 鈿狅笍 {seed['code']} 淇濈暀鐜版湁鏁版嵁锛屼粎鏇存柊鏍囪")
                else:
                    collection.insert_one({
                        "fund_code": seed["code"],
                        "fund_name": seed["name"],
                        "is_seed": True,
                        "update_time": int(time.time()),
                        "week_growth": 0.0,
                        "month_growth": 0.0,
                        "year_growth": 0.0,
                        "day_growth": 0.0,
                        "net_value": 0.0
                    })
                    print(f"[绉嶅瓙鍩洪噾] 鈿狅笍 {seed['code']} 浣跨敤鍩虹鏁版嵁妯℃澘")
            
            time.sleep(0.5)
        
        print(f"[seed] Initialization complete: success={initialized_count}, failed={failed_count}")
    except Exception as e:
        print(f"[绉嶅瓙鍩洪噾] 鍒濆鍖栧け锟? {str(e)}")
        print(traceback.format_exc())

def get_fund_info(fund_code):
    """
    锟?API 椹卞姩鐨勫熀閲戜俊鎭幏鍙栧嚱锟?
    鏁版嵁婧愶細
    1. fundgz.1234567.com.cn - 鑾峰彇瀹炴椂浼板€煎拰鏃ユ定锟?
    2. fundmobapi.eastmoney.com - 鑾峰彇鍩洪噾璇︽儏鍜屽巻鍙叉定锟?
    """
    headers_web = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
    headers_mobile = {
        'User-Agent': 'Dalvik/2.1.0 (Linux; U; Android 10; SM-G981B Build/QP1A.190711.020)',
        'Host': 'fundmobapi.eastmoney.com',
        'Connection': 'Keep-Alive',
    }
    
    print(f"[{fund_code}] 寮€濮嬭幏鍙栧熀閲戜俊鎭紙绾疉PI妯″紡锟?..")
    
    data_item = {
        "fund_code": fund_code,
        "update_time": int(time.time())
    }
    
    try:
        api_url = f"https://fundgz.1234567.com.cn/js/{fund_code}.js"
        response = requests.get(api_url, headers=headers_web, timeout=5)
        response.encoding = 'utf-8'
        
        if response.status_code == 200 and 'jsonpgz' in response.text:
            json_str = response.text.replace('jsonpgz(', '').replace(');', '')
            fund_data = json.loads(json_str)
            
            data_item['fund_name'] = fund_data.get('name', '鏈煡')
            data_item['net_value'] = float(fund_data.get('dwjz', 0)) if fund_data.get('dwjz') else 0.0
            data_item['net_value_date'] = fund_data.get('jzrq', '')
            data_item['day_growth'] = float(fund_data.get('gszzl', 0)) if fund_data.get('gszzl') else 0.0
            
            print(f"[{fund_code}] fundgz API: {data_item.get('fund_name')} | 鍑€锟? {data_item.get('net_value')} | 鏃ユ定锟? {data_item.get('day_growth')}%")
        else:
            print(f"[{fund_code}] fundgz API 杩斿洖寮傚父: HTTP {response.status_code}")
    except Exception as e:
        print(f"[{fund_code}] fundgz API 鑾峰彇澶辫触: {str(e)}")
    
    try:
        base_info_url = f"http://fundmobapi.eastmoney.com/FundMNewApi/FundMNBaseInfo?FCODE={fund_code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0"
        response = requests.get(base_info_url, headers=headers_mobile, timeout=5)
        
        if response.status_code == 200:
            result = response.json()
            
            if result.get('Success') and result.get('Datas'):
                fund_info = result['Datas']
                
                if 'fund_name' not in data_item or data_item.get('fund_name') == '鏈煡':
                    data_item['fund_name'] = fund_info.get('SHORTNAME', '鏈煡')
                
                if 'net_value' not in data_item or data_item.get('net_value') == 0:
                    dwjz = fund_info.get('DWJZ')
                    if dwjz:
                        data_item['net_value'] = float(dwjz)
                
                data_item['fund_type'] = fund_info.get('FTYPE', '')
                data_item['fund_company'] = fund_info.get('JJGS', '')
                data_item['fund_manager'] = fund_info.get('JJJL', '')
                data_item['fund_scale'] = fund_info.get('TOTALSCALE', '')
                
                syl_z = fund_info.get('SYL_Z')
                syl_y = fund_info.get('SYL_Y')
                syl_3y = fund_info.get('SYL_3Y')
                syl_6y = fund_info.get('SYL_6Y')
                syl_1n = fund_info.get('SYL_1N')
                
                if syl_z is not None:
                    data_item['week_growth'] = float(syl_z)
                if syl_y is not None:
                    data_item['month_growth'] = float(syl_y)
                if syl_3y is not None:
                    data_item['three_month_growth'] = float(syl_3y)
                if syl_6y is not None:
                    data_item['six_month_growth'] = float(syl_6y)
                if syl_1n is not None:
                    data_item['year_growth'] = float(syl_1n)
                
                print(
                    f"[{fund_code}] FundMNBaseInfo API: "
                    f"week {data_item.get('week_growth', 'N/A')}% | "
                    f"month {data_item.get('month_growth', 'N/A')}% | "
                    f"year {data_item.get('year_growth', 'N/A')}%"
                )
            else:
                print(f"[{fund_code}] FundMNBaseInfo API 鏃犳暟锟? {result.get('ErrMsg', 'Unknown error')}")
        else:
            print(f"[{fund_code}] FundMNBaseInfo API HTTP {response.status_code}")
    except Exception as e:
        print(f"[{fund_code}] FundMNBaseInfo API 鑾峰彇澶辫触: {str(e)}")
    
    growth_fields = ['week_growth', 'month_growth', 'three_month_growth', 'six_month_growth', 'year_growth', 'three_year_growth']
    for field in growth_fields:
        if field not in data_item:
            data_item[field] = 0.0
            print(f"[{fund_code}] {field} 缂哄け锛屼娇鐢ㄩ粯璁わ拷?0.0")
    
    if 'fund_name' not in data_item:
        data_item['fund_name'] = '鏈煡'
    if 'net_value' not in data_item:
        data_item['net_value'] = 0.0
    if 'day_growth' not in data_item:
        data_item['day_growth'] = 0.0
    
    print(f"[{fund_code}] 锟?鏁版嵁鑾峰彇瀹屾垚: {data_item.get('fund_name')}")
    
    return data_item

ensure_indexes()
init_seed_funds()

def check_db_status():
    if db_error_message:
        return jsonify({"status": "error", "message": db_error_message}), 500
    if collection is None:
        return jsonify({"status": "error", "message": "Database not initialized"}), 500
    return None

def token_required(f):
    @wraps(f)
    def decorated(*args, **kwargs):
        if request.method == 'OPTIONS':
            return jsonify({'status': 'ok'}), 200
        
        token = None
        
        if 'Authorization' in request.headers:
            auth_header = request.headers['Authorization']
            if auth_header.startswith('Bearer '):
                token = auth_header.split(' ')[1]
        
        if not token:
            return jsonify({'error': 'Token is missing'}), 401
        
        try:
            data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
            current_user_id = data['userId']
        except jwt.ExpiredSignatureError:
            return jsonify({'error': 'Token has expired'}), 401
        except jwt.InvalidTokenError:
            return jsonify({'error': 'Invalid token'}), 401
        
        return f(current_user_id, *args, **kwargs)
    
    return decorated

@app.route('/api/update')
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
    
    for c in DEFAULT_FUND_CODES:
        data = get_fund_info(c)
        if data:
            try:
                collection.update_one({"fund_code": c}, {"$set": data}, upsert=True)
                updated.append(c)
                print(f"[{c}] database updated successfully")
            except Exception as e:
                failed.append({"code": c, "reason": f"鏁版嵁搴撴洿鏂板け锟? {str(e)}"})
                print(f"[{c}] 锟?鏁版嵁搴撴洿鏂板け锟? {str(e)}")
        else:
            failed.append({"code": c, "reason": "鑾峰彇鍩洪噾淇℃伅澶辫触"})
        
        time.sleep(0.5)
    
    print("=" * 50)
    print(f"Update complete: success={len(updated)}, failed={len(failed)}")
    print("=" * 50)
    
    return jsonify({
        "status": "success",
        "count": len(updated),
        "codes": updated,
        "failed": failed,
        "total": len(DEFAULT_FUND_CODES)
    })

@app.route('/api/update_seeds')
def update_seed_funds():
    auth_error = require_update_api_key()
    if auth_error:
        return auth_error

    """
    鏇存柊鎵€鏈夌瀛愬熀閲戠殑瀹屾暣鏁版嵁
    """
    db_check = check_db_status()
    if db_check: return db_check
    
    updated = []
    failed = []
    
    print("=" * 50)
    print(f"Start updating seed funds, total {len(SEED_FUNDS)} funds")
    print("=" * 50)
    
    for seed in SEED_FUNDS:
        fund_code = seed["code"]
        print(f"[{fund_code}] 姝ｅ湪鑾峰彇瀹屾暣鏁版嵁...")
        
        data = get_fund_info(fund_code)
        if data:
            try:
                data["is_seed"] = True
                collection.replace_one({"fund_code": fund_code}, data, upsert=True)
                updated.append(fund_code)
                print(f"[{fund_code}] seed fund updated successfully")
            except Exception as e:
                failed.append({"code": fund_code, "reason": f"鏁版嵁搴撴洿鏂板け锟? {str(e)}"})
                print(f"[{fund_code}] 锟?鏁版嵁搴撴洿鏂板け锟? {str(e)}")
        else:
            failed.append({"code": fund_code, "reason": "鑾峰彇鍩洪噾淇℃伅澶辫触"})
        
        time.sleep(0.5)
    
    print("=" * 50)
    print(f"Seed update complete: success={len(updated)}, failed={len(failed)}")
    print("=" * 50)
    
    return jsonify({
        "status": "success",
        "count": len(updated),
        "codes": updated,
        "failed": failed,
        "total": len(SEED_FUNDS)
    })

@app.route('/api/search_proxy')
@app.route('/api/funds/search')
def search_proxy():
    """
    鎼滅储浠ｇ悊鎺ュ彛锛氳皟鐢ㄥぉ澶╁熀閲戠綉鐨勬悳绱PI
    """
    query = request.args.get('query', '')
    
    if not query:
        return jsonify({"error": "Query parameter is required"}), 400
    
    try:
        headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
        
        search_url = f"http://fundsuggest.eastmoney.com/FundSearch/api/FundSearchAPI.ashx?m=1&key={query}"
        
        response = requests.get(search_url, headers=headers, timeout=5)
        response.encoding = 'utf-8'
        
        if response.status_code == 200:
            import json
            data = json.loads(response.text)
            
            if 'Datas' in data:
                results = []
                for item in data['Datas'][:20]:
                    results.append({
                        'fund_code': item.get('CODE', ''),
                        'fund_name': item.get('NAME', ''),
                        'fund_type': item.get('FUNDTYPE', ''),
                        'fund_family': item.get('FUNDNAME', '')
                    })
                
                print(f"[search_proxy] query '{query}' returned {len(results)} results")
                return jsonify(results)
            else:
                return jsonify([])
        else:
            return jsonify({"error": "Failed to fetch search results"}), 500
            
    except requests.exceptions.Timeout:
        print(f"[鎼滅储浠ｇ悊] 鏌ヨ '{query}' 瓒呮椂")
        return jsonify({"error": "Search request timeout"}), 504
    except Exception as e:
        print(f"[鎼滅储浠ｇ悊] 鏌ヨ '{query}' 澶辫触: {str(e)}")
        return jsonify({"error": f"Search failed: {str(e)}"}), 500

@app.route('/api/funds')
def get_funds():
    db_check = check_db_status()
    if db_check: return db_check
    
    try:
        refresh_summary = refresh_default_funds_if_needed(force=False)
        if refresh_summary.get("refreshed"):
            print(
                f"[auto_refresh] refreshed default funds: "
                f"updated={refresh_summary.get('updated', 0)}, "
                f"failed={len(refresh_summary.get('failed', []))}"
            )

        token = None
        if 'Authorization' in request.headers:
            auth_header = request.headers['Authorization']
            if auth_header.startswith('Bearer '):
                token = auth_header.split(' ')[1]
        
        watched_fund_codes = []
        if token:
            try:
                data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
                current_user_id = data['userId']
                watched_items = list(watchlist_collection.find({'userId': current_user_id}, {'fundCode': 1, '_id': 0}))
                watched_fund_codes = [item['fundCode'] for item in watched_items]
            except:
                pass
        
        # Only return default seed funds for the main funds list.
        # This prevents ad-hoc searched funds from polluting the default list count.
        all_funds = list(
            collection.find({"fund_code": {"$in": DEFAULT_FUND_CODES}}, {"_id": 0})
        )
        
        for fund in all_funds:
            if fund.get('fund_code') in watched_fund_codes:
                fund['is_watched'] = True
            else:
                fund['is_watched'] = False
        
        sorted_funds = sorted(all_funds, key=lambda x: (
            not x.get('is_watched', False),
            not x.get('is_seed', False),
            x.get('fund_code', '')
        ))
        
        print(f"[鍩洪噾鍒楄〃] 杩斿洖 {len(sorted_funds)} 涓熀閲戯紝鍏朵腑 {len(watched_fund_codes)} 涓凡鍏虫敞")
        return jsonify(sorted_funds)
    except Exception as e:
        print(f"鑾峰彇鍩洪噾鍒楄〃澶辫触: {str(e)}")
        return jsonify({"status": "error", "message": f"鑾峰彇鏁版嵁澶辫触: {str(e)}"}), 500

@app.route('/api/fund/<fund_code>')
def get_fund(fund_code):
    db_check = check_db_status()
    if db_check:
        return db_check

    try:
        fund_data = collection.find_one({"fund_code": fund_code}, {"_id": 0})

        if fund_data and not is_stale_timestamp(fund_data.get("update_time")):
            print(f"[{fund_code}] served from cache")
            return jsonify(fund_data)

        if fund_data:
            print(f"[{fund_code}] cache is stale, refreshing from upstream...")
        else:
            print(f"[{fund_code}] not found in cache, fetching from upstream...")

        data = get_fund_info(fund_code)

        if data:
            # Persist only default funds, or refresh an already-cached fund.
            should_persist = (fund_code in DEFAULT_FUND_CODES) or (fund_data is not None)
            if should_persist:
                collection.update_one({"fund_code": fund_code}, {"$set": data}, upsert=True)
                print(f"[{fund_code}] fetched from remote and stored successfully")
            else:
                print(f"[{fund_code}] fetched from remote (not persisted, non-default fund)")
            return jsonify(data)

        if fund_data:
            return jsonify(fund_data)

        return jsonify({"error": "Fund not found"}), 404

    except Exception as e:
        print(f"Failed to fetch fund data: {str(e)}")
        return jsonify({"error": f"Failed to fetch fund data: {str(e)}"}), 500
@app.route('/api/watchlist', methods=['GET', 'OPTIONS'])
def get_watchlist():
    print(f"[DEBUG] get_watchlist 琚皟鐢紝鏂规硶: {request.method}")
    if request.method == 'OPTIONS':
        print("[DEBUG] OPTIONS 璇锋眰锛岀洿鎺ヨ繑锟?200")
        return jsonify({'status': 'ok'}), 200
    
    db_check = check_db_status()
    if db_check:
        print(f"[DEBUG] 鏁版嵁搴撴鏌ュけ锟? {db_check}")
        return db_check
    
    token = None
    if 'Authorization' in request.headers:
        auth_header = request.headers['Authorization']
        print(f"[DEBUG] Authorization header: {auth_header[:20]}..." if len(auth_header) > 20 else f"[DEBUG] Authorization header: {auth_header}")
        if auth_header.startswith('Bearer '):
            token = auth_header.split(' ')[1]
    
    if not token:
        print("[DEBUG] Token 缂哄け")
        return jsonify({'error': 'Token is missing'}), 401
    
    try:
        data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
        current_user_id = data['userId']
        print(f"[DEBUG] Token 瑙ｇ爜鎴愬姛锛岀敤鎴稩D: {current_user_id}")
    except jwt.ExpiredSignatureError:
        print("[DEBUG] Token expired")
        return jsonify({'error': 'Token has expired'}), 401
    except jwt.InvalidTokenError as e:
        print(f"[DEBUG] Token 鏃犳晥: {str(e)}")
        return jsonify({'error': 'Invalid token'}), 401
    
    try:
        watchlist = list(watchlist_collection.find({'userId': current_user_id}, {'_id': 0}))
        print(f"[watchlist] user {current_user_id} has {len(watchlist)} items")
        return jsonify(watchlist)
    except Exception as e:
        print(f"鑾峰彇鍏虫敞鍒楄〃澶辫触: {str(e)}")
        return jsonify({'error': 'Failed to fetch watchlist'}), 500

@app.route('/api/watchlist', methods=['POST', 'OPTIONS'])
def add_to_watchlist():
    print(f"[DEBUG] add_to_watchlist 琚皟鐢紝鏂规硶: {request.method}")
    if request.method == 'OPTIONS':
        print("[DEBUG] OPTIONS 璇锋眰锛岀洿鎺ヨ繑锟?200")
        return jsonify({'status': 'ok'}), 200
    
    db_check = check_db_status()
    if db_check:
        print(f"[DEBUG] 鏁版嵁搴撴鏌ュけ锟? {db_check}")
        return db_check
    
    token = None
    if 'Authorization' in request.headers:
        auth_header = request.headers['Authorization']
        print(f"[DEBUG] Authorization header: {auth_header[:20]}..." if len(auth_header) > 20 else f"[DEBUG] Authorization header: {auth_header}")
        if auth_header.startswith('Bearer '):
            token = auth_header.split(' ')[1]
    
    if not token:
        print("[DEBUG] Token 缂哄け")
        return jsonify({'error': 'Token is missing'}), 401
    
    try:
        data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
        current_user_id = data['userId']
        print(f"[DEBUG] Token 瑙ｇ爜鎴愬姛锛岀敤鎴稩D: {current_user_id}")
    except jwt.ExpiredSignatureError:
        print("[DEBUG] Token expired")
        return jsonify({'error': 'Token has expired'}), 401
    except jwt.InvalidTokenError as e:
        print(f"[DEBUG] Token 鏃犳晥: {str(e)}")
        return jsonify({'error': 'Invalid token'}), 401
    
    try:
        req_data = request.get_json()
        print(f"[DEBUG] 璇锋眰鏁版嵁: {req_data}")
        fund_code = req_data.get('fundCode')
        fund_name = req_data.get('fundName')
        threshold = req_data.get('alertThreshold', 5)
        
        if not fund_code or not fund_name:
            print(f"[DEBUG] 缂哄皯蹇呰鍙傛暟: fundCode={fund_code}, fundName={fund_name}")
            return jsonify({'error': 'fundCode and fundName are required'}), 400
        
        existing = watchlist_collection.find_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        if existing:
            print(f"[DEBUG] Fund {fund_code} already in watchlist")
            return jsonify({'error': 'Fund already in watchlist'}), 400
        
        watchlist_item = {
            'userId': current_user_id,
            'fundCode': fund_code,
            'fundName': fund_name,
            'alertThreshold': threshold,
            'addedAt': datetime.now(timezone.utc)
        }
        
        watchlist_collection.insert_one(watchlist_item)
        watchlist_item.pop('_id', None)
        print(f"[娣诲姞鍏虫敞] 鐢ㄦ埛 {current_user_id} 娣诲姞 {fund_code}")
        return jsonify(watchlist_item), 201
        
    except Exception as e:
        print(f"娣诲姞鍏虫敞澶辫触: {str(e)}")
        return jsonify({'error': 'Failed to add to watchlist'}), 500

@app.route('/api/watchlist/<fund_code>', methods=['DELETE', 'OPTIONS'])
def remove_from_watchlist(fund_code):
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    db_check = check_db_status()
    if db_check:
        return db_check
    
    token = None
    if 'Authorization' in request.headers:
        auth_header = request.headers['Authorization']
        if auth_header.startswith('Bearer '):
            token = auth_header.split(' ')[1]
    
    if not token:
        return jsonify({'error': 'Token is missing'}), 401
    
    try:
        data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
        current_user_id = data['userId']
    except jwt.ExpiredSignatureError:
        return jsonify({'error': 'Token has expired'}), 401
    except jwt.InvalidTokenError:
        return jsonify({'error': 'Invalid token'}), 401
    
    try:
        result = watchlist_collection.delete_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        if result.deleted_count == 0:
            return jsonify({'error': 'Fund not found in watchlist'}), 404
        
        print(f"[鍒犻櫎鍏虫敞] 鐢ㄦ埛 {current_user_id} 鍒犻櫎 {fund_code}")
        return jsonify({'message': 'Successfully removed from watchlist'})
        
    except Exception as e:
        print(f"鍒犻櫎鍏虫敞澶辫触: {str(e)}")
        return jsonify({'error': 'Failed to remove from watchlist'}), 500

@app.route('/api/watchlist/<fund_code>', methods=['PUT', 'OPTIONS'])
def update_watchlist_threshold(fund_code):
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    db_check = check_db_status()
    if db_check:
        return db_check
    
    token = None
    if 'Authorization' in request.headers:
        auth_header = request.headers['Authorization']
        if auth_header.startswith('Bearer '):
            token = auth_header.split(' ')[1]
    
    if not token:
        return jsonify({'error': 'Token is missing'}), 401
    
    try:
        data = jwt.decode(token, JWT_SECRET, algorithms=['HS256'])
        current_user_id = data['userId']
    except jwt.ExpiredSignatureError:
        return jsonify({'error': 'Token has expired'}), 401
    except jwt.InvalidTokenError:
        return jsonify({'error': 'Invalid token'}), 401
    
    try:
        req_data = request.get_json()
        new_threshold = req_data.get('alertThreshold')
        
        if new_threshold is None:
            return jsonify({'error': 'alertThreshold is required'}), 400
        
        result = watchlist_collection.update_one(
            {'userId': current_user_id, 'fundCode': fund_code},
            {'$set': {'alertThreshold': new_threshold}}
        )
        
        if result.matched_count == 0:
            return jsonify({'error': 'Fund not found in watchlist'}), 404
        
        updated_item = watchlist_collection.find_one(
            {'userId': current_user_id, 'fundCode': fund_code},
            {'_id': 0}
        )
        
        print(f"[鏇存柊闃堝€糫 鐢ㄦ埛 {current_user_id} 鏇存柊 {fund_code} 闃堝€间负 {new_threshold}")
        return jsonify(updated_item)
        
    except Exception as e:
        print(f"鏇存柊闃堝€煎け锟? {str(e)}")
        return jsonify({'error': 'Failed to update threshold'}), 500

@app.route('/api/auth/register', methods=['POST', 'OPTIONS'])
def register():
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200

    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        data = request.get_json()
        email = data.get('email')
        password = data.get('password')
        
        if not email or not password:
            return jsonify({'error': 'Email and password are required'}), 400
        
        existing_user = users_collection.find_one({'email': email})
        
        if existing_user and existing_user.get('is_verified'):
            print(f"[娉ㄥ唽] 锟?鐢ㄦ埛宸插瓨鍦ㄤ笖宸查獙锟? {email}")
            return jsonify({'error': 'User already exists'}), 409
        
        import random
        verification_code = str(random.randint(100000, 999999))
        
        success = send_verification_email(email, verification_code)
        
        if not success:
            print(f"[娉ㄥ唽] 锟?鐢ㄦ埛 {email} 閭欢鍙戦€佸け璐ワ紝鎷掔粷娉ㄥ唽")
            return jsonify({'error': '閭欢鏈嶅姟閫氳澶辫触锛岃妫€鏌ラ厤缃垨绋嶅悗閲嶈瘯'}), 500
        
        expires_at = datetime.now(timezone.utc) + timedelta(minutes=10)
        
        pending_user = {
            'email': email,
            'password': hash_password(password),
            'verification_code': verification_code,
            'verification_code_expires': expires_at,
            'createdAt': datetime.now(timezone.utc)
        }
        
        pending_users_collection.update_one(
            {'email': email},
            {'$set': pending_user},
            upsert=True
        )
        
        print(f"[娉ㄥ唽] 锟?鐢ㄦ埛 {email} 楠岃瘉鐮佸凡鍙戦€侊紝瀛樺叆pending_users")
        return jsonify({'status': 'success', 'message': 'Verification code sent to your email'}), 200
        
    except Exception as e:
        print(f"[娉ㄥ唽] 锟?娉ㄥ唽澶辫触: {str(e)}")
        return jsonify({'error': 'Registration failed'}), 500

@app.route('/api/auth/verify', methods=['POST', 'OPTIONS'])
def verify():
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200

    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        data = request.get_json()
        email = data.get('email')
        code = data.get('code')
        
        print(f"[楠岃瘉] 鏀跺埌楠岃瘉璇锋眰: email={email}, code={code}")
        
        if not email or not code:
            print(f"[楠岃瘉] 锟?鍙傛暟缂哄け: email={email}, code={code}")
            return jsonify({'error': 'Email and verification code are required'}), 400
        
        pending_user = pending_users_collection.find_one({'email': email})
        
        if not pending_user:
            print(f"[楠岃瘉] 锟?寰呴獙璇佺敤鎴蜂笉瀛樺湪: {email}")
            return jsonify({'error': 'User not found or registration expired'}), 404
        
        stored_code = pending_user.get('verification_code')
        expires_at = pending_user.get('verification_code_expires')
        
        print(f"[楠岃瘉] 鏁版嵁搴撳瓨锟? stored_code={stored_code}, type={type(stored_code)}")
        print(f"[楠岃瘉] 鍓嶇浼犲叆: code={code}, type={type(code)}")
        print(f"[楠岃瘉] 杩囨湡鏃堕棿: {expires_at}, type={type(expires_at)}")
        
        code_str = str(code).strip()
        stored_code_str = str(stored_code).strip() if stored_code else ''
        
        print(f"[楠岃瘉] 姣旇緝: code_str='{code_str}' vs stored_code_str='{stored_code_str}'")
        
        if code_str != stored_code_str:
            print(f"[楠岃瘉] 锟?楠岃瘉鐮佷笉鍖归厤")
            return jsonify({'error': 'Invalid verification code'}), 400
        
        if expires_at:
            if isinstance(expires_at, datetime):
                now = datetime.now(timezone.utc)
                if expires_at.tzinfo is None:
                    expires_at = expires_at.replace(tzinfo=timezone.utc)
                print(f"[楠岃瘉] 鏃堕棿姣旇緝: now={now}, expires_at={expires_at}")
                if now > expires_at:
                    print(f"[楠岃瘉] 锟?楠岃瘉鐮佸凡杩囨湡")
                    return jsonify({'error': 'Verification code expired'}), 400
            else:
                print(f"[楠岃瘉] 鈿狅笍 expires_at 绫诲瀷寮傚父: {type(expires_at)}")
        
        verified_user = {
            'email': email,
            'password': pending_user.get('password'),
            'is_verified': True,
            'verified_at': datetime.now(timezone.utc),
            'createdAt': pending_user.get('createdAt', datetime.now(timezone.utc))
        }

        users_collection.update_one(
            {'email': email},
            {'$set': verified_user},
            upsert=True
        )
        saved_user = users_collection.find_one({'email': email})
        user_id = str(saved_user['_id'])
        
        pending_users_collection.delete_one({'email': email})
        
        payload = {
            'userId': user_id,
            'email': email,
            'exp': datetime.now(timezone.utc) + timedelta(hours=24)
        }
        token = jwt.encode(payload, JWT_SECRET, algorithm='HS256')
        
        print(f"[楠岃瘉] 锟?鐢ㄦ埛 {email} 楠岃瘉鎴愬姛锛屽凡鍐欏叆users闆嗗悎")
        return jsonify({
            'status': 'success',
            'message': 'Verification successful',
            'token': token,
            'email': email
        }), 200
        
    except Exception as e:
        print(f"[楠岃瘉] 锟?楠岃瘉澶辫触: {str(e)}")
        import traceback
        traceback.print_exc()
        return jsonify({'error': 'Verification failed'}), 500


@app.route('/api/auth/resend', methods=['POST', 'OPTIONS'])
def resend_verification():
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200

    db_check = check_db_status()
    if db_check:
        return db_check

    try:
        data = request.get_json() or {}
        email = data.get('email')

        if not email:
            return jsonify({'error': 'Email is required'}), 400

        pending_user = pending_users_collection.find_one({'email': email})
        if not pending_user:
            return jsonify({'error': 'User not found or registration expired'}), 404

        import random
        verification_code = str(random.randint(100000, 999999))
        success = send_verification_email(email, verification_code)
        if not success:
            return jsonify({'error': 'Failed to send verification code'}), 500

        pending_users_collection.update_one(
            {'email': email},
            {
                '$set': {
                    'verification_code': verification_code,
                    'verification_code_expires': datetime.now(timezone.utc) + timedelta(minutes=10)
                }
            }
        )

        return jsonify({'status': 'success', 'message': 'Verification code resent'}), 200
    except Exception as e:
        print(f"[閲嶆柊鍙戦€乢 锟?澶辫触: {str(e)}")
        return jsonify({'error': 'Resend failed'}), 500

@app.route('/api/auth/login', methods=['POST', 'OPTIONS'])
def login():
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200

    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        data = request.get_json()
        email = data.get('email')
        password = data.get('password')
        
        if not email or not password:
            return jsonify({'error': 'Email and password are required'}), 400
        
        user = users_collection.find_one({'email': email})

        if not user or not verify_password(user.get('password'), password):
            return jsonify({'error': 'Invalid email or password'}), 401

        # Migrate legacy plaintext password to hashed password after successful login.
        if user.get('password') == password:
            users_collection.update_one(
                {'_id': user['_id']},
                {'$set': {'password': hash_password(password)}}
            )
        
        payload = {
            'userId': str(user['_id']),
            'email': email,
            'exp': datetime.now(timezone.utc) + timedelta(hours=24)
        }
        
        token = jwt.encode(payload, JWT_SECRET, algorithm='HS256')
        
        print(f"[鐧诲綍] 鐢ㄦ埛 {email} 鐧诲綍鎴愬姛")
        return jsonify({
            'status': 'success',
            'token': token,
            'email': email
        }), 200
        
    except Exception as e:
        print(f"[鐧诲綍] 锟?鐧诲綍澶辫触: {str(e)}")
        return jsonify({'error': 'Login failed'}), 500

@app.route('/')
def index():
    version = get_version_payload()
    version_text = f"version={version['version']}, commit={version['commit'] or 'unknown'}"
    if db_error_message:
        return f"API is Running, but Database Error: {db_error_message} ({version_text})", 200
    return f"Fund Tracking API is Running successfully with DB connected. ({version_text})", 200

@app.route('/api/version')
def api_version():
    return jsonify(get_version_payload())

@app.route('/health')
def health():
    latest_update_time = get_latest_update_time(DEFAULT_FUND_CODES)
    latest_update_age_seconds = None
    if latest_update_time:
        latest_update_age_seconds = max(0, int(time.time()) - int(latest_update_time))

    return jsonify({
        "status": "ok",
        "version": get_version_payload(),
        "db_connected": collection is not None,
        "db_error": db_error_message,
        "latest_update_time": latest_update_time,
        "latest_update_age_seconds": latest_update_age_seconds,
        "auto_refresh_interval_seconds": AUTO_REFRESH_INTERVAL_SECONDS
    })

@app.route('/api/health')
def api_health():
    return jsonify({
        "status": "ok",
        "version": get_version_payload()
    })

if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8080))
    print(f"Starting Flask server on port {port}...")
    app.run(host='0.0.0.0', port=port)

