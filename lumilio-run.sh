#!/bin/bash

# ==============================================================================
#  macOS 一键安装脚本
#  功能:
#  1. 检查依赖 (git, docker, python3)。
#  2. 从 GitHub 克隆项目。
#  3. 设置在本地运行的 ML 服务 (使用 venv 和 launchd)。
#  4. 使用 Docker Compose 启动其余服务。
# ==============================================================================

# --- 配置 ---
# !!! 请将此处的 URL 更改为您自己的项目仓库地址 !!!
GITHUB_REPO_URL="https://github.com/your-username/your-project-name.git"
# Docker Compose 文件的名称 (应存在于您的仓库中)
COMPOSE_FILE_NAME="docker-compose-hybrid.yml"
# ML 服务的 launchd 标识符
LAUNCHD_LABEL="com.yourcompany.mlservice"


# --- 脚本设置 ---
# 如果任何命令失败，则立即退出
set -e

# --- 日志函数 ---
# 定义颜色代码以便更清晰地输出日志
COLOR_RESET='\033[0m'
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[0;33m'
COLOR_RED='\033[0;31m'

info() {
    echo -e "${COLOR_YELLOW}[INFO] $1${COLOR_RESET}"
}

success() {
    echo -e "${COLOR_GREEN}[SUCCESS] $1${COLOR_RESET}"
}

error() {
    echo -e "${COLOR_RED}[ERROR] $1${COLOR_RESET}" >&2
    exit 1
}

# --- 1. 检查依赖 ---
info "开始检查系统依赖 (Git, Docker, Python 3)..."
check_command() {
    if ! command -v "$1" &> /dev/null; then
        error "必需的命令 '$1' 未找到。请先安装它。"
    fi
}
check_command "git"
check_command "docker"
check_command "python3"
success "所有依赖都已安装。"


# --- 2. 克隆项目仓库 ---
PROJECT_DIR_NAME=$(basename "$GITHUB_REPO_URL" .git)

if [ -d "$PROJECT_DIR_NAME" ]; then
    info "项目目录 '$PROJECT_DIR_NAME' 已存在，跳过克隆。"
else
    info "正在从 GitHub 克隆项目..."
    git clone "$GITHUB_REPO_URL"
    success "项目已成功克隆到 '$PROJECT_DIR_NAME' 目录。"
fi

cd "$PROJECT_DIR_NAME"
PROJECT_ABS_PATH=$(pwd)


# --- 3. 设置本地 ML 服务 ---
info "正在为本地 ML 服务设置 Python 环境..."

if [ ! -d "venv" ]; then
    python3 -m venv venv
    info "Python 虚拟环境已创建。"
fi

# 使用 venv 中的 pip 安装依赖
./venv/bin/pip install -r requirements.txt # 假设依赖文件名为 requirements.txt
success "ML 服务的 Python 依赖已安装。"


# --- 4. 配置并启动 ML 服务的 launchd ---
info "正在配置 ML 服务以作为 macOS 后台服务运行..."
PLIST_PATH="$HOME/Library/LaunchAgents/$LAUNCHD_LABEL.plist"

# 动态生成 plist 文件内容，以确保路径正确
# 注意: __PLACEHOLDER__ 将被实际路径替换
PLIST_TEMPLATE=$(cat <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${LAUNCHD_LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>__PYTHON_PATH__</string>
        <string>__SCRIPT_PATH__</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>WorkingDirectory</key>
    <string>__WORKING_DIR__</string>
    <key>StandardOutPath</key>
    <string>__WORKING_DIR__/logs/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>__WORKING_DIR__/logs/stderr.log</string>
</dict>
</plist>
EOF
)

# 创建日志目录
mkdir -p "${PROJECT_ABS_PATH}/logs"

# 替换占位符
PYTHON_EXEC_PATH="${PROJECT_ABS_PATH}/venv/bin/python"
# 假设您的 ML 服务启动脚本是 server.py 且在 pml/ 目录下
ML_SCRIPT_PATH="${PROJECT_ABS_PATH}/pml/server.py"

FINAL_PLIST="${PLIST_TEMPLATE//__PYTHON_PATH__/$PYTHON_EXEC_PATH}"
FINAL_PLIST="${FINAL_PLIST//__SCRIPT_PATH__/$ML_SCRIPT_PATH}"
FINAL_PLIST="${FINAL_PLIST//__WORKING_DIR__/$PROJECT_ABS_PATH}"

# 写入 plist 文件
echo "$FINAL_PLIST" > "$PLIST_PATH"
info "launchd 配置文件已创建于: $PLIST_PATH"

# 加载或重新加载 launchd 服务
# `|| true` 用于忽略在服务未加载时 unload 命令的错误
info "正在加载并启动 ML 后台服务..."
launchctl unload "$PLIST_PATH" 2>/dev/null || true
launchctl load "$PLIST_PATH"
launchctl start "$LAUNCHD_LABEL"
success "ML 服务已作为后台服务启动。日志文件位于 '$PROJECT_ABS_PATH/logs/'。"


# --- 5. 启动 Docker 服务 ---
info "正在使用 Docker Compose 拉取并启动其余服务..."

# 确保 Docker Desktop 正在运行
if ! docker info &> /dev/null; then
    error "Docker 守护进程未运行。请启动 Docker Desktop。"
fi

# 使用指定的 compose 文件
docker-compose -f "$COMPOSE_FILE_NAME" pull
docker-compose -f "$COMPOSE_FILE_NAME" up -d
success "所有 Docker 服务已成功启动！"


# --- 结束 ---
echo ""
echo -e "${COLOR_GREEN}=====================================================${COLOR_RESET}"
echo -e "${COLOR_GREEN}🎉 全部设置完成！                                 ${COLOR_RESET}"
echo -e "${COLOR_GREEN}=====================================================${COLOR_RESET}"
echo ""
echo -e "  - 您的 Web 应用现在应该运行在: ${COLOR_YELLOW}http://localhost:3000${COLOR_RESET}"
echo -e "  - 本地 ML 服务的日志可以在这里找到: ${COLOR_YELLOW}${PROJECT_ABS_PATH}/logs/${COLOR_RESET}"
echo -e "  - 要停止所有 Docker 服务, 请运行: ${COLOR_YELLOW}docker-compose -f ${COMPOSE_FILE_NAME} down${COLOR_RESET}"
echo -e "  - 要停止后台的 ML 服务, 请运行: ${COLOR_YELLOW}launchctl unload ${PLIST_PATH}${COLOR_RESET}"
echo ""
