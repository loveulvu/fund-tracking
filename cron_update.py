import requests
import time
from datetime import datetime

API_URL = "https://fund-tracking-production.up.railway.app/api/update"
LOG_FILE = "update.log"
MAX_RETRIES = 3
RETRY_DELAY = 5

def log_message(message):
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    log_entry = f"[{timestamp}] {message}\n"
    print(log_entry.strip())
    with open(LOG_FILE, "a", encoding="utf-8") as f:
        f.write(log_entry)

def trigger_update():
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            log_message(f"尝试第 {attempt} 次更新...")
            response = requests.get(API_URL, timeout=30)
            
            if response.status_code == 200:
                data = response.json()
                log_message(f"✅ 更新成功: {data}")
                return True
            else:
                log_message(f"❌ 更新失败 (HTTP {response.status_code}): {response.text}")
                
        except requests.exceptions.Timeout:
            log_message(f"❌ 请求超时")
        except requests.exceptions.RequestException as e:
            log_message(f"❌ 请求异常: {str(e)}")
        except Exception as e:
            log_message(f"❌ 未知错误: {str(e)}")
        
        if attempt < MAX_RETRIES:
            log_message(f"等待 {RETRY_DELAY} 秒后重试...")
            time.sleep(RETRY_DELAY)
    
    log_message(f"❌ 所有重试均失败")
    return False

if __name__ == "__main__":
    log_message("=" * 50)
    log_message("开始定时更新任务")
    trigger_update()
    log_message("定时更新任务结束")
    log_message("=" * 50)
