import requests
from flask import current_app


def is_email_configured():
    return bool(
        current_app.config.get("EMAIL_SENDER")
        and current_app.config.get("EMAIL_PASSWORD")
    )


def email_config_missing_message():
    return "Email service is not configured. Missing EMAIL_SENDER or EMAIL_PASSWORD."


def send_verification_email(email, code):
    email_sender = current_app.config.get("EMAIL_SENDER")
    email_password = current_app.config.get("EMAIL_PASSWORD")

    if not email_password or not email_sender:
        print(f"[email] {email_config_missing_message()}")
        return False

    try:
        html_content = f"""
        <div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
            <h2 style="color: #333;">邮箱验证码</h2>
            <p style="font-size: 16px; color: #666;">你的验证码如下：</p>
            <div style="background: #f5f5f5; padding: 20px; text-align: center; font-size: 32px; font-weight: bold; letter-spacing: 5px; margin: 20px 0;">
                {code}
            </div>
            <p style="font-size: 14px; color: #999;">验证码10分钟内有效，请勿泄露。</p>
            <p style="font-size: 14px; color: #999;">如果不是你本人操作，请忽略此邮件。</p>
        </div>
        """

        response = requests.post(
            "https://api.resend.com/emails",
            headers={
                "Authorization": f"Bearer {email_password}",
                "Content-Type": "application/json",
            },
            json={
                "from": email_sender,
                "to": [email],
                "subject": "邮箱验证码 - Fund Tracking",
                "html": html_content,
            },
            timeout=10,
        )

        if 200 <= response.status_code < 300:
            result = response.json()
            print(f"[email] Verification sent to {email}, Resend ID: {result.get('id')}")
            return True

        print(
            f"[email] Failed to send verification. "
            f"HTTP {response.status_code} - {response.text}"
        )
        return False
    except Exception as exc:
        print(f"[email] Exception while sending verification: {str(exc)}")
        return False
