#!/bin/bash

OLD_MODULE="github.com/eli-yip/zsxq-parser"
NEW_MODULE="github.com/eli-yip/rss-zero"

# 检测操作系统并设置相应的 sed 命令
if [[ "$OSTYPE" == "darwin"* ]]; then
  if ! command -v gsed &>/dev/null; then
    echo "GNU sed (gsed) 未安装。正在尝试通过 Homebrew 安装..."
    brew install gnu-sed
  fi
  SED_COMMAND="gsed -i"
else
  SED_COMMAND="sed -i"
fi

$SED_COMMAND "s|module $OLD_MODULE|module $NEW_MODULE|g" go.mod

# 使用 fd 查找所有 Go 文件并更新模块引用
fd '\.go$' --exec $SED_COMMAND "s|$OLD_MODULE|$NEW_MODULE|g" {} \;

go mod tidy
