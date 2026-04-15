import json
import time

import requests


def _safe_print(message):
    try:
        print(message)
    except UnicodeEncodeError:
        fallback = str(message).encode("utf-8", errors="replace").decode("utf-8")
        print(fallback)


def validate_fund_data(fund_data, expected_code, expected_name_hint=None):
    if not fund_data:
        return False, "数据为空"

    fund_name = fund_data.get("fund_name", "")
    if fund_name == "鏈煡" or not fund_name:
        return False, "Fund name is empty or unknown"

    if fund_data.get("fund_code") != expected_code:
        return (
            False,
            f"基金代码不匹配: expected {expected_code}, got {fund_data.get('fund_code')}",
        )

    critical_fields = ["net_value", "day_growth", "week_growth", "month_growth"]
    missing_fields = [f for f in critical_fields if f not in fund_data]
    if missing_fields:
        return False, f"Missing critical fields: {missing_fields}"

    all_zero = all(
        fund_data.get(field, 0) == 0
        for field in ["week_growth", "month_growth", "year_growth"]
    )

    if all_zero and fund_name == "鏈煡":
        return False, "All growth fields are zero and name is unknown"

    return True, "valid"


def get_fund_info(fund_code):
    headers_web = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
    }
    headers_mobile = {
        "User-Agent": (
            "Dalvik/2.1.0 (Linux; U; Android 10; "
            "SM-G981B Build/QP1A.190711.020)"
        ),
        "Host": "fundmobapi.eastmoney.com",
        "Connection": "Keep-Alive",
    }

    print(f"[{fund_code}] start fetching fund info (pure API mode)...")

    data_item = {
        "fund_code": fund_code,
        "update_time": int(time.time()),
    }

    try:
        api_url = f"https://fundgz.1234567.com.cn/js/{fund_code}.js"
        response = requests.get(api_url, headers=headers_web, timeout=5)
        response.encoding = "utf-8"

        if response.status_code == 200 and "jsonpgz" in response.text:
            json_str = response.text.replace("jsonpgz(", "").replace(");", "")
            fund_data = json.loads(json_str)

            data_item["fund_name"] = fund_data.get("name", "鏈煡")
            data_item["net_value"] = (
                float(fund_data.get("dwjz", 0)) if fund_data.get("dwjz") else 0.0
            )
            data_item["net_value_date"] = fund_data.get("jzrq", "")
            data_item["day_growth"] = (
                float(fund_data.get("gszzl", 0)) if fund_data.get("gszzl") else 0.0
            )

            _safe_print(
                f"[{fund_code}] fundgz API: "
                f"{data_item.get('fund_name')} | "
                f"net {data_item.get('net_value')} | "
                f"day {data_item.get('day_growth')}%"
            )
        else:
            print(f"[{fund_code}] fundgz API returned abnormal HTTP {response.status_code}")
    except Exception as exc:
        print(f"[{fund_code}] fundgz API fetch failed: {str(exc)}")

    try:
        base_info_url = (
            "http://fundmobapi.eastmoney.com/FundMNewApi/FundMNBaseInfo"
            f"?FCODE={fund_code}&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0"
        )
        response = requests.get(base_info_url, headers=headers_mobile, timeout=5)

        if response.status_code == 200:
            result = response.json()

            if result.get("Success") and result.get("Datas"):
                fund_info = result["Datas"]

                if "fund_name" not in data_item or data_item.get("fund_name") == "鏈煡":
                    data_item["fund_name"] = fund_info.get("SHORTNAME", "鏈煡")

                if "net_value" not in data_item or data_item.get("net_value") == 0:
                    dwjz = fund_info.get("DWJZ")
                    if dwjz:
                        data_item["net_value"] = float(dwjz)

                data_item["fund_type"] = fund_info.get("FTYPE", "")
                data_item["fund_company"] = fund_info.get("JJGS", "")
                data_item["fund_manager"] = fund_info.get("JJJL", "")
                data_item["fund_scale"] = fund_info.get("TOTALSCALE", "")

                syl_z = fund_info.get("SYL_Z")
                syl_y = fund_info.get("SYL_Y")
                syl_3y = fund_info.get("SYL_3Y")
                syl_6y = fund_info.get("SYL_6Y")
                syl_1n = fund_info.get("SYL_1N")

                if syl_z is not None:
                    data_item["week_growth"] = float(syl_z)
                if syl_y is not None:
                    data_item["month_growth"] = float(syl_y)
                if syl_3y is not None:
                    data_item["three_month_growth"] = float(syl_3y)
                if syl_6y is not None:
                    data_item["six_month_growth"] = float(syl_6y)
                if syl_1n is not None:
                    data_item["year_growth"] = float(syl_1n)

                print(
                    f"[{fund_code}] FundMNBaseInfo API: "
                    f"week {data_item.get('week_growth', 'N/A')}% | "
                    f"month {data_item.get('month_growth', 'N/A')}% | "
                    f"year {data_item.get('year_growth', 'N/A')}%"
                )
            else:
                print(
                    f"[{fund_code}] FundMNBaseInfo API no data: "
                    f"{result.get('ErrMsg', 'Unknown error')}"
                )
        else:
            print(f"[{fund_code}] FundMNBaseInfo API HTTP {response.status_code}")
    except Exception as exc:
        print(f"[{fund_code}] FundMNBaseInfo API fetch failed: {str(exc)}")

    growth_fields = [
        "week_growth",
        "month_growth",
        "three_month_growth",
        "six_month_growth",
        "year_growth",
        "three_year_growth",
    ]
    for field in growth_fields:
        if field not in data_item:
            data_item[field] = 0.0
            print(f"[{fund_code}] {field} missing; defaulting to 0.0")

    if "fund_name" not in data_item:
        data_item["fund_name"] = "鏈煡"
    if "net_value" not in data_item:
        data_item["net_value"] = 0.0
    if "day_growth" not in data_item:
        data_item["day_growth"] = 0.0

    _safe_print(f"[{fund_code}] fetch completed: {data_item.get('fund_name')}")
    return data_item
