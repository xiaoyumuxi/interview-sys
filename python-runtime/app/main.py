from fastapi import FastAPI, HTTPException

from app.config import settings
from app.memory import MemoryCandidateCreate, MemoryCandidateEdit, MemoryReviewRequest, MemoryStore
from app.models import RuntimeTaskRequest, RuntimeTaskResponse
from app.tasks import run_task

app = FastAPI(title="AI Interview Python Runtime", version="0.1.0")
memory_store = MemoryStore(settings.memory_db_path)


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


@app.get("/api/runtime/memory/candidates")
async def list_memory_candidates(user_id: str = "", status: str = "", limit: int = 100) -> dict:
	return {
		"schema_version": "runtime.memory.candidates.v1",
		"items": memory_store.list_candidates(user_id=user_id, status=status, limit=limit),
	}


@app.post("/api/runtime/memory/candidates", status_code=201)
async def create_memory_candidate(request: MemoryCandidateCreate) -> dict:
	try:
		item = memory_store.create_candidate(request)
	except ValueError as exc:
		raise HTTPException(status_code=400, detail=str(exc)) from exc
	return {"schema_version": "runtime.memory.candidate.v1", "item": item}


@app.post("/api/runtime/memory/candidates/{candidate_id}/approve")
async def approve_memory_candidate(candidate_id: str, request: MemoryReviewRequest) -> dict:
	try:
		item = memory_store.approve(candidate_id, request)
	except KeyError as exc:
		raise HTTPException(status_code=404, detail=str(exc)) from exc
	return {"schema_version": "runtime.memory.candidate.v1", "item": item}


@app.post("/api/runtime/memory/candidates/{candidate_id}/reject")
async def reject_memory_candidate(candidate_id: str, request: MemoryReviewRequest) -> dict:
	try:
		item = memory_store.reject(candidate_id, request)
	except KeyError as exc:
		raise HTTPException(status_code=404, detail=str(exc)) from exc
	return {"schema_version": "runtime.memory.candidate.v1", "item": item}


@app.post("/api/runtime/memory/candidates/{candidate_id}/edit")
async def edit_memory_candidate(candidate_id: str, request: MemoryCandidateEdit) -> dict:
	try:
		item = memory_store.edit(candidate_id, request)
	except KeyError as exc:
		raise HTTPException(status_code=404, detail=str(exc)) from exc
	except ValueError as exc:
		raise HTTPException(status_code=400, detail=str(exc)) from exc
	return {"schema_version": "runtime.memory.candidate.v1", "item": item}


@app.get("/api/runtime/memory/profile")
async def get_memory_profile(user_id: str = "") -> dict:
	return memory_store.profile(user_id=user_id)


@app.get("/api/runtime/memory/search")
async def search_memory(user_id: str = "", q: str = "", limit: int = 20) -> dict:
	return {
		"schema_version": "runtime.memory.search.v1",
		"items": memory_store.search(user_id=user_id, query=q, limit=limit),
	}


@app.get("/api/runtime/reviews/due")
async def list_due_reviews(user_id: str = "", limit: int = 50) -> dict:
	return {
		"schema_version": "runtime.reviews.due.v1",
		"items": memory_store.due_reviews(user_id=user_id, limit=limit),
	}
