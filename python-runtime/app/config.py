from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    app_env: str = "local"
    runtime_host: str = "0.0.0.0"
    runtime_port: int = 8090
    deepseek_base_url: str = "https://api.deepseek.com"
    deepseek_chat_endpoint_path: str = "/chat/completions"
    deepseek_chat_model: str = "deepseek-v4-flash"
    deepseek_api_key: str = ""
    openai_compatible_base_url: str = "https://api.openai.com/v1"
    openai_compatible_chat_endpoint_path: str = "/chat/completions"
    openai_compatible_chat_model: str = ""
    openai_compatible_api_key: str = ""
    llm_timeout_seconds: float = 30.0
    memory_db_path: str = "data/runtime_memory.sqlite3"


settings = Settings()
