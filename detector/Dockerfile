FROM python:3.12-slim

# gpt56 检测器 + 调度/服务脚本均为纯标准库，无第三方依赖
WORKDIR /app

COPY gpt56/ /app/gpt56/
COPY scheduler.py runner.py web.py data.py /app/

VOLUME ["/reports"]
EXPOSE 8080

ENV REPORT_DIR=/reports \
    REPORT_HOST=0.0.0.0 \
    REPORT_PORT=8080 \
    DETECT_INTERVAL_MINUTES=60 \
    PYTHONUNBUFFERED=1

HEALTHCHECK --interval=5m --timeout=5s --start-period=30s \
    CMD python3 -c "import urllib.request;s=urllib.request.urlopen('http://127.0.0.1:8080/index.html', timeout=5).status;assert s==200" || exit 1

ENTRYPOINT ["python3", "/app/runner.py"]