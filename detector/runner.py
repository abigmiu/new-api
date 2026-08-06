#!/usr/bin/env python3
"""容器主进程：调度循环 + 公开报告 HTTP 服务。

- 后台线程启动报告 Web 服务（web.py）；
- 主线程按 DETECT_INTERVAL_MINUTES 间隔循环执行检测周期（scheduler.main 本身跑完整一轮）。
"""

from __future__ import annotations

import os
import signal
import threading
import time

import scheduler
import web


def main() -> None:
    interval = max(1, int(os.environ.get("DETECT_INTERVAL_MINUTES", "60")))

    web_thread = threading.Thread(target=web.main, daemon=True, name="reports-web")
    web_thread.start()

    stop = threading.Event()

    def _shutdown(_signum, _frame):
        print("[runner] 收到退出信号，正在停止...", flush=True)
        stop.set()

    signal.signal(signal.SIGTERM, _shutdown)
    signal.signal(signal.SIGINT, _shutdown)

    # 启动即先跑一轮，保证容器起来就有最新报告。
    # 单轮失败不退出进程：记录后继续，保证报告服务与调度循环常驻。
    while True:
        try:
            scheduler.main()
        except Exception as exc:  # noqa: BLE001
            print(f"[runner] 本轮检测失败（{exc}），下一轮重试。", flush=True)
        if stop.wait(interval * 60):
            break

    print("[runner] 已退出。", flush=True)


if __name__ == "__main__":
    main()