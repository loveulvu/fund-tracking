import os
import sys
import time
import json
import requests
from pymongo import MongoClient
from datetime import datetime

MONGO_URI = os.environ.get("MONGO_URI")
RESEND_API_KEY = os.environ.get("RESEND_API_KEY")
SENDER_EMAIL = os.environ.get("SENDER_EMAIL", "no-reply@fundtracking.online")

if not MONGO_URI:
    print("[错误] MONGO_URI 环境变量缺失")
    sys.exit(1)

if not RESEND_API_KEY:
    print("[警告] RESEND_API_KEY 环境变量缺失，邮件发送功能将不可用")

try:
    client = MongoClient(MONGO_URI, serverSelectionTimeoutMS=5000)
    db = client['fund_tracking']
    watchlist_collection = db['watchlists']
    users_collection = db['users']
    fund_data_collection = db['fund_data']
    client.admin.command('ping')
    print(f"[数据库] 连接成功: {db.name}")
except Exception as e:
    print(f"[错误] 数据库连接失败: {str(e)}")
    sys.exit(1)

def get_fund_info(fund_code):
    """
    获取基金最新净值信息
    """
    headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
    
    try:
        api_url = f"http://fundgz.1234567.com.cn/js/{fund_code}.js"
        response = requests.get(api_url, headers=headers, timeout=5)
        response.encoding = 'utf-8'
        
        jsonp_str = response.text
        if jsonp_str and 'jsonpgz' in jsonp_str:
            json_str = jsonp_str.replace('jsonpgz(', '').replace(');', '')
            fund_data = json.loads(json_str)
            
            return {
                'fund_code': fund_code,
                'fund_name': fund_data.get('name', '未知'),
                'net_value': float(fund_data.get('dwjz', 0)),
                'day_growth': float(fund_data.get('gszzl', 0)),
                'update_time': fund_data.get('jzrq', '')
            }
        
        print(f"[{fund_code}] API 返回数据格式错误")
        return None
        
    except requests.exceptions.Timeout:
        print(f"[{fund_code}] 请求超时")
        return None
    except requests.exceptions.RequestException as e:
        print(f"[{fund_code}] 请求异常: {str(e)}")
        return None
    except Exception as e:
        print(f"[{fund_code}] 获取数据失败: {str(e)}")
        return None

def send_alert_email(user_email, fund_code, fund_name, current_growth, threshold):
    """
    使用 Resend API 发送报警邮件
    """
    if not RESEND_API_KEY:
        print(f"[邮件] RESEND_API_KEY 未配置，跳过发送")
        return False
    
    try:
        url = "https://api.resend.com/emails"
        
        headers = {
            "Authorization": f"Bearer {RESEND_API_KEY}",
            "Content-Type": "application/json"
        }
        
        data = {
            "from": f"FundTracking <{SENDER_EMAIL}>",
            "to": [user_email],
            "subject": f"基金涨幅预警 - {fund_name} ({fund_code})",
            "html": f"""
            <html>
            <body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
                <h2 style="color: #333;">基金涨幅预警通知</h2>
                <p>您好！</p>
                <p>您关注的基金 <strong>{fund_name}</strong> ({fund_code}) 已触发预警阈值。</p>
                
                <div style="background-color: #f5f5f5; padding: 20px; margin: 20px 0; border-radius: 5px;">
                    <p style="margin: 5px 0;"><strong>基金名称：</strong>{fund_name}</p>
                    <p style="margin: 5px 0;"><strong>基金代码：</strong>{fund_code}</p>
                    <p style="margin: 5px 0;"><strong>当前涨幅：</strong><span style="color: {'#ff4444' if current_growth >= 0 else '#00aa00'}; font-weight: bold;">{current_growth}%</span></p>
                    <p style="margin: 5px 0;"><strong>预警阈值：</strong>{threshold}%</p>
                </div>
                
                <p>请及时登录系统查看详情。</p>
                <p style="color: #999; font-size: 12px; margin-top: 30px;">
                    此邮件由基金追踪系统自动发送，请勿回复。
                </p>
            </body>
            </html>
            """
        }
        
        response = requests.post(url, headers=headers, json=data, timeout=10)
        
        if response.status_code == 200:
            print(f"[邮件] ✅ 发送成功: {user_email}")
            return True
        else:
            print(f"[邮件] ❌ 发送失败: {response.status_code} - {response.text}")
            return False
            
    except Exception as e:
        print(f"[邮件] ❌ 发送异常: {str(e)}")
        return False

def monitor_funds():
    """
    主监控函数：检查所有关注基金的涨幅并触发预警
    """
    print("=" * 60)
    print(f"[监控开始] {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 60)
    
    try:
        watchlist_items = list(watchlist_collection.find({}))
        
        if not watchlist_items:
            print("[监控] 没有需要监控的基金")
            return
        
        print(f"[监控] 共找到 {len(watchlist_items)} 个关注项")
        
        alerts_sent = 0
        alerts_failed = 0
        
        for item in watchlist_items:
            fund_code = item.get('fundCode')
            fund_name = item.get('fundName', '未知')
            threshold = item.get('alertThreshold', 5)
            user_id = item.get('userId')
            
            print(f"\n[检查] {fund_name} ({fund_code}) - 阈值: {threshold}%")
            
            fund_info = get_fund_info(fund_code)
            
            if not fund_info:
                print(f"[跳过] {fund_code} 数据获取失败")
                continue
            
            current_growth = fund_info.get('day_growth', 0)
            print(f"[数据] 当前涨幅: {current_growth}%")
            
            if abs(current_growth) >= abs(threshold):
                print(f"[触发] 涨幅 {current_growth}% 超过阈值 {threshold}%")
                
                user = users_collection.find_one({'_id': user_id})
                if not user:
                    try:
                        from bson import ObjectId
                        user = users_collection.find_one({'_id': ObjectId(user_id)})
                    except:
                        pass
                
                if user:
                    user_email = user.get('email')
                    if user_email:
                        success = send_alert_email(
                            user_email, 
                            fund_code, 
                            fund_name, 
                            current_growth, 
                            threshold
                        )
                        
                        if success:
                            alerts_sent += 1
                        else:
                            alerts_failed += 1
                    else:
                        print(f"[警告] 用户 {user_id} 没有邮箱地址")
                else:
                    print(f"[警告] 找不到用户 {user_id}")
            else:
                print(f"[正常] 涨幅未超过阈值")
            
            time.sleep(0.5)
        
        print("\n" + "=" * 60)
        print(f"[监控结束] 预警邮件发送: 成功 {alerts_sent} 封，失败 {alerts_failed} 封")
        print("=" * 60)
        
    except Exception as e:
        print(f"[错误] 监控过程异常: {str(e)}")
        import traceback
        traceback.print_exc()
    finally:
        client.close()
        print("[数据库] 连接已关闭")

if __name__ == "__main__":
    monitor_funds()
