# syntax=docker/dockerfile:1
#
# Dev container image for Lumilio Photos.
# Includes Go, Node.js, Rust (WASM), and all system-level dependencies
# needed for local development.
#
# NOTE: The db service and other infrastructure containers are defined
# in .devcontainer/docker-compose.yml and run alongside this container.

FROM golang:1.25.0-trixie

ENV LANG=C.UTF-8 \
    LC_ALL=C.UTF-8

# ---------------------------------------------------------------
# Base system dependencies
# ---------------------------------------------------------------
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libvips-dev \
    libvips-tools \
    libraw-dev \
    ffmpeg \
    exiftool \
    ca-certificates \
    git \
    unzip \
    curl \
    zsh \
    gnupg \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# ---------------------------------------------------------------
# Better terminal: zsh + Starship
# ---------------------------------------------------------------
RUN curl -sS https://starship.rs/install.sh | sh -s -- -y

RUN mkdir -p /root/.config && cat > /root/.config/starship.toml <<'EOF'
add_newline = false
command_timeout = 1000

format = """
$username\
$hostname\
$directory\
$git_branch\
$git_status\
$nodejs\
$golang\
$rust\
$docker_context\
$cmd_duration\
$line_break\
$character"""

[username]
show_always = false

[hostname]
ssh_only = true

[directory]
truncation_length = 3
truncate_to_repo = true

[git_branch]
symbol = "git: "
format = "[$symbol$branch]($style) "

[git_status]
format = "([$all_status$ahead_behind]($style) )"

[nodejs]
symbol = "node: "
format = "[$symbol($version)]($style) "

[golang]
symbol = "go: "
format = "[$symbol($version)]($style) "

[rust]
symbol = "rs: "
format = "[$symbol($version)]($style) "

[docker_context]
symbol = "docker: "
format = "[$symbol$context]($style) "

[cmd_duration]
min_time = 1000
format = "took [$duration]($style) "

[character]
success_symbol = "[❯](bold green)"
error_symbol = "[❯](bold red)"
EOF

RUN cat > /root/.zshrc <<'EOF'
# zsh configuration for the dev container.

export PATH="/root/.vite-plus/bin:/root/go/bin:/usr/local/cargo/bin:/usr/local/go/bin:$PATH"

# Enable Starship prompt.
eval "$(starship init zsh)"

# History
HISTFILE=/root/.zsh_history
HISTSIZE=10000
SAVEHIST=10000
setopt HIST_IGNORE_DUPS
setopt HIST_IGNORE_SPACE
setopt SHARE_HISTORY

# Quality-of-life options
setopt AUTO_CD
setopt CORRECT
setopt PROMPT_SUBST

# Completion
autoload -Uz compinit
compinit

# Aliases
alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CF'

alias gs='git status'
alias gd='git diff'
alias gl='git log --oneline --graph --decorate'
alias gco='git checkout'
alias gb='git branch'

alias vpw='cd /workspace/web && vp'
alias vpi='cd /workspace/web && vp install'
alias vpd='cd /workspace/web && vp dev --host --port 6657'
alias vpb='cd /workspace/web && vp build'
alias vpt='cd /workspace/web && vp test'
alias vpc='cd /workspace/web && vp check'

alias dc='docker compose'
alias dps='docker ps'
EOF

RUN chsh -s /usr/bin/zsh root

# ---------------------------------------------------------------
# Node.js 23.x + Vite+ CLI (matches web/.nvmrc)
# ---------------------------------------------------------------
ARG NODE_VERSION=23.11.1

RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
      amd64) node_arch="x64" ;; \
      arm64) node_arch="arm64" ;; \
      *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSLO "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node_arch}.tar.xz"; \
    tar -xJf "node-v${NODE_VERSION}-linux-${node_arch}.tar.xz" -C /usr/local --strip-components=1; \
    rm "node-v${NODE_VERSION}-linux-${node_arch}.tar.xz"; \
    corepack enable; \
    corepack prepare pnpm@latest --activate; \
    curl -fsSL https://vite.plus | bash; \
    node --version; \
    npm --version; \
    pnpm --version; \
    /root/.vite-plus/bin/vp --version

# ---------------------------------------------------------------
# Rust toolchain (nightly + wasm target for WASM crates)
# Use the wasm-pack binary installer instead of `cargo install`
# to avoid compiling it from source (~700MB saved).
# ---------------------------------------------------------------
ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/root/.vite-plus/bin:/usr/local/cargo/bin:$PATH

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y \
      --default-toolchain nightly-2025-11-15 \
      --profile minimal \
    && rustup component add rust-analyzer \
    && rustup target add wasm32-unknown-unknown \
    && curl -fsSL https://rustwasm.github.io/wasm-pack/installer/init.sh | sh \
    && rm -rf /usr/local/cargo/registry /usr/local/cargo/git

# Go CLI tools are installed in postCreateCommand via `make setup` so
# they land on the mounted /root/go volume instead of
# bloating the image layer.

# ---------------------------------------------------------------
# Docker CLI (host socket is mounted at runtime)
# ---------------------------------------------------------------
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian trixie stable" > /etc/apt/sources.list.d/docker.list && \
    apt-get update && apt-get install -y --no-install-recommends \
        docker-ce-cli \
        docker-compose-plugin && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /workspace
