from fastapi import FastAPI, HTTPException

from app.config import settings
from app.models import RuntimeTaskRequest, RuntimeTaskResponse
from app.tasks import run_task

app = FastAPI(title="AI Interview Python Runtime", version="0.1.0")


@app.get("/healthz")
async def healthz() -> dict:
    return {
        "status": "ok",
        "schema_version": "runtime.health.v1",
        "app_env": settings.app_env,
    }


@app.post("/api/runtime/tasks", response_model=RuntimeTaskResponse)
async def runtime_task(request: RuntimeTaskRequest) -> RuntimeTaskResponse:
    try:
        return await run_task(request)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
