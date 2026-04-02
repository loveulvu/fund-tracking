import re

# 读取文件
with open('app.py', 'r', encoding='utf-8') as f:
    content = f.read()

# 定义新的get_fund_info函数
new_function = '''def get_fund_info(fund_code):
    """
    获取基金信息（纯API驱动模式）
    """
    headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
    
    print(f"[{fund_code}] 开始获取基金信息...")
    
    try:
        data_item = {
            "fund_code": fund_code,
            "update_time": int(time.time())
        }
        
        # 第一步：从fundgz API获取基本信息
        try:
            api_url = f"http://fundgz.1234567.com.cn/js/{fund_code}.js"
            response = requests.get(api_url, headers=headers, timeout=5)
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
                
                print(f"[{fund_code}] ✅ 从fundgz API获取基本信息成功: {data_item['fund_name']}")
        except Exception as e:
            print(f"[{fund_code}] ⚠️ fundgz API获取失败: {str(e)}")
        
        # 第二步：从fundf10页面获取涨幅数据（使用正则表达式）
        try:
            import re
            f10_url = f"http://fundf10.eastmoney.com/jdzf_{fund_code}.html"
            response = requests.get(f10_url, headers=headers, timeout=5)
            response.encoding = 'utf-8'
            
            if response.status_code == 200:
                html_text = response.text
                
                # 使用正则表达式提取涨幅数据
                patterns = {
                    'week_growth': r'近1周</td><td[^>]*>([+-]?\\d+\\.?\\d*)%',
                    'month_growth': r'近1月</td><td[^>]*>([+-]?\\d+\\.?\\d*)%',
                    'three_month_growth': r'近3月</td><td[^>]*>([+-]?\\d+\\.?\\d*)%',
                    'six_month_growth': r'近6月</td><td[^>]*>([+-]?\\d+\\.?\\d*)%',
                    'year_growth': r'近1年</td><td[^>]*>([+-]?\\d+\\.?\\d*)%',
                    'three_year_growth': r'近3年</td><td[^>]*>([+-]?\\d+\\.?\\d*)%'
                }
                
                for field, pattern in patterns.items():
                    if field not in data_item:
                        match = re.search(pattern, html_text)
                        if match:
                            try:
                                value = float(match.group(1))
                                data_item[field] = value
                                print(f"[{fund_code}] ✅ {field}: {value}%")
                            except Exception as e:
                                print(f"[{fund_code}] ⚠️ {field}解析失败: {str(e)}")
                
                print(f"[{fund_code}] ✅ 从fundf10页面获取涨幅数据成功")
        except Exception as e:
            print(f"[{fund_code}] ⚠️ fundf10页面获取失败: {str(e)}")
        
        # 第三步：数据完整性检查和默认值兜底
        if 'week_growth' not in data_item:
            data_item['week_growth'] = 0.0
            print(f"[{fund_code}] ⚠️ 近1周数据缺失，使用默认值 0.0")
        if 'month_growth' not in data_item:
            data_item['month_growth'] = 0.0
            print(f"[{fund_code}] ⚠️ 近1月数据缺失，使用默认值 0.0")
        if 'three_month_growth' not in data_item:
            data_item['three_month_growth'] = 0.0
            print(f"[{fund_code}] ⚠️ 近3月数据缺失，使用默认值 0.0")
        if 'six_month_growth' not in data_item:
            data_item['six_month_growth'] = 0.0
            print(f"[{fund_code}] ⚠️ 近6月数据缺失，使用默认值 0.0")
        if 'year_growth' not in data_item:
            data_item['year_growth'] = 0.0
            print(f"[{fund_code}] ⚠️ 近1年数据缺失，使用默认值 0.0")
        if 'three_year_growth' not in data_item:
            data_item['three_year_growth'] = 0.0
            print(f"[{fund_code}] ⚠️ 近3年数据缺失，使用默认值 0.0")
        
        print(f"[{fund_code}] ✅ 获取完成: {data_item.get('fund_name', '未知')}")
        print(f"[{fund_code}] 📊 数据完整性检查: {list(data_item.keys())}")
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

'''

# 使用正则表达式替换整个get_fund_info函数
pattern = r'def get_fund_info\(fund_code\):.*?(?=\n\ninit_seed_funds\(\))'
content = re.sub(pattern, new_function, content, flags=re.DOTALL)

# 写回文件
with open('app.py', 'w', encoding='utf-8') as f:
    f.write(content)

print('✅ get_fund_info函数已重写')
