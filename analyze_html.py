import re

with open('debug_main.html', 'r', encoding='utf-8') as f:
    html = f.read()

print('=== Main page analysis ===')
print(f'Total length: {len(html)}')

keywords = ['近1周', '近1月', '近3月', '涨幅', '收益率', 'growth', '周', '月', '净值', '累计']
for kw in keywords:
    count = html.count(kw)
    print(f'{kw}: {count} occurrences')

percent_matches = re.findall(r'([+-]?\d+\.?\d*)%', html)
print(f'\nPercentage values found: {percent_matches[:20]}')

script_matches = re.findall(r'<script[^>]*>(.*?)</script>', html, re.DOTALL)
print(f'\nScript tags found: {len(script_matches)}')

for i, script in enumerate(script_matches):
    if 'data' in script.lower() or 'fund' in script.lower():
        print(f'\nScript {i} (first 500 chars):')
        print(script[:500])
