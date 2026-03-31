#爬虫脚本
import requests
from bs4 import BeautifulSoup
import json
import sys
import time
import io

# 设置标准输出编码为UTF-8
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

# 默认基金代码列表（50个）- 精选热门基金
DEFAULT_FUND_CODES = [
    "006030", "000001", "110022", "161725", "110011",
    "001184", "000988", "001048", "001511", "001555",
    "001618", "001630", "001668", "001717", "001809",
    "001838", "001865", "001938", "002001", "002147",
    "002288", "002300", "002311", "002344", "002351",
    "002390", "002407", "002420", "002438", "002441",
    "002442", "002443", "002445", "002446", "000021",
    "000031", "000051", "000061", "000071", "000081",
    "000091", "000101", "000111", "000121", "000131",
    "000141", "000151", "000161", "000171", "000181"
]

def get_fund_info(fund_code):
    url = f"https://fund.eastmoney.com/{fund_code}.html"
    
    # 设置请求头（headers），包括 Cookie 和 User-Agent
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 Edg/146.0.0.0",
        "Cookie": "qgqp_b_id=6f8e0ecf6a105e2ecb77d47467c17c38; EMFUND1=null; EMFUND2=null; EMFUND3=null; EMFUND4=null; EMFUND5=null; EMFUND6=null; EMFUND7=null; EMFUND0=null; EMFUND8=03-28%2015%3A19%3A18@%23%24%u6C38%u8D62%u79D1%u6280%u667A%u9009%u6DF7%u5408%u53D1%u8D77A@%23%24022364; _adsame_fullscreen_20308=1; st_si=62545629237852; st_asi=delete; st_nvi=Xc54vT7GzU3iIpKB1XrCNa62e; nid18=09738dcf7cabef5a60214bb0f0d960ce; nid18_create_time=1774686086769; gviem=dh-kXMXOrUHPWw1sLhB-Af11c; gviem_create_time=1774686086769; ASP.NET_SessionId=gmupwb5uwnmy12fsrd3wvrgn; _adsame_fullscreen_18503=1; EMFUND9=03-28 16:41:57@#$%u5357%u65B9%u660C%u5143%u53EF%u8F6C%u503A%u5238A@%23%24006030; st_pvi=24448246789369; st_sp=2026-03-28%2015%3A12%3A14; st_inirUrl=https%3A%2F%2Ffund.eastmoney.com%2F; st_sn=6; st_psi=20260328170228367-112200304021-3015741241"
    }

    # 发送 GET 请求并带上 headers（包括 Cookie 和 UA）
    response = requests.get(url, headers=headers)
    response.encoding = 'utf-8'  # 设置编码，避免中文乱码

    # 使用 BeautifulSoup 解析 HTML
    soup = BeautifulSoup(response.text, 'html.parser')

    # 提取基金名称
    fund_name = soup.find('span', class_='funCur-FundName')
    fund_name = fund_name.text.strip() if fund_name else "未找到基金名称"

    # 获取所有数据区域，按照顺序匹配数据
    data_items = soup.find_all('dl', class_=lambda x: x and x.startswith('dataItem'))
    
    # 初始化默认值
    one_month_profit = "N/A"
    one_year_profit = "N/A"
    three_month_profit = "N/A"
    six_month_profit = "N/A"

    # 安全地获取数据
    try:
        if len(data_items) > 0:
            # 1. 获取 dataItem01 中的近1月和近1年收益
            item01 = data_items[0]  # dataItem01 区域
            dds = item01.find_all('dd')
            if len(dds) > 1:
                one_month_span = dds[1].find('span', class_='ui-num')
                if one_month_span:
                    one_month_profit = one_month_span.text.strip()
            if len(dds) > 2:
                one_year_span = dds[2].find('span', class_='ui-num')
                if one_year_span:
                    one_year_profit = one_year_span.text.strip()
        
        if len(data_items) > 1:
            # 2. 获取 dataItem02 中的近3月收益
            item02 = data_items[1]  # dataItem02 区域
            dds = item02.find_all('dd')
            if len(dds) > 1:
                three_month_span = dds[1].find('span', class_='ui-num')
                if three_month_span:
                    three_month_profit = three_month_span.text.strip()
        
        if len(data_items) > 2:
            # 3. 获取 dataItem03 中的近6月收益
            item03 = data_items[2]  # dataItem03 区域
            dds = item03.find_all('dd')
            if len(dds) > 1:
                six_month_span = dds[1].find('span', class_='ui-num')
                if six_month_span:
                    six_month_profit = six_month_span.text.strip()
    except Exception as e:
        print(f"解析基金 {fund_code} 数据时出错: {e}", file=sys.stderr)

    # 返回抓取到的数据
    return {
        "fund_code": fund_code,
        "fund_name": fund_name,
        "one_year_profit": one_year_profit,
        "six_month_profit": six_month_profit,
        "three_month_profit": three_month_profit,
        "one_month_profit": one_month_profit
    }

def get_all_funds():
    """获取默认的50个基金数据"""
    funds = []
    for fund_code in DEFAULT_FUND_CODES:
        try:
            fund_info = get_fund_info(fund_code)
            funds.append(fund_info)
            # 避免请求过快被封
            time.sleep(0.5)
        except Exception as e:
            print(f"获取基金 {fund_code} 失败: {e}", file=sys.stderr)
    return funds

if __name__ == "__main__":
    # 从命令行参数获取基金代码
    if len(sys.argv) > 1:
        if sys.argv[1] == "all":
            # 获取所有默认基金
            funds = get_all_funds()
            json_output = json.dumps(funds, ensure_ascii=False)
            # 写入文件
            with open('funds.json', 'w', encoding='utf-8') as f:
                f.write(json_output)
            # 输出文件路径，让Node.js读取文件
            print('FUNDS_JSON:funds.json', flush=True)
        else:
            # 获取单个基金
            fund_code = sys.argv[1]
            fund_info = get_fund_info(fund_code)
            json_output = json.dumps(fund_info, ensure_ascii=False)
            # 写入临时文件
            temp_file = f'fund_{fund_code}.json'
            with open(temp_file, 'w', encoding='utf-8') as f:
                f.write(json_output)
            print(f'FUND_JSON:{temp_file}', flush=True)
    else:
        # 默认获取单个基金
        fund_code = "006030"  # 默认基金代码
        fund_info = get_fund_info(fund_code)
        json_output = json.dumps(fund_info, ensure_ascii=False)
        # 写入临时文件
        temp_file = f'fund_{fund_code}.json'
        with open(temp_file, 'w', encoding='utf-8') as f:
            f.write(json_output)
        print(f'FUND_JSON:{temp_file}', flush=True)