from typing import Any

import httpx

from app.config import settings
from app.models import ChatMessage, ProviderConfig


def resolve_provider(config: ProviderConfig | None) -> ProviderConfig:
    if config and config.api_key and config.model and config.base_url:
        return config
    provider_type = config.provider_type if config else "deepseek"
    if provider_type == "openai_compatible":
        return ProviderConfig(
            provider_type="openai_compatible",
            base_url=settings.openai_compatible_base_url,
            chat_endpoint_path=settings.openai_compatible_chat_endpoint_path,
            model=settings.openai_compatible_chat_model,
            api_key=settings.openai_compatible_api_key,
            supports_json=True,
        )
    return ProviderConfig(
        provider_type="deepseek",
        base_url=settings.deepseek_base_url,
        chat_endpoint_path=settings.deepseek_chat_endpoint_path,
        model=settings.deepseek_chat_model,
        api_key=settings.deepseek_api_key,
        supports_json=True,
    )


async def chat_completion(provider: ProviderConfig, messages: list[ChatMessage]) -> str:
    if not provider.api_key:
        raise ValueError(f"{provider.provider_type} api key is not configured")
    if not provider.model:
        raise ValueError(f"{provider.provider_type} model is not configured")

    endpoint = provider.base_url.rstrip("/") + normalize_path(provider.chat_endpoint_path)
    payload: dict[str, Any] = {
        "model": provider.model,
        "messages": [message.model_dump() for message in messages],
        "stream": False,
        "temperature": 0.2,
    }
    if provider.supports_json:
        payload["response_format"] = {"type": "json_object"}

    async with httpx.AsyncClient(timeout=settings.llm_timeout_seconds) as client:
        response = await client.post(
            endpoint,
            headers={
                "Authorization": f"Bearer {provider.api_key}",
                "Content-Type": "application/json",
            },
            json=payload,
        )
    response.raise_for_status()
    data = response.json()
    return data["choices"][0]["message"]["content"]


def normalize_path(path: str) -> str:
    path = (path or "/chat/completions").strip()
    if not path.startswith("/"):
        path = "/" + path
    return path
