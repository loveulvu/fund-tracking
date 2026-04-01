import os
import time
import requests
import random
import threading
from bs4 import BeautifulSoup
from flask import Flask, jsonify, request
from flask_cors import CORS
from pymongo import MongoClient
from functools import wraps
import jwt
from datetime import datetime, timedelta
import resend

app = Flask(__name__)
CORS(app, resources={r"/api/*": {"origins": "*"}})

# 1. 环境变量读取
MONGO_URI = os.environ.get("MONGO_URI")
JWT_SECRET = os.environ.get("JWT_SECRET", "fund_tracking_secret_key_2026")
RESEND_API_KEY = os.environ.get("RESEND_API_KEY")

# 初始化 Resend
if RESEND_API_KEY:
    resend.api_key = RESEND_API_KEY

# 2. 全局数据库对象占位
db = None
collection = None
watchlist_collection = None
users_collection = None
db_error_message = None

# 3. 柔性连接逻辑：即使失败也不触发进程崩溃，保证 Railway 能顺利启动
if not MONGO_URI:
    db_error_message = "CRITICAL: MONGO_URI 环境变量缺失，请在 Railway 的 Variables 中添加。"
    print(db_error_message)
else:
    try:
        # 设置 2 秒超时，防止网络卡死导致 Railway 健康检查超时
        client = MongoClient(MONGO_URI, serverSelectionTimeoutMS=2000)
        db = client['fund_tracking']
        collection = db['fund_data']
        watchlist_collection = db['watchlists']
        users_collection = db['users']
        # 测试连接
        client.admin.command('ping')
        print(f"Flask 正在连接数据库: {db.name}")
        print("MongoDB 连接成功。")
    except Exception as e:
        db_error_message = f"MongoDB 连接失败: {str(e)}"
        print(db_error_message)

DEFAULT_FUND_CODES = [
    "006030", "000001", "110022", "161725", "110011"
]

def get_fund_info(fund_code):
    headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
    
    print(f"[{fund_code}] 开始获取基金信息...")
    
    try:
        data_item = {
            "fund_code": fund_code,
            "update_time": int(time.time())
        }
        
        try:
            api_url = f"http://fundgz.1234567.com.cn/js/{fund_code}.js"
            response = requests.get(api_url, headers=headers, timeout=3)
            response.encoding = 'utf-8'
            
            jsonp_str = response.text
            if jsonp_str and 'jsonpgz' in jsonp_str:
                json_str = jsonp_str.replace('jsonpgz(', '').replace(');', '')
                import json
                fund_data = json.loads(json_str)
                
                data_item['fund_name'] = fund_data.get('name', '未知')
                data_item['net_value'] = float(fund_data.get('dwjz', 0))
                data_item['net_value_date'] = fund_data.get('jzrq', '')
                data_item['day_growth'] = float(fund_data.get('gszzl', 0))
                
                print(f"[{fund_code}] 从 API 获取基本信息成功: {data_item['fund_name']}")
        except Exception as e:
            print(f"[{fund_code}] API 获取失败: {str(e)}")
        
        try:
            pingzhong_url = f"http://fund.eastmoney.com/pingzhongdata/{fund_code}.js"
            response = requests.get(pingzhong_url, headers=headers, timeout=3)
            response.encoding = 'utf-8'
            
            import re
            text = response.text
            
            syl_1y_match = re.search(r'var\s+syl_1y\s*=\s*"([^"]*)";', text)
            syl_3y_match = re.search(r'var\s+syl_3y\s*=\s*"([^"]*)";', text)
            syl_6y_match = re.search(r'var\s+syl_6y\s*=\s*"([^"]*)";', text)
            syl_1n_match = re.search(r'var\s+syl_1n\s*=\s*"([^"]*)";', text)
            
            def parse_growth_value(match):
                if match:
                    value = match.group(1).strip()
                    if value and value != '--' and value != '':
                        try:
                            return float(value)
                        except:
                            return 0.0
                return 0.0
            
            data_item['month_growth'] = parse_growth_value(syl_1y_match)
            data_item['three_month_growth'] = parse_growth_value(syl_3y_match)
            data_item['six_month_growth'] = parse_growth_value(syl_6y_match)
            data_item['year_growth'] = parse_growth_value(syl_1n_match)
            
            print(f"[{fund_code}] 从品种数据接口获取收益数据成功")
        except Exception as e:
            print(f"[{fund_code}] 品种数据接口获取失败: {str(e)}")
        
        try:
            url = f"https://fund.eastmoney.com/{fund_code}.html"
            response = requests.get(url, headers=headers, timeout=3)
            response.encoding = 'utf-8'
            soup = BeautifulSoup(response.text, 'html.parser')
            html_text = response.text
            
            if 'fund_name' not in data_item:
                fund_name = soup.find('span', class_='funCur-FundName')
                data_item['fund_name'] = fund_name.text.strip() if fund_name else "未知"
            
            if 'net_value' not in data_item:
                net_value_elem = soup.find('dl', class_='dataItem02')
                if net_value_elem:
                    net_value = net_value_elem.find('span', class_='ui-font-large')
                    if net_value:
                        data_item['net_value'] = float(net_value.text.strip())
                    
                    net_value_date = net_value_elem.find('dt')
                    if net_value_date:
                        data_item['net_value_date'] = net_value_date.text.strip()
            
            if 'day_growth' not in data_item:
                day_growth_elem = soup.find('dl', class_='dataItem03')
                if day_growth_elem:
                    day_growth = day_growth_elem.find('span', class_='ui-font-large')
                    if day_growth:
                        growth_text = day_growth.text.strip().replace('%', '')
                        try:
                            data_item['day_growth'] = float(growth_text)
                        except:
                            pass
            
            print(f"[{fund_code}] 从主页获取补充数据成功")
        except Exception as e:
            print(f"[{fund_code}] 主页获取失败: {str(e)}")
        
        print(f"[{fund_code}] 获取完成: {data_item.get('fund_name', '未知')}")
        return data_item
        
    except requests.exceptions.Timeout:
        print(f"[{fund_code}] ❌ 请求超时")
        return None
    except requests.exceptions.RequestException as e:
        print(f"[{fund_code}] ❌ 请求异常: {str(e)}")
        return None
    except Exception as e:
        print(f"[{fund_code}] ❌ 未知错误: {str(e)}")
        return None

# API 路由安全检查装饰逻辑
def check_db_status():
    if db_error_message:
        return jsonify({"status": "error", "message": db_error_message}), 500
    if collection is None:
        return jsonify({"status": "error", "message": "数据库未初始化"}), 500
    return None

@app.route('/api/update')
def update_funds():
    db_check = check_db_status()
    if db_check: return db_check
    
    updated = []
    failed = []
    
    print("=" * 50)
    print(f"开始更新基金数据，共 {len(DEFAULT_FUND_CODES)} 个基金")
    print("=" * 50)
    
    for c in DEFAULT_FUND_CODES:
        data = get_fund_info(c)
        if data:
            try:
                collection.update_one({"fund_code": c}, {"$set": data}, upsert=True)
                updated.append(c)
                print(f"[{c}] ✅ 数据库更新成功")
            except Exception as e:
                failed.append({"code": c, "reason": f"数据库更新失败: {str(e)}"})
                print(f"[{c}] ❌ 数据库更新失败: {str(e)}")
        else:
            failed.append({"code": c, "reason": "获取基金信息失败"})
        
        # 添加短暂延迟，避免请求过快
        time.sleep(0.5)
    
    print("=" * 50)
    print(f"更新完成: 成功 {len(updated)} 个，失败 {len(failed)} 个")
    print("=" * 50)
    
    return jsonify({
        "status": "success",
        "count": len(updated),
        "codes": updated,
        "failed": failed,
        "total": len(DEFAULT_FUND_CODES)
    })

@app.route('/api/funds')
def get_funds():
    db_check = check_db_status()
    if db_check: return db_check
    
    try:
        data = list(collection.find({}, {"_id": 0}))
        return jsonify(data)
    except Exception as e:
        print(f"获取基金列表失败: {str(e)}")
        return jsonify({"status": "error", "message": f"获取数据失败: {str(e)}"}), 500

@app.route('/api/fund/<fund_code>')
def get_fund(fund_code):
    """获取单个基金数据"""
    db_check = check_db_status()
    if db_check: return db_check
    
    try:
        # 先从数据库查找
        fund_data = collection.find_one({"fund_code": fund_code}, {"_id": 0})
        
        if fund_data:
            print(f"[{fund_code}] 从数据库获取成功")
            return jsonify(fund_data)
        
        # 数据库中没有，从天天基金网爬取
        print(f"[{fund_code}] 数据库中不存在，从天天基金网获取...")
        data = get_fund_info(fund_code)
        
        if data:
            # 保存到数据库
            collection.update_one({"fund_code": fund_code}, {"$set": data}, upsert=True)
            print(f"[{fund_code}] ✅ 从天天基金网获取成功并保存到数据库")
            return jsonify(data)
        else:
            return jsonify({"error": "Fund not found"}), 404
            
    except Exception as e:
        print(f"获取基金数据失败: {str(e)}")
        return jsonify({"error": f"Failed to fetch fund data: {str(e)}"}), 500

@app.route('/')
def index():
    if db_error_message:
        return f"API is Running, but Database Error: {db_error_message}", 200
    return "Fund Tracking API is Running successfully with DB connected.", 200

@app.route('/health')
def health():
    return jsonify({
        "status": "ok",
        "db_connected": collection is not None,
        "db_error": db_error_message
    })

# ============ JWT 验证装饰器 ============

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

# ============ 邮件发送功能 ============

def send_verification_email(to_email, code):
    """
    使用 Resend API 发送验证码邮件
    发件人必须为 no-reply@send.fundtracking.online
    """
    print(f"[邮件] 开始发送验证码到 {to_email}")
    
    if not RESEND_API_KEY:
        print("[邮件] ❌ RESEND_API_KEY 未配置")
        print(f"[邮件] 验证码: {code}")
        return False, f"邮件服务未配置，验证码: {code}"
    
    try:
        print("[邮件] 正在通过 Resend API 发送...")
        
        params = {
            "from": "no-reply@fundtracking.online",
            "to": [to_email],
            "subject": "基金追踪系统 - 邮箱验证码",
            "html": f'''
            <html>
            <body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
            <h2 style="color: #333;">基金追踪系统</h2>
            <p>您好！</p>
            <p>您的邮箱验证码是：</p>
            <div style="background-color: #f5f5f5; padding: 20px; text-align: center; margin: 20px 0;">
                <span style="font-size: 32px; font-weight: bold; color: #4CAF50;">{code}</span>
            </div>
            <p>验证码有效期为 <strong>10分钟</strong>，请尽快完成验证。</p>
            <p style="color: #999; font-size: 12px;">如果您没有注册账号，请忽略此邮件。</p>
            </body>
            </html>
            '''
        }
        
        result = resend.Emails.send(params)
        print(f"[邮件] ✅ 发送成功: {result}")
        return True, "发送成功"
        
    except Exception as e:
        print(f"[邮件] ❌ 发送失败: {str(e)}")
        return False, f"邮件发送失败，验证码: {code}"

# ============ 用户认证 API ============

@app.route('/api/auth/register', methods=['POST', 'OPTIONS'])
def register():
    """
    用户注册接口
    业务逻辑：用户在通过邮箱验证之前，绝对不能成为系统中的有效用户
    状态存储：注册时，将用户的注册信息和生成的验证 Token 存入 users 集合，is_verified 设为 false
    """
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    if users_collection is None:
        return jsonify({'error': 'Database not connected'}), 500
    
    try:
        data = request.get_json()
        email = data.get('email')
        password = data.get('password')
        
        if not email or not password:
            return jsonify({'error': 'Email and password are required'}), 400
        
        # 检查用户是否已存在
        existing_user = users_collection.find_one({'email': email})
        if existing_user:
            # 如果用户已存在且已验证，返回错误
            if existing_user.get('is_verified'):
                return jsonify({'error': 'User already exists and verified'}), 400
            # 如果用户已存在但未验证，重新发送验证码
            else:
                verification_code = str(random.randint(100000, 999999))
                users_collection.update_one(
                    {'email': email},
                    {'$set': {
                        'password': password,  # 更新密码
                        'verification_code': verification_code,
                        'verification_code_expires': datetime.utcnow() + timedelta(minutes=10)
                    }}
                )
                # 异步发送邮件
                threading.Thread(target=send_verification_email, args=(email, verification_code)).start()
                return jsonify({'emailSent': True, 'message': '验证邮件正在发送中，请检查邮箱（包括垃圾邮件）'}), 200
        
        # 新用户注册
        verification_code = str(random.randint(100000, 999999))
        
        user = {
            'email': email,
            'password': password,  # 注意：实际生产环境应哈希密码
            'is_verified': False,  # 未验证状态
            'verification_code': verification_code,
            'verification_code_expires': datetime.utcnow() + timedelta(minutes=10),
            'created_at': datetime.utcnow()
        }
        
        # 存入数据库，但 is_verified 为 false，不代表注册成功
        users_collection.insert_one(user)
        
        # 异步发送邮件
        threading.Thread(target=send_verification_email, args=(email, verification_code)).start()
        
        return jsonify({'emailSent': True, 'message': '验证邮件正在发送中，请检查邮箱（包括垃圾邮件）'}), 200
        
    except Exception as e:
        print(f"注册失败: {str(e)}")
        return jsonify({'error': 'Registration failed'}), 500

@app.route('/api/auth/login', methods=['POST', 'OPTIONS'])
def login():
    """
    用户登录接口
    业务逻辑：只有验证通过的用户才能登录
    """
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    if users_collection is None:
        return jsonify({'error': 'Database not connected'}), 500
    
    try:
        data = request.get_json()
        email = data.get('email')
        password = data.get('password')
        
        if not email or not password:
            return jsonify({'error': 'Email and password are required'}), 400
        
        user = users_collection.find_one({'email': email})
        if not user:
            return jsonify({'error': 'User not found'}), 404
        
        # 检查是否已验证
        if not user.get('is_verified'):
            return jsonify({'error': 'Please verify your email first', 'needVerification': True}), 401
        
        # 验证密码
        if user['password'] != password:
            return jsonify({'error': 'Invalid password'}), 401
        
        # 生成 JWT Token
        user_id = str(user['_id'])
        
        token = jwt.encode({
            'userId': user_id,
            'email': email,
            'exp': datetime.utcnow() + timedelta(days=7)
        }, JWT_SECRET, algorithm='HS256')
        
        return jsonify({
            'message': 'Login successful',
            'token': token,
            'email': email
        }), 200
        
    except Exception as e:
        print(f"登录失败: {str(e)}")
        return jsonify({'error': 'Login failed'}), 500

@app.route('/api/auth/verify', methods=['POST', 'OPTIONS'])
def verify_email():
    """
    邮箱验证接口
    业务逻辑：验证通过后，将 is_verified 设为 true，用户才能成为有效用户
    """
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    if users_collection is None:
        return jsonify({'error': 'Database not connected'}), 500
    
    try:
        data = request.get_json()
        email = data.get('email')
        code = data.get('code')
        password = data.get('password')
        
        if not email or not code:
            return jsonify({'error': 'Email and code are required'}), 400
        
        user = users_collection.find_one({'email': email})
        if not user:
            return jsonify({'error': 'User not found'}), 404
        
        # 如果已经验证过
        if user.get('is_verified'):
            return jsonify({'message': 'Email already verified'}), 200
        
        # 获取存储的验证码
        stored_code = user.get('verification_code')
        expires = user.get('verification_code_expires')
        
        if not stored_code or not expires:
            return jsonify({'error': 'Verification code not found, please register again'}), 400
        
        # 检查验证码是否过期
        if datetime.utcnow() > expires:
            return jsonify({'error': 'Verification code expired'}), 400
        
        # 验证验证码
        if code != stored_code:
            return jsonify({'error': 'Invalid verification code'}), 400
        
        # 验证通过，将用户转为正式有效用户
        users_collection.update_one(
            {'email': email},
            {'$set': {
                'is_verified': True,
                'verification_code': None,
                'verification_code_expires': None
            }}
        )
        
        # 生成 JWT Token，自动登录
        user_id = str(user['_id'])
        
        token = jwt.encode({
            'userId': user_id,
            'email': email,
            'exp': datetime.utcnow() + timedelta(days=7)
        }, JWT_SECRET, algorithm='HS256')
        
        return jsonify({
            'message': 'Email verified successfully',
            'token': token,
            'email': email
        }), 200
        
    except Exception as e:
        print(f"验证失败: {str(e)}")
        return jsonify({'error': 'Verification failed'}), 500

@app.route('/api/auth/check', methods=['GET', 'OPTIONS'])
@token_required
def verify_token(current_user_id):
    """
    验证 Token 有效性接口
    """
    if request.method == 'OPTIONS':
        return jsonify({'status': 'ok'}), 200
    
    if users_collection is None:
        return jsonify({'error': 'Database not connected'}), 500
    
    try:
        from bson.objectid import ObjectId
        user = users_collection.find_one({'_id': ObjectId(current_user_id)})
        
        if not user:
            return jsonify({'error': 'User not found'}), 404
        
        return jsonify({
            'status': 'success',
            'user': {
                'id': current_user_id,
                'email': user['email'],
                'is_verified': user.get('is_verified', False)
            }
        }), 200
        
    except Exception as e:
        print(f"验证失败: {str(e)}")
        return jsonify({'error': 'Verification failed'}), 500

# ============ 关注列表 API ============

print("System: Loading Route /api/watchlist...")

@app.route('/api/watchlist', methods=['GET'])
@token_required
def get_watchlist(current_user_id):
    """获取当前用户的关注列表"""
    print(f"[DEBUG] Received request: {request.method} {request.path}")
    
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        watchlist = list(watchlist_collection.find({'userId': current_user_id}, {'_id': 0}))
        print(f"[DEBUG] Returning {len(watchlist)} items for user {current_user_id}")
        return jsonify(watchlist)
    except Exception as e:
        print(f"获取关注列表失败: {str(e)}")
        return jsonify({'error': 'Failed to fetch watchlist'}), 500

@app.route('/api/watchlist', methods=['POST'])
@token_required
def add_to_watchlist(current_user_id):
    """添加基金到关注列表"""
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        data = request.get_json()
        fund_code = data.get('fundCode')
        fund_name = data.get('fundName')
        threshold = data.get('alertThreshold', 5)
        
        if not fund_code or not fund_name:
            return jsonify({'error': 'fundCode and fundName are required'}), 400
        
        # 检查是否已存在
        existing = watchlist_collection.find_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        if existing:
            return jsonify({'error': 'Fund already in watchlist'}), 400
        
        # 添加到关注列表
        watchlist_item = {
            'userId': current_user_id,
            'fundCode': fund_code,
            'fundName': fund_name,
            'alertThreshold': threshold,
            'addedAt': datetime.utcnow()
        }
        
        watchlist_collection.insert_one(watchlist_item)
        
        # 返回时移除 _id
        watchlist_item.pop('_id', None)
        return jsonify(watchlist_item), 201
        
    except Exception as e:
        print(f"添加关注失败: {str(e)}")
        return jsonify({'error': 'Failed to add to watchlist'}), 500

@app.route('/api/watchlist/<fund_code>', methods=['DELETE'])
@token_required
def remove_from_watchlist(current_user_id, fund_code):
    """从关注列表中删除基金"""
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        result = watchlist_collection.delete_one({
            'userId': current_user_id,
            'fundCode': fund_code
        })
        
        if result.deleted_count == 0:
            return jsonify({'error': 'Fund not found in watchlist'}), 404
        
        return jsonify({'message': 'Successfully removed from watchlist'})
        
    except Exception as e:
        print(f"删除关注失败: {str(e)}")
        return jsonify({'error': 'Failed to remove from watchlist'}), 500

@app.route('/api/watchlist/<fund_code>', methods=['PUT'])
@token_required
def update_watchlist_threshold(current_user_id, fund_code):
    """更新关注基金的预警阈值"""
    db_check = check_db_status()
    if db_check:
        return db_check
    
    try:
        data = request.get_json()
        new_threshold = data.get('alertThreshold')
        
        if new_threshold is None:
            return jsonify({'error': 'alertThreshold is required'}), 400
        
        result = watchlist_collection.update_one(
            {'userId': current_user_id, 'fundCode': fund_code},
            {'$set': {'alertThreshold': new_threshold}}
        )
        
        if result.matched_count == 0:
            return jsonify({'error': 'Fund not found in watchlist'}), 404
        
        # 返回更新后的数据
        updated_item = watchlist_collection.find_one(
            {'userId': current_user_id, 'fundCode': fund_code},
            {'_id': 0}
        )
        
        return jsonify(updated_item)
        
    except Exception as e:
        print(f"更新阈值失败: {str(e)}")
        return jsonify({'error': 'Failed to update threshold'}), 500

if __name__ == "__main__":
    # Railway 必须动态读取 PORT 变量
    port = int(os.environ.get("PORT", 8080))
    print(f"Starting Flask server on port {port}...")
    app.run(host='0.0.0.0', port=port)
