#!/usr/bin/env python3
"""共享数据处理：读取原始检测 JSON，加工成可用于动态展示的数据集。

对外只暴露渠道"代号"（alias），绝不返回真实渠道名。
"""
from __future__ import annotations

import hashlib
import hmac
import json
import os
import re
from datetime import datetime
from pathlib import Path
from typing import Any

REPORT_RE = re.compile(r"ch(\d+)-(?:.*-)?(\d{8}-\d{6})\.json$")


def pretty_model(value: str) -> str:
    value = (value or "").strip()
    if not value:
        return "待定"
    return (value.replace("gpt_5_6", "GPT-5.6")
                 .replace("gpt_5_5", "GPT-5.5")
                 .replace("gpt_5_4_mini", "GPT-5.4 mini")
                 .replace("gpt_5_4", "GPT-5.4")
                 .replace("_", " "))


def fmt_time(value: str) -> str:
    value = (value or "").strip()
    if not value:
        return ""
    try:
        dt = datetime.fromisoformat(value.replace("Z", "+00:00"))
        return dt.astimezone().strftime("%Y-%m-%d %H:%M:%S")
    except Exception:
        return value


def alias_for(channel_id: int, secret: str) -> str:
    """由渠道 ID 派生稳定的展示代号（如 CH-A1B2），与渠道名无关。"""
    digest = hmac.new(secret.encode(), str(channel_id).encode(), hashlib.sha256).hexdigest()
    return "CH-" + digest[:6].upper()


def alias_secret() -> str:
    return os.environ.get("ALIAS_SECRET", "gpt56-detector-alias")


def load_aliases(report_dir: Path) -> dict[int, str]:
    """读取网关同步的渠道 ID→alias 映射（与渠道偏好页一致）。

    优先用 aliases.json（调度器每轮从网关同步）；缺失时退化为本地 HMAC 代号。
    """
    path = report_dir / "aliases.json"
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        aliases = {int(k): v for k, v in data.items()}
        if aliases:
            return aliases
    except Exception:
        pass
    secret = alias_secret()
    return {}


def display_alias(report_dir: Path, channel_id: int, aliases: dict[int, str]) -> str:
    if channel_id in aliases:
        return aliases[channel_id]
    return alias_for(channel_id, alias_secret())


def load_owner_names(report_dir: Path) -> dict[int, str]:
    """读取管理员私有映射（渠道 ID→真名），仅供管理员本地查看，web 不暴露。"""
    path = report_dir / "owner-names.json"
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        return {int(k): v for k, v in data.items()}
    except Exception:
        return {}


def extract_report(json_path: Path) -> dict[str, Any] | None:
    try:
        report = json.loads(json_path.read_text(encoding="utf-8"))
    except Exception:
        return None
    m = REPORT_RE.match(json_path.name)
    cid = int(m.group(1)) if m else 0
    cs = report.get("combined_summary") or {}
    juice = report.get("juice_summary") or {}
    net = report.get("network_summary") or {}
    return {
        "id": cid,
        "passed": cs.get("passed_cn") or "",
        "title": cs.get("title_cn") or report.get("combined_verdict") or "未知",
        "model": pretty_model(cs.get("juice_likely_model") or juice.get("likely_model_cn") or ""),
        "network": net.get("title_cn") or net.get("status") or "",
        "conf": cs.get("juice_confidence") or "",
        "time": fmt_time(str(report.get("created_at") or "")),
        "file": json_path.name,
    }


def scan_records(report_dir: Path) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    for p in sorted(report_dir.glob("ch[0-9]*-*.json")):
        rec = extract_report(p)
        if rec:
            records.append(rec)
    return records


def now_str() -> str:
    return datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S")


EFFORT_CN = {"low": "低档", "medium": "中档", "high": "高档", "xhigh": "超高档", "max": "最高档"}


def report_detail(json_path: Path) -> dict[str, Any] | None:
    """把单份检测 JSON 加工成可读明细（结论、线路、分档 juice、字面量对照）。"""
    try:
        report = json.loads(json_path.read_text(encoding="utf-8"))
    except Exception:
        return None
    cs = report.get("combined_summary") or {}
    net = report.get("network_summary") or {}
    juice = report.get("juice_summary") or {}
    olt = report.get("output_literal_control_summary") or {}

    efforts: dict[str, dict[str, Any]] = {}
    for obs in report.get("juice_observations") or []:
        effort = obs.get("effort") or ""
        if effort not in efforts:
            efforts[effort] = {"ok": 0, "error": 0, "numbers": []}
        st = obs.get("status") or ""
        if st == "ok":
            num = obs.get("normalized_value") or obs.get("raw_output") or ""
            efforts[effort]["ok"] += 1
            if num:
                efforts[effort]["numbers"].append(str(num))
        else:
            efforts[effort]["error"] += 1

    effort_rows = []
    for effort, data in efforts.items():
        numbers = data["numbers"][:8]
        effort_rows.append({
            "effort_cn": EFFORT_CN.get(effort, effort),
            "ok": data["ok"],
            "error": data["error"],
            "numbers": "、".join(numbers) + ("…" if len(data["numbers"]) > 8 else ""),
        })

    return {
        "time": fmt_time(str(report.get("created_at") or "")),
        "mode": str(report.get("mode") or ""),
        "passed": cs.get("passed_cn") or "",
        "title": cs.get("title_cn") or report.get("combined_verdict") or "未知",
        "explanation": cs.get("explanation_cn") or report.get("reason") or "",
        "model": pretty_model(cs.get("juice_likely_model") or juice.get("likely_model_cn") or ""),
        "conf": cs.get("juice_confidence") or "",
        "network": net.get("title_cn") or net.get("status") or "",
        "network_detail": net.get("detail_cn") or "",
        "literal_control": olt.get("title_cn") or olt.get("status") or "",
        "efforts": effort_rows,
    }


def process(report_dir: Path) -> dict[str, Any]:
    """把原始检测数据加工成展示数据集（只含代号，不含真名）。"""
    aliases = load_aliases(report_dir)
    records = scan_records(report_dir)

    def alias(cid: int) -> str:
        return display_alias(report_dir, cid, aliases)

    by_channel: dict[int, list[dict[str, Any]]] = {}
    for rec in records:
        by_channel.setdefault(rec["id"], []).append(rec)
    for lst in by_channel.values():
        lst.sort(key=lambda r: r["time"] or "")

    channels = []
    for cid in sorted(by_channel):
        hist = by_channel[cid]
        latest = hist[-1]
        passed_counts = {"辅助通过": 0, "尚未判定": 0, "不通过": 0}
        for r in hist:
            passed_counts[r["passed"]] = passed_counts.get(r["passed"], 0) + 1
        total = len(hist)
        channels.append({
            "id": cid,
            "alias": alias(cid),
            "latest": latest,
            "detections": total,
            "pass_count": passed_counts.get("辅助通过", 0),
            "pending_count": passed_counts.get("尚未判定", 0),
            "fail_count": passed_counts.get("不通过", 0),
            "pass_rate": round(passed_counts.get("辅助通过", 0) / total * 100) if total else 0,
        })

    stats = {
        "channels": len(channels),
        "detections": len(records),
        "passed": sum(c["pass_count"] for c in channels),
        "pending": sum(c["pending_count"] for c in channels),
        "failed": sum(c["fail_count"] for c in channels),
    }

    history = []
    for rec in sorted(records, key=lambda r: r["time"] or "", reverse=True):
        history.append({
            "id": rec["id"],
            "alias": alias(rec["id"]),
            "passed": rec["passed"],
            "title": rec["title"],
            "model": rec["model"],
            "network": rec["network"],
            "conf": rec["conf"],
            "time": rec["time"],
            "file": rec["file"],
        })

    return {
        "generated_at": datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S"),
        "stats": stats,
        "channels": channels,
        "history": history,
    }
