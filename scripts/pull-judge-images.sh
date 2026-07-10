#!/usr/bin/env sh
set -eu

load_env_file() {
  file="$1"
  if [ ! -f "$file" ]; then
    return
  fi
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in
      ""|\#*) continue ;;
    esac
    key=${line%%=*}
    value=${line#*=}
    current=$(eval "printf '%s' \"\${$key:-}\"")
    if [ -n "$key" ] && [ -z "$current" ]; then
      export "$key=$(printf '%s' "$value" | sed "s/^['\"]//;s/['\"]$//")"
    fi
  done < "$file"
}

load_env_file ".env"

docker_bin=${CODING_JUDGE_DOCKER_BINARY:-docker}
images="
${CODING_JUDGE_GO_IMAGE:-golang:1.26-alpine}
${CODING_JUDGE_JAVA_IMAGE:-eclipse-temurin:21-jdk-alpine}
${CODING_JUDGE_PYTHON_IMAGE:-python:3.13-alpine}
${CODING_JUDGE_JAVASCRIPT_IMAGE:-node:22-alpine}
${CODING_JUDGE_TYPESCRIPT_IMAGE:-denoland/deno:alpine-2.1.4}
${CODING_JUDGE_CPP_IMAGE:-gcc:14-alpine}
"

for image in $images; do
  echo "Pulling judge image: $image"
  "$docker_bin" pull "$image"
done
