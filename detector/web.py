#!/usr/bin/env python3
"""gpt5.6 检测数据动态展示服务（纯标准库，亮色主题）。

- 只保存数据（JSON），页面全部实时渲染，不落盘 HTML。
- 渠道一律用「渠道偏好页同款 alias」展示，绝不暴露真实渠道名。
- 历史按天归档/筛选，避免单页数据量无限增长。
"""
from __future__ import annotations

import functools
import http.server
import json
import os
import socketserver
from datetime import datetime
from html import escape
from pathlib import Path
from urllib.parse import unquote

import data

REPORT_DIR = Path(os.environ.get("REPORT_DIR", "/reports")).resolve()
HOST = os.environ.get("REPORT_HOST", "0.0.0.0")
PORT = int(os.environ.get("REPORT_PORT", "8080"))


def _e(v) -> str:
    return escape(str(v if v is not None else ""))


PASS_CLS = {"辅助通过": "ok", "尚未判定": "warn", "不通过": "err"}


CSS = """
 body{font-family:system-ui,sans-serif;margin:2rem;background:#f6f8fa;color:#1f2328}
 h1{font-size:1.3rem;color:#111} .meta{color:#57606a;font-size:.85rem;margin-bottom:1rem;display:flex;gap:1rem;flex-wrap:wrap}
 table{border-collapse:collapse;width:100%;background:#fff;border:1px solid #d0d7de;border-radius:8px;overflow:hidden}
 th,td{border-bottom:1px solid #d0d7de;padding:.5rem .7rem;font-size:.9rem;text-align:left}
 th{background:#f0f3f6;position:sticky;top:0}
 tr:last-child td{border-bottom:none}
 tr:nth-child(even) td{background:#fafbfc}
 a{color:#0969da;text-decoration:none}
 .ok{color:#1a7f37;font-weight:600} .warn{color:#9a6700;font-weight:600} .err{color:#cf222e;font-weight:600}
 .cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(160px,1fr));gap:.8rem;margin:1rem 0}
 .card{background:#fff;border:1px solid #d0d7de;border-radius:8px;padding:.8rem;text-align:center}
 .card .num{font-size:1.6rem;font-weight:700}
 .detail{background:#fff;border:1px solid #d0d7de;border-radius:8px;padding:1rem 1.2rem;margin-bottom:1rem}
 .detail h2{margin:.2rem 0 .6rem;font-size:1.05rem}
 .kpi{display:flex;gap:2rem;flex-wrap:wrap;color:#24292f}
 .kpi b{font-size:1.1rem}
 nav{margin-bottom:1rem} nav a{margin-right:1rem}
 form.filter{margin:0 0 1rem}
"""


def _page(title: str, body: str) -> bytes:
    return f"""<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{_e(title)}</title><style>{CSS}</style></head><body>
<h1>{_e(title)}</h1>
<div class="meta"><span>生成时间：{_e(data.now_str())}</span><span>数据源：{_e(REPORT_DIR)}</span></div>
{body}
</body></html>""".encode("utf-8")


def _nav() -> str:
    return ("<nav><a href='/'>总览</a><a href='/history'>历史总览</a>"
            "<a href='/api/data.json'>数据(JSON)</a></nav>")


def _stat_cards(stats: dict) -> str:
    return f"""<div class="cards">
<div class="card"><div class="num">{stats['channels']}</div><div>渠道</div></div>
<div class="card"><div class="num">{stats['detections']}</div><div>累计检测</div></div>
<div class="card"><div class="num ok">{stats['passed']}</div><div>通过</div></div>
<div class="card"><div class="num warn">{stats['pending']}</div><div>待定</div></div>
<div class="card"><div class="num err">{stats['failed']}</div><div>不通过</div></div>
</div>"""


def _index(ds: dict) -> bytes:
    stats = ds["stats"]
    rows = []
    for ch in ds["channels"]:
        latest = ch["latest"]
        cls = PASS_CLS.get(latest["passed"], "")
        rows.append(
            f"<tr><td><a href='/channel/{_e(ch['alias'])}'>{_e(ch['alias'])}</a></td>"
            f"<td class='{cls}'>{_e(latest['passed'])}</td>"
            f"<td>{_e(latest['model'])}</td>"
            f"<td>{_e(latest['network'])}</td>"
            f"<td>{ch['pass_rate']}%</td>"
            f"<td>{ch['detections']}</td>"
            f"<td>{_e(latest['time'])}</td></tr>")
    body = (_nav() + _stat_cards(stats) + f"""<table><thead><tr>
<th>渠道代号</th><th>最新结论</th><th>型号</th><th>线路</th>
<th>通过率</th><th>检测次数</th><th>最近检测</th></tr></thead>
<tbody>{''.join(rows)}</tbody></table>""")
    return _page("gpt5.6 渠道检测总览", body)


def _history(ds: dict, only_alias: str | None = None, date: str | None = None) -> bytes:
    """历史总览，按天分组展示。"""
    by_day: dict[str, list[dict]] = {}
    for h in ds["history"]:
        if only_alias and h["alias"] != only_alias:
            continue
        day = (h["time"] or "")[:10]
        if date and day != date:
            continue
        by_day.setdefault(day, []).append(h)

    sections = []
    for day in sorted(by_day, reverse=True):
        day_rows = []
        for h in by_day[day]:
            cls = PASS_CLS.get(h["passed"], "")
            alias_cell = f"<a href='/channel/{_e(h['alias'])}'>{_e(h['alias'])}</a>"
            day_rows.append(
                f"<tr><td>{alias_cell}</td>"
                f"<td class='{cls}'>{_e(h['passed'])}</td>"
                f"<td>{_e(h['model'])}</td>"
                f"<td>{_e(h['network'])}</td>"
                f"<td>{_e(h['time'])}</td>"
                f"<td><a href='/detail/{_e(h['file'])}'>明细</a></td></tr>")
        sections.append(
            f"<h2>{day}</h2><table><thead><tr><th>渠道代号</th><th>结论</th>"
            f"<th>型号</th><th>线路</th><th>时间</th><th>明细</th></tr></thead>"
            f"<tbody>{''.join(day_rows)}</tbody></table>")

    if not sections:
        sections = ["<p>暂无数据</p>"]

    if only_alias:
        title = f"渠道 {only_alias} 检测历史"
        nav = "<nav><a href='/'>总览</a><a href='/history'>历史总览</a></nav>"
    else:
        title = "gpt5.6 全部检测历史"
        nav = _nav()

    filter_form = ""
    if not only_alias:
        days = sorted({(h["time"] or "")[:10] for h in ds["history"] if h["time"]}, reverse=True)
        opts = "".join(f"<option value='{d}'{' selected' if d == date else ''}>{d}</option>" for d in days)
        filter_form = ("<form class='filter' method='get' action='/history'>按天筛选："
                       f"<select name='date'><option value=''>全部</option>{opts}</select>"
                       "<input type='submit' value='查看'></form>")

    return _page(title, nav + filter_form + "".join(sections))


def _detail(ds: dict, fname: str) -> bytes:
    """单份检测的可读明细页（替代原始 JSON）。"""
    fpath = (REPORT_DIR / fname).resolve()
    if not str(fpath).startswith(str(REPORT_DIR.resolve())) or not fname.startswith("ch") or not fname.endswith(".json"):
        return None
    detail = data.report_detail(fpath)
    if detail is None:
        return None

    effort_rows = []
    for e in detail["efforts"]:
        effort_rows.append(
            f"<tr><td>{_e(e['effort_cn'])}</td><td>{e['ok']}</td><td>{e['error']}</td>"
            f"<td>{_e(e['numbers'])}</td></tr>")
    efforts_table = f"""<table><thead><tr><th>档位</th><th>有效</th><th>错误</th>
<th>数字样本</th></tr></thead><tbody>{''.join(effort_rows)}</tbody></table>"""

    cls = PASS_CLS.get(detail["passed"], "")
    kpi = f"""<div class="kpi">
<div>结论：<b class='{cls}'>{_e(detail['passed'])}</b> <span class='meta'>{_e(detail['title'])}</span></div>
<div>型号：<b>{_e(detail['model'])}</b> <span class='meta'>置信度 {_e(detail['conf'])}</span></div>
<div>线路：<b>{_e(detail['network'])}</b></div>
<div>时间：{_e(detail['time'])}</div></div>"""
    body = ("<nav><a href='/'>总览</a>"
            f"<a href='/history'>历史总览</a></nav>"
            f"<div class='detail'>{kpi}"
            f"<p>{_e(detail['explanation'])}</p>"
            f"<p><small>{_e(detail['network_detail'])} · 字面量对照：{_e(detail['literal_control'])}</small></p>"
            f"</div>{efforts_table}")
    return _page(f"检测明细 {_e(fname)}", body)


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):  # noqa: C901
        parsed = self.path.split("?", 1)
        path = unquote(parsed[0])
        query = {}
        if len(parsed) > 1:
            for kv in parsed[1].split("&"):
                if "=" in kv:
                    k, v = kv.split("=", 1)
                    query[k] = v
        ds = data.process(REPORT_DIR)
        try:
            if path in ("/", "/index.html"):
                body = _index(ds)
            elif path == "/history":
                body = _history(ds, date=query.get("date"))
            elif path == "/api/data.json":
                self._send(200, json.dumps(ds, ensure_ascii=False).encode("utf-8"), "application/json; charset=utf-8")
                return
            elif path.startswith("/channel/"):
                alias = path[len("/channel/"):]
                if not any(ch["alias"] == alias for ch in ds["channels"]):
                    self._send(404, b"not found", "text/plain")
                    return
                body = _history(ds, only_alias=alias)
            elif path.startswith("/detail/"):
                fname = path[len("/detail/"):]
                body = _detail(ds, fname)
                if body is None:
                    self._send(404, b"not found", "text/plain")
                    return
            else:
                self._send(404, b"not found", "text/plain")
                return
        except Exception as exc:  # noqa: BLE001
            self._send(500, str(exc).encode("utf-8"), "text/plain")
            return
        self._send(200, body, "text/html; charset=utf-8")

    def _send(self, code: int, body: bytes, ctype: str) -> None:
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args) -> None:
        pass


def main() -> None:
    REPORT_DIR.mkdir(parents=True, exist_ok=True)
    socketserver.ThreadingTCPServer.allow_reuse_address = True
    server = socketserver.ThreadingTCPServer((HOST, PORT), functools.partial(Handler))
    print(f"[web] dynamic report server: {REPORT_DIR} on http://{HOST}:{PORT}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()