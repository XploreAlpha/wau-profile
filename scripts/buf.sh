#!/bin/bash
# buf generate 一键脚本
# 跟 wau-intent 仓 scripts/buf.sh 完全对齐
set -euo pipefail
cd "$(dirname "$0")/.."
buf generate
echo "✓ buf generate done → profilev1/"
