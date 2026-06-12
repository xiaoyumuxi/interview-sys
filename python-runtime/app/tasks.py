from app.models import ChatMessage, RuntimeTaskRequest, RuntimeTaskResponse
from app.prompt_security import ANTI_INJECTION_INSTRUCTION, scan_prompt_injection, wrap_user_data
from app.providers import chat_completion, resolve_provider
from app.structured_output import STRICT_JSON_INSTRUCTION, parse_json_object


TASK_INSTRUCTIONS = {
    "question_generation": "生成一道面试问题，输出 JSON: {\"question\":\"...\",\"intent\":\"...\",\"rubric\":[\"...\"]}",
    "answer_evaluation": "评估候选人答案，输出 JSON: {\"score\":0,\"strengths\":[],\"weaknesses\":[],\"evidence\":[]}",
    "follow_up_decision": "判断是否追问，输出 JSON: {\"should_follow_up\":true,\"question\":\"...\",\"reason\":\"...\"}",
    "summary": "总结本次会话，输出 JSON: {\"summary\":\"...\",\"key_points\":[],\"risks\":[]}",
    "memory_extraction": "只提取候选记忆，必须 pending，输出 JSON: {\"candidates\":[]}",
}


async def run_task(request: RuntimeTaskRequest) -> RuntimeTaskResponse:
    warnings = scan_prompt_injection(request.user_input)
    provider = resolve_provider(request.provider)
    messages = build_messages(request)

    trace = {
        "provider_type": provider.provider_type,
        "model": provider.model,
        "context_count": len(request.context_items),
        "prompt_injection_warnings": warnings,
        "dry_run": request.dry_run,
    }

    if request.dry_run:
        return RuntimeTaskResponse(
            task_type=request.task_type,
            ok=True,
            output={"dry_run": True, "messages": [message.model_dump() for message in messages]},
            warnings=warnings,
            trace=trace,
        )

    raw = await chat_completion(provider, messages)
    output = parse_json_object(raw)
    return RuntimeTaskResponse(
        task_type=request.task_type,
        ok=True,
        output=output,
        raw_output=raw,
        warnings=warnings,
        trace=trace,
    )


def build_messages(request: RuntimeTaskRequest) -> list[ChatMessage]:
    context = "\n\n".join(
        f"[{item.source_type}:{item.source_id} score={item.score}]\n{item.content}"
        for item in request.context_items
    )
    schema_hint = f"\n\n目标 schema:\n{request.output_schema}" if request.output_schema else ""
    system = (
        "你是个人 AI 面试训练平台的 Python AI Runtime。"
        "所有输出必须结构化、可解释、可追踪。"
        "\n\n"
        + ANTI_INJECTION_INSTRUCTION
        + "\n\n"
        + STRICT_JSON_INSTRUCTION
    )
    user = (
        TASK_INSTRUCTIONS[request.task_type]
        + schema_hint
        + "\n\n上下文：\n"
        + wrap_user_data(context)
        + "\n\n用户输入：\n"
        + wrap_user_data(request.user_input)
    )
    return [
        ChatMessage(role="system", content=system),
        ChatMessage(role="user", content=user),
    ]
