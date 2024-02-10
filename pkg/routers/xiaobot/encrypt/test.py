import hashlib
from datetime import datetime, timezone, timedelta


def md5(code: str):
    return hashlib.md5(code.encode("utf8")).hexdigest()


def get_sign(t):
    timestamp = str(int(t))
    return md5(f"dbbc1dd37360b4084c3a69346e0ce2b2.{timestamp}"), timestamp


if __name__ == "__main__":
    est = timezone(timedelta(hours=+8))
    dt = datetime(2020, 1, 1, 0, 0, 0, 0, tzinfo=est)
    print(get_sign(dt.timestamp()))