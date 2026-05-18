# syntax=docker/dockerfile:1.6
#
# citadel-detector — Python 3.11 + FastAPI + uvicorn + httpx.
#
# Pulls events from the backend on an interval, evaluates detection rules,
# POSTs detections back. See app/main.py for the lifespan wiring.

FROM python:3.11-slim AS runtime

ENV PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app

# Install deps from pyproject.toml. Using `pip install .` here lets us pin a
# single source of truth (no separate requirements.txt to drift).
COPY pyproject.toml ./
RUN pip install --upgrade pip && \
    pip install fastapi 'uvicorn[standard]>=0.27' httpx 'pydantic>=2.6' python-dateutil pyyaml

COPY app /app/app

EXPOSE 8000

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
