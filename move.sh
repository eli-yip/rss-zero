#!/bin/bash

# 接收第一个参数作为name
name=$1

# 使用fd查找所有文件，并用sed进行替换
fd . --type f -x sed -i "s|pkg/$name|internal/$name|g"

