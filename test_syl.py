import requests
import re

r = requests.get('http://fund.eastmoney.com/pingzhongdata/000001.js', headers={'User-Agent': 'Mozilla/5.0'}, timeout=5)
matches = re.findall(r'var\s+(syl_\w+)\s*=\s*"([^"]*)"', r.text)
print("Found syl variables:")
for name, value in matches:
    print(f"  {name} = {value}")
