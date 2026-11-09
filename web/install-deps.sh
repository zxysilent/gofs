#!/usr/bin/env bash
# gofs 前端依赖安装与构建脚本
# 用法: bash install-deps.sh [--build-only]
set -euo pipefail

cd "$(dirname "$0")"

echo "==> node 版本: $(node -v)"
echo "==> npm  版本: $(npm -v)"

if [[ "${1:-}" != "--build-only" ]]; then
  echo "==> 安装依赖 (npm install)..."
  npm install
fi

echo "==> 构建前端 (npm run build)..."
npm run build

echo "==> 构建产物:"
ls -la dist/

echo "==> 完成"
