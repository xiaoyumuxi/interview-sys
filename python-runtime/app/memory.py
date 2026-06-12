from __future__ import annotations

import json
import math
import sqlite3
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any
from uuid import uuid4

from pydantic import BaseModel, Field


MEMORY_TYPES = {
    "weak_point",
    "strong_point",
    "behavior_signal",
    "preference",
    "project_fact",
    "review_task",
}


class EvidenceItem(BaseModel):
    source_type: str = ""
    session_id: str = ""
    question: str = ""
    answer_excerpt: str = ""
    evaluation_excerpt: str = ""
    reason: str = ""


class MemoryCandidateCreate(BaseModel):
    user_id: str = "local-user"
    type: str
    topic: str = ""
    content: str
    evidence: list[EvidenceItem] = Field(default_factory=list)
    confidence: float = 0
    source_session_id: str = ""
    source_answer_id: str = ""
    conflicts_with: list[str] = Field(default_factory=list)


class MemoryReviewRequest(BaseModel):
    review_note: str = ""


class MemoryCandidateEdit(BaseModel):
    type: str = ""
    topic: str = ""
    content: str = ""
    evidence: list[EvidenceItem] | None = None
    confidence: float | None = None
    conflicts_with: list[str] = Field(default_factory=list)
    review_note: str = ""


@dataclass
class MemoryStore:
    path: str

    def __post_init__(self) -> None:
        db_path = Path(self.path)
        db_path.parent.mkdir(parents=True, exist_ok=True)
        with self._connect() as conn:
            conn.executescript(
                """
                CREATE TABLE IF NOT EXISTS memory_candidates (
                    candidate_id TEXT PRIMARY KEY,
                    user_id TEXT NOT NULL,
                    type TEXT NOT NULL,
                    topic TEXT NOT NULL DEFAULT '',
                    content TEXT NOT NULL,
                    evidence TEXT NOT NULL DEFAULT '[]',
                    confidence REAL NOT NULL DEFAULT 0,
                    source_session_id TEXT NOT NULL DEFAULT '',
                    source_answer_id TEXT NOT NULL DEFAULT '',
                    conflicts_with TEXT NOT NULL DEFAULT '[]',
                    status TEXT NOT NULL DEFAULT 'pending',
                    review_note TEXT NOT NULL DEFAULT '',
                    created_at REAL NOT NULL,
                    updated_at REAL NOT NULL,
                    reviewed_at REAL
                );
                CREATE INDEX IF NOT EXISTS idx_memory_candidates_user_status
                    ON memory_candidates (user_id, status, updated_at DESC);
                CREATE INDEX IF NOT EXISTS idx_memory_candidates_topic
                    ON memory_candidates (topic, type);

                CREATE TABLE IF NOT EXISTS memory_profile_projections (
                    user_id TEXT PRIMARY KEY,
                    profile TEXT NOT NULL DEFAULT '{}',
                    source_candidate_ids TEXT NOT NULL DEFAULT '[]',
                    updated_at REAL NOT NULL
                );

                CREATE TABLE IF NOT EXISTS review_tasks (
                    review_id TEXT PRIMARY KEY,
                    user_id TEXT NOT NULL,
                    candidate_id TEXT NOT NULL UNIQUE,
                    topic TEXT NOT NULL DEFAULT '',
                    content TEXT NOT NULL,
                    status TEXT NOT NULL DEFAULT 'due',
                    due_at REAL NOT NULL,
                    interval_days INTEGER NOT NULL DEFAULT 1,
                    ease_factor REAL NOT NULL DEFAULT 2.5,
                    repetition_count INTEGER NOT NULL DEFAULT 0,
                    last_reviewed_at REAL,
                    created_at REAL NOT NULL,
                    updated_at REAL NOT NULL
                );
                CREATE INDEX IF NOT EXISTS idx_review_tasks_due
                    ON review_tasks (user_id, status, due_at);
                """
            )

    def create_candidate(self, request: MemoryCandidateCreate) -> dict[str, Any]:
        self._validate_candidate(request.type, request.content)
        now = time.time()
        user_id = request.user_id.strip() or "local-user"
        conflicts = request.conflicts_with or self._detect_conflicts(
            user_id=user_id,
            memory_type=request.type,
            topic=request.topic,
            content=request.content,
        )
        candidate_id = "mem_" + uuid4().hex
        with self._connect() as conn:
            conn.execute(
                """
                INSERT INTO memory_candidates (
                    candidate_id, user_id, type, topic, content, evidence, confidence,
                    source_session_id, source_answer_id, conflicts_with, status,
                    created_at, updated_at
                ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
                """,
                (
                    candidate_id,
                    user_id,
                    request.type,
                    request.topic.strip(),
                    request.content.strip(),
                    self._json([item.model_dump() for item in request.evidence]),
                    self._clamp(request.confidence),
                    request.source_session_id.strip(),
                    request.source_answer_id.strip(),
                    self._json(conflicts),
                    "pending",
                    now,
                    now,
                ),
            )
        return self.get_candidate(candidate_id)

    def list_candidates(self, user_id: str = "", status: str = "", limit: int = 100) -> list[dict[str, Any]]:
        limit = self._limit(limit, 200)
        conditions: list[str] = []
        args: list[Any] = []
        if user_id.strip():
            conditions.append("user_id = ?")
            args.append(user_id.strip())
        if status.strip():
            conditions.append("status = ?")
            args.append(status.strip())
        where = "WHERE " + " AND ".join(conditions) if conditions else ""
        with self._connect() as conn:
            rows = conn.execute(
                f"""
                SELECT * FROM memory_candidates
                {where}
                ORDER BY updated_at DESC
                LIMIT ?
                """,
                (*args, limit),
            ).fetchall()
        return [self._candidate_from_row(row) for row in rows]

    def get_candidate(self, candidate_id: str) -> dict[str, Any]:
        with self._connect() as conn:
            row = conn.execute("SELECT * FROM memory_candidates WHERE candidate_id = ?", (candidate_id,)).fetchone()
        if row is None:
            raise KeyError("memory candidate not found")
        return self._candidate_from_row(row)

    def approve(self, candidate_id: str, request: MemoryReviewRequest) -> dict[str, Any]:
        item = self._set_status(candidate_id, "approved", request.review_note)
        self._rebuild_profile(item["user_id"])
        if item["type"] in {"weak_point", "review_task"}:
            self._ensure_review_task(item)
        return self.get_candidate(candidate_id)

    def reject(self, candidate_id: str, request: MemoryReviewRequest) -> dict[str, Any]:
        item = self._set_status(candidate_id, "rejected", request.review_note)
        self._rebuild_profile(item["user_id"])
        return item

    def edit(self, candidate_id: str, request: MemoryCandidateEdit) -> dict[str, Any]:
        current = self.get_candidate(candidate_id)
        memory_type = request.type.strip() or current["type"]
        topic = request.topic.strip() or current["topic"]
        content = request.content.strip() or current["content"]
        confidence = current["confidence"] if request.confidence is None else self._clamp(request.confidence)
        evidence = current["evidence"] if request.evidence is None else [item.model_dump() for item in request.evidence]
        self._validate_candidate(memory_type, content)
        now = time.time()
        with self._connect() as conn:
            conn.execute(
                """
                UPDATE memory_candidates
                SET type=?, topic=?, content=?, evidence=?, confidence=?, conflicts_with=?,
                    status='edited', review_note=?, reviewed_at=?, updated_at=?
                WHERE candidate_id=?
                """,
                (
                    memory_type,
                    topic,
                    content,
                    self._json(evidence),
                    confidence,
                    self._json(request.conflicts_with),
                    request.review_note.strip(),
                    now,
                    now,
                    candidate_id,
                ),
            )
        item = self.get_candidate(candidate_id)
        self._rebuild_profile(item["user_id"])
        return item

    def profile(self, user_id: str = "") -> dict[str, Any]:
        user_id = user_id.strip() or "local-user"
        with self._connect() as conn:
            row = conn.execute(
                "SELECT profile, source_candidate_ids, updated_at FROM memory_profile_projections WHERE user_id=?",
                (user_id,),
            ).fetchone()
        if row is None:
            return {
                "schema_version": "runtime.memory.profile.v1",
                "user_id": user_id,
                "items": [],
                "by_type": {},
                "source_candidate_ids": [],
            }
        profile = json.loads(row["profile"])
        profile["source_candidate_ids"] = json.loads(row["source_candidate_ids"])
        profile["updated_at"] = self._iso(row["updated_at"])
        return profile

    def search(self, user_id: str = "", query: str = "", limit: int = 20) -> list[dict[str, Any]]:
        user_id = user_id.strip() or "local-user"
        query = query.strip()
        limit = self._limit(limit, 50)
        like = f"%{query}%"
        with self._connect() as conn:
            rows = conn.execute(
                """
                SELECT * FROM memory_candidates
                WHERE user_id=? AND status='approved'
                  AND (?='' OR topic LIKE ? OR content LIKE ?)
                ORDER BY updated_at DESC
                LIMIT ?
                """,
                (user_id, query, like, like, limit),
            ).fetchall()
        results = []
        for row in rows:
            candidate = self._candidate_from_row(row)
            score, reasons = self._memory_context_score(candidate, query)
            results.append(
                {
                    "candidate": candidate,
                    "memory_context_score": score,
                    "reasons": reasons,
                }
            )
        return results

    def due_reviews(self, user_id: str = "", limit: int = 50) -> list[dict[str, Any]]:
        user_id = user_id.strip() or "local-user"
        limit = self._limit(limit, 100)
        now = time.time()
        with self._connect() as conn:
            rows = conn.execute(
                """
                SELECT * FROM review_tasks
                WHERE user_id=? AND status IN ('due','scheduled') AND due_at <= ?
                ORDER BY due_at, updated_at DESC
                LIMIT ?
                """,
                (user_id, now, limit),
            ).fetchall()
        return [self._review_from_row(row) for row in rows]

    def _set_status(self, candidate_id: str, status: str, note: str) -> dict[str, Any]:
        now = time.time()
        with self._connect() as conn:
            cur = conn.execute(
                """
                UPDATE memory_candidates
                SET status=?, review_note=?, reviewed_at=?, updated_at=?
                WHERE candidate_id=?
                """,
                (status, note.strip(), now, now, candidate_id),
            )
        if cur.rowcount == 0:
            raise KeyError("memory candidate not found")
        return self.get_candidate(candidate_id)

    def _rebuild_profile(self, user_id: str) -> None:
        items = self.list_candidates(user_id=user_id, status="approved", limit=500)
        by_type: dict[str, int] = {}
        profile_items = []
        source_ids = []
        for item in items:
            by_type[item["type"]] = by_type.get(item["type"], 0) + 1
            source_ids.append(item["candidate_id"])
            profile_items.append(
                {
                    "candidate_id": item["candidate_id"],
                    "type": item["type"],
                    "topic": item["topic"],
                    "content": item["content"],
                    "confidence": item["confidence"],
                    "evidence": item["evidence"],
                    "approved_at": item.get("reviewed_at", ""),
                }
            )
        profile = {
            "schema_version": "runtime.memory.profile.v1",
            "user_id": user_id,
            "items": profile_items,
            "by_type": by_type,
            "source_candidate_ids": source_ids,
        }
        now = time.time()
        with self._connect() as conn:
            conn.execute(
                """
                INSERT INTO memory_profile_projections (user_id, profile, source_candidate_ids, updated_at)
                VALUES (?, ?, ?, ?)
                ON CONFLICT(user_id) DO UPDATE SET
                    profile=excluded.profile,
                    source_candidate_ids=excluded.source_candidate_ids,
                    updated_at=excluded.updated_at
                """,
                (user_id, self._json(profile), self._json(source_ids), now),
            )

    def _ensure_review_task(self, item: dict[str, Any]) -> None:
        now = time.time()
        with self._connect() as conn:
            conn.execute(
                """
                INSERT OR IGNORE INTO review_tasks (
                    review_id, user_id, candidate_id, topic, content, status, due_at,
                    interval_days, ease_factor, repetition_count, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, 'due', ?, 1, 2.5, 0, ?, ?)
                """,
                (
                    "rev_" + uuid4().hex,
                    item["user_id"],
                    item["candidate_id"],
                    item["topic"],
                    item["content"],
                    now,
                    now,
                    now,
                ),
            )

    def _detect_conflicts(self, user_id: str, memory_type: str, topic: str, content: str) -> list[str]:
        with self._connect() as conn:
            rows = conn.execute(
                """
                SELECT candidate_id FROM memory_candidates
                WHERE user_id=? AND status='approved' AND type=? AND topic=? AND content <> ?
                ORDER BY updated_at DESC
                LIMIT 10
                """,
                (user_id, memory_type, topic.strip(), content.strip()),
            ).fetchall()
        return [row["candidate_id"] for row in rows]

    def _candidate_from_row(self, row: sqlite3.Row) -> dict[str, Any]:
        return {
            "candidate_id": row["candidate_id"],
            "user_id": row["user_id"],
            "type": row["type"],
            "topic": row["topic"],
            "content": row["content"],
            "evidence": json.loads(row["evidence"]),
            "confidence": row["confidence"],
            "source_session_id": row["source_session_id"],
            "source_answer_id": row["source_answer_id"],
            "conflicts_with": json.loads(row["conflicts_with"]),
            "status": row["status"],
            "review_note": row["review_note"],
            "created_at": self._iso(row["created_at"]),
            "updated_at": self._iso(row["updated_at"]),
            "reviewed_at": self._iso(row["reviewed_at"]) if row["reviewed_at"] else "",
        }

    def _review_from_row(self, row: sqlite3.Row) -> dict[str, Any]:
        return {
            "review_id": row["review_id"],
            "user_id": row["user_id"],
            "candidate_id": row["candidate_id"],
            "topic": row["topic"],
            "content": row["content"],
            "status": row["status"],
            "due_at": self._iso(row["due_at"]),
            "interval_days": row["interval_days"],
            "ease_factor": row["ease_factor"],
            "repetition_count": row["repetition_count"],
            "last_reviewed_at": self._iso(row["last_reviewed_at"]) if row["last_reviewed_at"] else "",
        }

    def _memory_context_score(self, item: dict[str, Any], query: str) -> tuple[float, dict[str, float]]:
        q = query.lower().strip()
        text = (item["topic"] + " " + item["content"]).lower()
        task_match = 1.0 if not q or q in text else 0.2
        evidence_score = self._clamp(item["confidence"])
        review_priority = 1.0 if item["type"] in {"weak_point", "review_task"} else 0.5
        recency = self._recency_score(item["updated_at"])
        repeat = self._clamp(len(item["conflicts_with"]) * 0.25)
        confirmed = 1.0
        score = (
            task_match * 0.30
            + evidence_score * 0.25
            + review_priority * 0.20
            + recency * 0.10
            + repeat * 0.10
            + confirmed * 0.05
        )
        return round(score, 4), {
            "task_match_score": task_match,
            "evidence_score": evidence_score,
            "review_priority_score": review_priority,
            "recency_score": recency,
            "repeat_score": repeat,
            "user_confirmed_score": confirmed,
        }

    def _recency_score(self, value: str) -> float:
        try:
            days = (time.time() - time.mktime(time.strptime(value, "%Y-%m-%dT%H:%M:%SZ"))) / 86400
        except ValueError:
            return 0.5
        if days <= 7:
            return 1.0
        if days <= 30:
            return 0.7
        if days <= 90:
            return 0.4
        return 0.2

    def _connect(self) -> sqlite3.Connection:
        conn = sqlite3.connect(self.path)
        conn.row_factory = sqlite3.Row
        return conn

    @staticmethod
    def _validate_candidate(memory_type: str, content: str) -> None:
        if memory_type not in MEMORY_TYPES:
            raise ValueError(f"invalid memory type: {memory_type}")
        if not content.strip():
            raise ValueError("content is required")

    @staticmethod
    def _clamp(value: float, low: float = 0, high: float = 1) -> float:
        if math.isnan(value):
            return low
        return max(low, min(high, value))

    @staticmethod
    def _limit(value: int, maximum: int) -> int:
        if value <= 0:
            return min(100, maximum)
        return min(value, maximum)

    @staticmethod
    def _json(value: Any) -> str:
        return json.dumps(value, ensure_ascii=False, separators=(",", ":"))

    @staticmethod
    def _iso(value: float) -> str:
        return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(value))
