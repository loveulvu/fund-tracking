import requests
import json
import time

def get_fund_info(fund_code):
    """
    纯 API 驱动的基金信息获取函数
    """
    headers_web = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
    headers_mobile = {
        'User-Agent': 'Dalvik/2.1.0 (Linux; U; Android 10; SM-G981B Build/QP1A.190711.020)',
        'Host': 'fundmobapi.eastmoney.com',
        'Connection': 'Keep-Alive',
    }
    
    print(f"[{fund_code}] 开始获取基金信息（纯API模式）...")
    
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
            
            data_item['fund_name'] = fund_data.get('name', '未知')
            data_item['net_value'] = float(fund_data.get('dwjz', 0)) if fund_data.get('dwjz') else 0.0
            data_item['net_value_date'] = fund_data.get('jzrq', '')
            data_item['day_growth'] = float(fund_data.get('gszzl', 0)) if fund_data.get('gszzl') else 0.0
            
            print(f"[{fund_code}] fundgz API: {data_item.get('fund_name')} | 净值: {data_item.get('net_value')} | 日涨幅: {data_item.get('day_growth')}%")
        else:
            print(f"[{fund_code}] fundgz API 返回异常: HTTP {response.status_code}")
    except Exception as e:
        print(f"[{fund_code}] fundgz API 获取失败: {str(e)}")
    
    try:
        base_info_url = f"http://fundmobapi.eastmoney.com/FundMNewApi/FundMNBaseInfo?FCODE={fund_code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0"
        response = requests.get(base_info_url, headers=headers_mobile, timeout=5)
        
        if response.status_code == 200:
            result = response.json()
            
            if result.get('Success') and result.get('Datas'):
                fund_info = result['Datas']
                
                if 'fund_name' not in data_item or data_item.get('fund_name') == '未知':
                    data_item['fund_name'] = fund_info.get('SHORTNAME', '未知')
                
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
                
                print(f"[{fund_code}] FundMNBaseInfo API: 周{data_item.get('week_growth', 'N/A')}% | 月{data_item.get('month_growth', 'N/A')}% | 年{data_item.get('year_growth', 'N/A')}%")
            else:
                print(f"[{fund_code}] FundMNBaseInfo API 无数据: {result.get('ErrMsg', 'Unknown error')}")
        else:
            print(f"[{fund_code}] FundMNBaseInfo API HTTP {response.status_code}")
    except Exception as e:
        print(f"[{fund_code}] FundMNBaseInfo API 获取失败: {str(e)}")
    
    growth_fields = ['week_growth', 'month_growth', 'three_month_growth', 'six_month_growth', 'year_growth', 'three_year_growth']
    for field in growth_fields:
        if field not in data_item:
            data_item[field] = 0.0
            print(f"[{fund_code}] {field} 缺失，使用默认值 0.0")
    
    if 'fund_name' not in data_item:
        data_item['fund_name'] = '未知'
    if 'net_value' not in data_item:
        data_item['net_value'] = 0.0
    if 'day_growth' not in data_item:
        data_item['day_growth'] = 0.0
    
    print(f"[{fund_code}] ✅ 数据获取完成: {data_item.get('fund_name')}")
    
    return data_item

def validate_fund_data(fund_data, expected_code, expected_name_hint=None):
    if not fund_data:
        return False, "数据为空"
    
    fund_name = fund_data.get('fund_name', '')
    
    if fund_name == '未知' or not fund_name:
        return False, "基金名称为空或未知"
    
    if fund_data.get('fund_code') != expected_code:
        return False, f"基金代码不匹配: 期望 {expected_code}, 实际 {fund_data.get('fund_code')}"
    
    critical_fields = ['net_value', 'day_growth', 'week_growth', 'month_growth']
    missing_fields = [f for f in critical_fields if f not in fund_data]
    if missing_fields:
        return False, f"缺少关键字段: {missing_fields}"
    
    all_zero = all(
        fund_data.get(field, 0) == 0 
        for field in ['week_growth', 'month_growth', 'year_growth']
    )
    
    if all_zero and fund_name == '未知':
        return False, "所有涨幅为0且基金名称未知，数据可能无效"
    
    return True, "数据有效"

print("=" * 60)
print("测试纯 API 驱动的 get_fund_info 函数")
print("=" * 60)

test_codes = ['161725', '008540', '510300']

for code in test_codes:
    print(f"\n{'=' * 40}")
    print(f"测试基金代码: {code}")
    print(f"{'=' * 40}")
    
    data = get_fund_info(code)
    
    if data:
        print(f"\n获取到的数据:")
        for key, value in data.items():
            print(f"  {key}: {value}")
        
        is_valid, reason = validate_fund_data(data, code)
        print(f"\n数据验证: {'✅ 有效' if is_valid else '❌ 无效'} - {reason}")
    else:
        print(f"❌ 获取数据失败")
    
    time.sleep(0.5)

print("\n" + "=" * 60)
print("测试完成")
print("=" * 60)
