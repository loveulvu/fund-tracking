import requests
import json

headers = {
    'User-Agent': 'Dalvik/2.1.0 (Linux; U; Android 10; SM-G981B Build/QP1A.190711.020)',
    'Host': 'fundmobapi.eastmoney.com',
    'Connection': 'Keep-Alive',
    'Accept-Encoding': 'gzip',
}

code = '161725'

print("=== Testing with full parameters ===")

# Test 1: FundMNFInfo with all parameters
url = f'http://fundmobapi.eastmoney.com/FundMNewApi/FundMNFInfo?FCODE={code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0'
try:
    r = requests.get(url, headers=headers, timeout=10)
    print(f'\n1. FundMNFInfo')
    print(f'Status: {r.status_code}')
    data = r.json()
    print(f'Success: {data.get("Success")}')
    print(f'Data: {json.dumps(data, ensure_ascii=False, indent=2)[:1000]}')
except Exception as e:
    print(f'Error: {e}')

# Test 2: FundMNInfo
url = f'http://fundmobapi.eastmoney.com/FundMNewApi/FundMNInfo?FCODE={code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0'
try:
    r = requests.get(url, headers=headers, timeout=10)
    print(f'\n2. FundMNInfo')
    print(f'Status: {r.status_code}')
    data = r.json()
    print(f'Success: {data.get("Success")}')
    print(f'Data: {json.dumps(data, ensure_ascii=False, indent=2)[:1000]}')
except Exception as e:
    print(f'Error: {e}')

# Test 3: FundMNDetail
url = f'http://fundmobapi.eastmoney.com/FundMNewApi/FundMNDetail?FCODE={code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0'
try:
    r = requests.get(url, headers=headers, timeout=10)
    print(f'\n3. FundMNDetail')
    print(f'Status: {r.status_code}')
    data = r.json()
    print(f'Success: {data.get("Success")}')
    print(f'Data: {json.dumps(data, ensure_ascii=False, indent=2)[:1000]}')
except Exception as e:
    print(f'Error: {e}')

# Test 4: FundMNBaseInfo
url = f'http://fundmobapi.eastmoney.com/FundMNewApi/FundMNBaseInfo?FCODE={code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0'
try:
    r = requests.get(url, headers=headers, timeout=10)
    print(f'\n4. FundMNBaseInfo')
    print(f'Status: {r.status_code}')
    data = r.json()
    print(f'Success: {data.get("Success")}')
    print(f'Data: {json.dumps(data, ensure_ascii=False, indent=2)[:1000]}')
except Exception as e:
    print(f'Error: {e}')

# Test 5: FundMNSylInfo (收益率信息)
url = f'http://fundmobapi.eastmoney.com/FundMNewApi/FundMNSylInfo?FCODE={code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0'
try:
    r = requests.get(url, headers=headers, timeout=10)
    print(f'\n5. FundMNSylInfo (收益率)')
    print(f'Status: {r.status_code}')
    data = r.json()
    print(f'Success: {data.get("Success")}')
    print(f'Data: {json.dumps(data, ensure_ascii=False, indent=2)[:1000]}')
except Exception as e:
    print(f'Error: {e}')

# Test 6: FundMNPerformance
url = f'http://fundmobapi.eastmoney.com/FundMNewApi/FundMNPerformance?FCODE={code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0'
try:
    r = requests.get(url, headers=headers, timeout=10)
    print(f'\n6. FundMNPerformance')
    print(f'Status: {r.status_code}')
    data = r.json()
    print(f'Success: {data.get("Success")}')
    print(f'Data: {json.dumps(data, ensure_ascii=False, indent=2)[:1000]}')
except Exception as e:
    print(f'Error: {e}')

# Test 7: FundMNRInfo
url = f'http://fundmobapi.eastmoney.com/FundMNewApi/FundMNRInfo?FCODE={code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0'
try:
    r = requests.get(url, headers=headers, timeout=10)
    print(f'\n7. FundMNRInfo')
    print(f'Status: {r.status_code}')
    data = r.json()
    print(f'Success: {data.get("Success")}')
    print(f'Data: {json.dumps(data, ensure_ascii=False, indent=2)[:1000]}')
except Exception as e:
    print(f'Error: {e}')
