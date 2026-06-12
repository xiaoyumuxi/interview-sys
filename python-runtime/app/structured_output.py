import json
import re
from typing import Any


STRICT_JSON_INSTRUCTION = """
请仅返回可被 JSON 解析器直接解析的 JSON 对象：
1. 不要输出 Markdown 代码块。
2. 不要输出解释文字、前后缀或注释。
3. 字符串内引号必须正确转义。
"""

_CODE_FENCE_RE = re.compile(r"^```(?:json)?\s*|\s*```$", re.IGNORECASE | re.MULTILINE)


def parse_json_object(content: str) -> dict[str, Any]:
    cleaned = _CODE_FENCE_RE.sub("", content.strip()).strip()
    try:
        data = json.loads(cleaned)
    except json.JSONDecodeError:
        data = json.loads(extract_json_object(cleaned))
    if not isinstance(data, dict):
        raise ValueError("structured output must be a JSON object")
    return data


def extract_json_object(content: str) -> str:
    start = content.find("{")
    end = content.rfind("}")
    if start < 0 or end <= start:
        raise ValueError("no JSON object found")
    return content[start : end + 1]
