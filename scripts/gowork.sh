#!/usr/bin/env bash
# 开发期「三仓联动」：在指定目录生成/更新 go.work，让两个下游仓库直接用本仓的当前代码，
# 而不是去代理拉某个已发布版本（P2-2 / P2-28）。
#
#   bash scripts/gowork.sh                 # 默认在上一级目录（同时放着三个仓库时）
#   bash scripts/gowork.sh /path/to/dir    # 指定 go.work 落点
#   bash scripts/gowork.sh --off           # 删除该目录的 go.work
#
# 为什么放在仓库外：go.work 是**机器本地**文件，不该进任何一个仓库；
# 三个仓库各自被单独 clone 时也不该看见它。发布前置校验由
# 两个下游仓库的 scripts/share-version.sh 负责（版本号必须与最新 tag 一致）。
set -euo pipefail

SHARE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PARENT="$(dirname "$SHARE_DIR")"
TARGET="${1:-$PARENT}"

if [[ "${1:-}" == "--off" ]]; then
    TARGET="${2:-$PARENT}"
    rm -f "$TARGET/go.work" "$TARGET/go.work.sum"
    echo "已删除 $TARGET/go.work"
    exit 0
fi

SERVER="$PARENT/api-doc-go/server"
CLIENT="$PARENT/api-doc-go-client"
if [[ ! -d "$SERVER" || ! -d "$CLIENT" ]]; then
    echo "找不到下游仓库：期望 $SERVER 与 $CLIENT 同时存在" >&2
    exit 1
fi

# go.work 的 go 版本必须 >= 各模块 go.mod 的声明（否则 workspace 模式直接报错），
# 取服务端声明值（当前是最高的那个）。
GOVER="$(grep -m1 '^go ' "$SERVER/go.mod" | awk '{print $2}')"

cat > "$TARGET/go.work" <<WORK
// 自动生成（api-doc-go-share/scripts/gowork.sh），勿手工编辑、勿提交到任何仓库。
// 作用：三个模块用本地代码互相引用，改 share 后两个下游立即可见，无需发版。
go $GOVER

use (
	$SHARE_DIR
	$SERVER
	$CLIENT
)
WORK
echo "已写入 $TARGET/go.work"
echo "  验证：cd $SERVER && go build ./..."
echo "  取消：bash $SHARE_DIR/scripts/gowork.sh --off"
