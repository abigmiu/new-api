#!/usr/bin/env python3
"""gpt56 渠道定时检测调度器。

流程（每次运行一个完整周期）：
  1. 通过 new-api 管理端 API 拉取全部"已启用"渠道（每次运行前实时同步，不缓存）；
  2. 对每个渠道从 Models 里挑选一个 gpt-5.6 系模型；
  3. 用 Token 硬固定机制把请求路由到指定渠道：
        Authorization: Bearer sk-<DETECTOR_API_KEY>-<channelId>
     该机制由 new-api 的 middleware/auth.go + middleware/distributor.go 保证
     精确路由到目标渠道，失败重试也不会落到其它渠道；
  4. 调用 gpt56 检测器（单次检测），生成 JSON + HTML 报告；
  5. 重建 reports/index.html 汇总页。

所有网络访问均为纯标准库实现，无第三方依赖。
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
import time
import urllib.parse
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
from typing import Any

SCRIPT_DIR = Path(__file__).resolve().parent
DETECTOR_DIR = SCRIPT_DIR / "gpt56"


def env(name: str, default: str = "") -> str:
    return os.environ.get(name, default).strip()


def required(name: str) -> str:
    value = env(name)
    if not value:
        raise SystemExit(f"[scheduler] 缺少必需环境变量: {name}")
    return value


def log(msg: str) -> None:
    print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] {msg}", flush=True)


DEFAULT_USER_AGENT = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
                      "(KHTML, like Gecko) Chrome/126.0 Safari/537.36")


def http_json(url: str, method: str = "GET", headers: dict | None = None,
              timeout: float = 30.0) -> dict[str, Any]:
    """执行一次 HTTP 请求并返回 JSON（dict）。非 2xx 或非 JSON 抛异常。"""
    request_headers = dict(headers or {})
    request_headers.setdefault("User-Agent", env("HTTP_USER_AGENT", DEFAULT_USER_AGENT))
    req = urllib.request.Request(url, method=method, headers=request_headers)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        raw = resp.read().decode("utf-8")
    return json.loads(raw)


def fetch_enabled_channels(admin_base: str, access_token: str) -> list[dict[str, Any]]:
    """通过管理端 API 实时获取全部启用渠道。"""
    page = 1
    page_size = 500
    channels: list[dict[str, Any]] = []
    while True:
        url = (admin_base.rstrip("/") + "/api/channel"
               + "?" + urllib.parse.urlencode({"status": "1", "page": page, "page_size": page_size}))
        body = http_json(url, headers={"Authorization": f"Bearer {access_token}"})
        if not body.get("success"):
            raise RuntimeError(f"获取渠道失败: {body.get('message')}")
        data = body.get("data") or {}
        items = data.get("items") or []
        channels.extend(items)
        total = int(data.get("total") or 0)
        if page * page_size >= total or not items:
            break
        page += 1
    return channels


def fetch_channel_aliases(admin_base: str, access_token: str,
                          channels: list[dict[str, Any]]) -> dict[str, str]:
    """从渠道偏好接口取渠道 ID→alias 映射（管理员可见 channel_id，与偏好页一致）。

    渠道可能出现在多个分组，alias 因分组而异；这里优先取渠道主分组（group 字段）
    对应的 alias，以便对外展示稳定一致。
    """
    url = admin_base.rstrip("/") + "/api/user/self/channel_preferences"
    body = http_json(url, headers={"Authorization": f"Bearer {access_token}"})
    if not body.get("success"):
        raise RuntimeError(f"获取渠道偏好失败: {body.get('message')}")

    primary_group = {ch.get("id"): ch.get("group") or "" for ch in channels}
    best: dict[int, tuple[str, str]] = {}  # id -> (alias, group)
    for group in body.get("data", {}).get("groups", []):
        group_name = group.get("group") or ""
        for option in group.get("channels", []):
            cid = option.get("channel_id")
            alias = option.get("alias")
            if not cid or not alias:
                continue
            current = best.get(cid)
            if current is None or group_name == primary_group.get(cid):
                best[cid] = (alias, group_name)
    return {str(cid): alias for cid, (alias, _) in best.items()}


def pick_model(models_str: str, pattern: str, fallback_model: str) -> tuple[str | None, str]:
    """从渠道 Models 里挑选匹配 pattern 的模型。返回 (model, reason)。"""
    models = [m.strip() for m in models_str.split(",") if m.strip()]
    if not models:
        return fallback_model, "渠道未配置 Models，使用回退模型"
    pat = re.compile(pattern, re.I)
    for m in models:
        if pat.search(m):
            return m, "匹配 Models"
    return None, f"Models 中无匹配 '{pattern}' 的模型"


def run_probe(args: list[str], env_extra: dict[str, str], log_path: Path) -> tuple[int, str]:
    """运行一次检测器子进程，返回 (exit_code, 末行摘要)。"""
    env = os.environ.copy()
    env.update(env_extra)
    with open(log_path, "w", encoding="utf-8") as logf:
        proc = subprocess.run(
            [sys.executable, str(DETECTOR_DIR / "gpt56_reasoning_probe.py"), *args],
            env=env, stdout=logf, stderr=subprocess.STDOUT, text=True,
        )
    tail = ""
    if log_path.exists():
        lines = log_path.read_text(encoding="utf-8", errors="replace").splitlines()
        tail = lines[-1].strip() if lines else ""
    return proc.returncode, tail


def channel_label(ch: dict[str, Any]) -> str:
    cid = ch.get("id")
    name = ch.get("name") or str(cid)
    return f"[{cid}] {name}"


def _e(v: Any) -> str:
    from html import escape
    return escape(str(v if v is not None else ""))


def main() -> None:
    gateway_base = required("GATEWAY_BASE_URL")      # 例如 https://gw.example.com/v1
    admin_base = required("NEWAPI_ADMIN_BASE_URL")   # 例如 https://gw.example.com
    access_token = required("NEWAPI_ADMIN_ACCESS_TOKEN")
    detector_key = required("DETECTOR_API_KEY")       # admin 账户 token 的原始 key（不含 sk-）
    mode = env("DETECT_MODE", "juice").lower()
    if mode not in ("juice", "full"):
        raise SystemExit(f"[scheduler] DETECT_MODE 仅支持 juice|full，当前: {mode}")
    model_pattern = env("MODEL_PATTERN", r"gpt-5\.6")
    fallback_model = env("FALLBACK_MODEL", "gpt-5.6-sol")
    timeout = float(env("DETECT_TIMEOUT", "180"))
    workers = int(env("DETECT_WORKERS", "1"))
    report_dir = Path(env("REPORT_DIR", "/reports"))
    report_dir.mkdir(parents=True, exist_ok=True)

    trusted_base = env("TRUSTED_BASE_URL")
    trusted_model = env("TRUSTED_MODEL", "gpt-5.6-sol")
    trusted_key = env("TRUSTED_API_KEY")
    if mode == "full":
        if not trusted_base or not trusted_key:
            raise SystemExit("[scheduler] full 模式需要 TRUSTED_BASE_URL 与 TRUSTED_API_KEY")

    type_names = {
        1: "OpenAI", 2: "Anthropic", 3: "Baidu", 5: "Zhipu", 6: "Ali",
        8: "Tencent", 9: "360", 10: "Moonshot", 12: "Xunfei", 15: "Gemini",
        18: "DeepSeek", 24: "Kimi", 30: "Ollama", 41: "NewAPI", 58: "AdvancedCustom",
    }

    log("[scheduler] 开始同步渠道...")
    channels = fetch_enabled_channels(admin_base, access_token)
    if not channels:
        log("[scheduler] 没有启用渠道，跳过本轮。")
        return
    log(f"[scheduler] 共 {len(channels)} 个启用渠道。")

    # 保存渠道 ID→真实名称映射，供历史页显示原名
    try:
        names = {str(ch.get("id")): (ch.get("name") or str(ch.get("id"))) for ch in channels}
        (report_dir / "owner-names.json").write_text(
            json.dumps(names, ensure_ascii=False), encoding="utf-8")
    except Exception as exc:
        log(f"[scheduler] 写入渠道名映射失败: {exc}")

    # 从网关「渠道偏好」接口取渠道 ID→alias 映射（与渠道偏好页完全一致），
    # 存为对外展示用的别名。该接口对管理员返回 channel_id 字段。
    try:
        aliases = fetch_channel_aliases(admin_base, access_token, channels)
        (report_dir / "aliases.json").write_text(
            json.dumps(aliases, ensure_ascii=False), encoding="utf-8")
        log(f"[scheduler] 已同步渠道别名 {len(aliases)} 个。")
    except Exception as exc:
        log(f"[scheduler] 同步渠道别名失败: {exc}")

    concurrency = int(env("DETECT_CONCURRENCY", "1"))
    if concurrency < 1:
        concurrency = 1

    cfg = {
        "gateway_base": gateway_base,
        "detector_key": detector_key,
        "mode": mode,
        "model_pattern": model_pattern,
        "fallback_model": fallback_model,
        "timeout": timeout,
        "workers": workers,
        "report_dir": report_dir,
        "trusted_base": trusted_base,
        "trusted_model": trusted_model,
        "trusted_key": trusted_key,
        "type_names": type_names,
    }

    with ThreadPoolExecutor(max_workers=concurrency, thread_name_prefix="detect") as pool:
        futures = [pool.submit(detect_channel, ch, cfg) for ch in channels if ch.get("id")]
        for future in as_completed(futures):
            future.result()

    log("[scheduler] 本轮结束，报告已更新。")


def detect_channel(ch: dict[str, Any], cfg: dict[str, Any]) -> dict[str, Any] | None:
    """检测单个渠道（可并发执行），返回用于汇总页的 meta dict。"""
    cid = ch.get("id")
    if not cid:
        return None
    name = ch.get("name") or str(cid)
    label = channel_label(ch)
    type_names = cfg["type_names"]
    report_dir = cfg["report_dir"]
    log(f"[scheduler] === 检测 {label} ===")

    model, model_reason = pick_model(ch.get("models") or "", cfg["model_pattern"], cfg["fallback_model"])
    if not model:
        log(f"[scheduler] 跳过 {label}：{model_reason}")
        return {
            "id": cid, "name": name, "kind": "skip", "title": model_reason,
            "passed": "", "model": "", "network": "",
            "time": time.strftime("%Y-%m-%d %H:%M:%S"), "json": None, "html": None,
        }

    ts = time.strftime("%Y%m%d-%H%M%S")
    json_path = report_dir / f"ch{cid}-{ts}.json"   # 文件名不含渠道名，避免泄露
    log_path = report_dir / f"ch{cid}-{ts}.log"

    env_extra = {
        "DETECT_CANDIDATE_KEY": f"sk-{cfg['detector_key']}-{cid}",
    }
    args = [
        "--candidate-base-url", cfg["gateway_base"],
        "--candidate-model", model,
        "--candidate-key-env", "DETECT_CANDIDATE_KEY",
        "--output", str(json_path),
        "--timeout", str(cfg["timeout"]),
        "--workers", str(cfg["workers"]),
    ]
    if cfg["mode"] == "juice":
        args.append("--juice-only")
    else:
        env_extra["DETECT_TRUSTED_KEY"] = cfg["trusted_key"]
        args += [
            "--trusted-base-url", cfg["trusted_base"],
            "--trusted-model", cfg["trusted_model"],
            "--trusted-key-env", "DETECT_TRUSTED_KEY",
        ]

    rc, tail = run_probe(args, env_extra, log_path)
    html_path = json_path.with_suffix(".html")
    if html_path.exists():
        html_path.unlink()  # 只保留数据(JSON)，不保存 HTML

    if not json_path.exists():
        log(f"[scheduler] 失败 {label}：未生成报告 rc={rc} tail={tail[:120]!r}")
        return {
            "id": cid, "name": name, "kind": "fail", "title": f"调用失败 (rc={rc}) {tail[:80]}",
            "passed": "", "model": "", "network": "",
            "time": time.strftime("%Y-%m-%d %H:%M:%S"), "json": json_path.name, "html": None,
        }

    try:
        report = json.loads(json_path.read_text(encoding="utf-8"))
    except Exception as exc:
        report = None
    cs = report.get("combined_summary") or {}
    juice = report.get("juice_summary") or {}
    net = report.get("network_summary") or {}
    verdict = cs.get("passed_cn") or cs.get("title_cn") or report.get("combined_verdict", "未知")
    raw_model = cs.get("juice_likely_model") or juice.get("likely_model_cn") or ""
    detail = (f"{verdict} {raw_model} {net.get('title_cn','') or ''} "
              f"conf={cs.get('juice_confidence','') or ''} rc={rc} tail={tail[:60]!r}")
    log(f"[scheduler] 完成 {label} -> {detail}")

    # 历史/汇总页统一从磁盘上的 JSON 重建，这里无需返回记录
    return None


if __name__ == "__main__":
    try:
        main()
    except SystemExit as exc:
        if exc.code:
            print(exc, file=sys.stderr, flush=True)
        raise
    except Exception as exc:  # noqa: BLE001
        print(f"[scheduler] 失败: {exc}", file=sys.stderr, flush=True)
        raise
