import requests
import re

url = "https://fund.eastmoney.com/000001.html"
headers = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}
response = requests.get(url, headers=headers, timeout=5)
response.encoding = 'utf-8'
html_text = response.text

# 查找所有近1周的位置
import re
for match in re.finditer(r'近1周', html_text):
    start = max(0, match.start() - 100)
    end = min(len(html_text), match.end() + 200)
    print(f"Position {match.start()}:")
    print(html_text[start:end])
    print("---")
