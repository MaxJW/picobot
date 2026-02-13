#!/bin/bash
set -e

PICOBOT_HOME="${PICOBOT_HOME:-/home/picobot/.picobot}"

# Ensure data directory exists and is writable by picobot (fixes bind-mount permissions)
mkdir -p "${PICOBOT_HOME}"
chown -R picobot:picobot "${PICOBOT_HOME}"

# Helper to run commands as picobot user
run_as_picobot() { su picobot -s /bin/sh -c "exec $*"; }

# Auto-onboard if config doesn't exist yet
if [ ! -f "${PICOBOT_HOME}/config.json" ]; then
  echo "First run detected — running onboard..."
  if ! run_as_picobot 'picobot onboard'; then
    echo "❌ Onboard failed. Check that ${PICOBOT_HOME} is writable."
    exit 1
  fi
  echo "✅ Onboard complete. Config at ${PICOBOT_HOME}/config.json"
  echo ""
  echo "⚠️  You need to configure your API key and model."
  echo "   Mount a config file or set environment variables."
  echo ""
fi

# Allow overriding config values via environment variables
# Helper to apply config change (reads from stdin) and ensure picobot can read it
apply_config() {
  TMP=$(mktemp "${PICOBOT_HOME}/.config.XXXXXX")
  cat > "$TMP" && mv "$TMP" "${PICOBOT_HOME}/config.json" && chown picobot:picobot "${PICOBOT_HOME}/config.json"
}

if [ -n "${OPENAI_API_KEY}" ]; then
  echo "Applying OPENAI_API_KEY from environment..."
  cat "${PICOBOT_HOME}/config.json" | \
    sed "s|sk-or-v1-REPLACE_ME|${OPENAI_API_KEY}|g" | apply_config
fi

if [ -n "${OPENAI_API_BASE}" ]; then
  echo "Applying OPENAI_API_BASE from environment..."
  cat "${PICOBOT_HOME}/config.json" | \
    sed "s|https://openrouter.ai/api/v1|${OPENAI_API_BASE}|g" | apply_config
fi

if [ -n "${TELEGRAM_BOT_TOKEN}" ]; then
  echo "Applying TELEGRAM_BOT_TOKEN from environment..."
  cat "${PICOBOT_HOME}/config.json" | \
    sed 's|"enabled": false|"enabled": true|g' | \
    sed "s|\"token\": \"\"|\"token\": \"${TELEGRAM_BOT_TOKEN}\"|g" | apply_config
fi

if [ -n "${TELEGRAM_ALLOW_FROM}" ]; then
  echo "Applying TELEGRAM_ALLOW_FROM from environment..."
  ALLOW_JSON=$(echo "${TELEGRAM_ALLOW_FROM}" | sed 's/,/","/g' | sed 's/^/["/' | sed 's/$/"]/')
  cat "${PICOBOT_HOME}/config.json" | \
    sed "s/\"allowFrom\": null/\"allowFrom\": ${ALLOW_JSON}/g" | \
    sed "s/\"allowFrom\": \[\]/\"allowFrom\": ${ALLOW_JSON}/g" | apply_config
fi

if [ -n "${PICOBOT_MODEL}" ]; then
  echo "Applying PICOBOT_MODEL from environment..."
  cat "${PICOBOT_HOME}/config.json" | \
    sed "s|\"model\": \"stub-model\"|\"model\": \"${PICOBOT_MODEL}\"|g" | apply_config
fi

echo "Starting picobot $*..."
exec su picobot -s /bin/sh -c 'exec picobot "$@"' sh "$@"
