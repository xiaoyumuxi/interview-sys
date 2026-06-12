from typing import Any, Literal

from pydantic import BaseModel, Field


TaskType = Literal[
    "question_generation",
    "answer_evaluation",
    "follow_up_decision",
    "summary",
    "memory_extraction",
]

ProviderType = Literal["deepseek", "openai_compatible"]


class ChatMessage(BaseModel):
    role: Literal["system", "user", "assistant"]
    content: str


class ProviderConfig(BaseModel):
    provider_type: ProviderType = "deepseek"
    base_url: str = ""
    chat_endpoint_path: str = "/chat/completions"
    model: str = ""
    api_key: str = ""
    supports_json: bool = True


class ContextItem(BaseModel):
    id: str
    source_type: str
    source_id: str
    trust_level: str = "trusted"
    content: str
    score: float = 0
    reason: str = ""


class RuntimeTaskRequest(BaseModel):
    task_type: TaskType
    provider: ProviderConfig | None = None
    context_items: list[ContextItem] = Field(default_factory=list)
    user_input: str = ""
    output_schema: dict[str, Any] = Field(default_factory=dict)
    dry_run: bool = False


class RuntimeTaskResponse(BaseModel):
    schema_version: str = "runtime.task.v1"
    task_type: TaskType
    ok: bool
    output: dict[str, Any] = Field(default_factory=dict)
    raw_output: str = ""
    warnings: list[str] = Field(default_factory=list)
    trace: dict[str, Any] = Field(default_factory=dict)
