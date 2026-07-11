# Browser-based VS Code image for interview-labs.

FROM ubuntu:26.04 AS base

LABEL org.opencontainers.image.source="https://github.com/openbuzz/interview-labs-images"
LABEL org.opencontainers.image.licenses="Apache-2.0"

ARG TARGETARCH
ARG DEBIAN_FRONTEND=noninteractive

# Configure the shell with bash pipefail so mid-pipe failures abort the build.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Install base system packages.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash-completion ca-certificates curl dumb-init git gnupg htop jq less locales mc \
      nano sudo tree unzip vim wget && \
    rm -rf /var/lib/apt/lists/*

# Configure locale and timezone.
RUN locale-gen en_US.UTF-8
ENV LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 TZ=UTC

# Suppress telemetry, update-notifiers, and non-essential network chatter.
# DO_NOT_TRACK is the community-standard opt-out honored by many CLIs.
ENV DO_NOT_TRACK=1
ENV NPM_CONFIG_UPDATE_NOTIFIER=false
ENV NPM_CONFIG_FUND=false
ENV NPM_CONFIG_AUDIT=false

# Install code-server: VS Code in the browser.
# Available releases: https://github.com/coder/code-server/releases
ARG CODE_SERVER_VERSION=4.125.0
ARG CODE_SERVER_SHA256_AMD64=3389e1effad1e20d75cbacd1af48b9a85b5eb131a25110a0053b81e4cf6014e7
ARG CODE_SERVER_SHA256_ARM64=fef237ce4ba6b3258b981c2c2b7b93a67c08e1a6471e10eda66037a456f262ec
RUN case "${TARGETARCH}" in \
      amd64) sha="${CODE_SERVER_SHA256_AMD64}" ;; \
      arm64) sha="${CODE_SERVER_SHA256_ARM64}" ;; \
      *) echo "unsupported architecture: ${TARGETARCH}" >&2 ; exit 1 ;; \
    esac && \
    asset="code-server_${CODE_SERVER_VERSION}_${TARGETARCH}.deb" && \
    curl -fsSL -o /tmp/code-server.deb \
      "https://github.com/coder/code-server/releases/download/v${CODE_SERVER_VERSION}/${asset}" && \
    printf '%s  /tmp/code-server.deb\n' "${sha}" | sha256sum -c - && \
    apt-get update && \
    apt-get install -y --no-install-recommends /tmp/code-server.deb && \
    rm -f /tmp/code-server.deb && \
    rm -rf /var/lib/apt/lists/*

# Install ttyd: browser terminal; fallback when the code-server terminal lags.
# Available releases: https://github.com/tsl0922/ttyd/releases
ARG TTYD_VERSION=1.7.7
ARG TTYD_SHA256_AMD64=8a217c968aba172e0dbf3f34447218dc015bc4d5e59bf51db2f2cd12b7be4f55
ARG TTYD_SHA256_ARM64=b38acadd89d1d396a0f5649aa52c539edbad07f4bc7348b27b4f4b7219dd4165
RUN case "${TARGETARCH}" in \
      amd64) arch=x86_64;  sha="${TTYD_SHA256_AMD64}" ;; \
      arm64) arch=aarch64; sha="${TTYD_SHA256_ARM64}" ;; \
      *) echo "unsupported architecture: ${TARGETARCH}" >&2 ; exit 1 ;; \
    esac && \
    curl -fsSL -o /tmp/ttyd \
      "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.${arch}" && \
    printf '%s  /tmp/ttyd\n' "${sha}" | sha256sum -c - && \
    install -m 0755 /tmp/ttyd /usr/local/bin/ttyd && \
    rm -f /tmp/ttyd

# Install uv: Python installer and package manager.
# Available releases: https://github.com/astral-sh/uv/releases
ARG UV_VERSION=0.11.24
ARG UV_SHA256_AMD64=5ce1ad074a78f96c5c8122088bb85a12eb282195bc1453151a48762e4fc31fed
ARG UV_SHA256_ARM64=e22c66d36a0098b17cff80a8647e0b8c58202af899d4e9eb820fc7ad126435a1
RUN case "${TARGETARCH}" in \
      amd64) arch=x86_64;  sha="${UV_SHA256_AMD64}" ;; \
      arm64) arch=aarch64; sha="${UV_SHA256_ARM64}" ;; \
      *) echo "unsupported architecture: ${TARGETARCH}" >&2 ; exit 1 ;; \
    esac && \
    asset="uv-${arch}-unknown-linux-gnu.tar.gz" && \
    curl -fsSL -o /tmp/uv.tar.gz \
      "https://github.com/astral-sh/uv/releases/download/${UV_VERSION}/${asset}" && \
    printf '%s  /tmp/uv.tar.gz\n' "${sha}" | sha256sum -c - && \
    mkdir -p /tmp/uv && \
    tar -xzf /tmp/uv.tar.gz --strip-components=1 -C /tmp/uv && \
    install -m 0755 /tmp/uv/uv  /usr/local/bin/uv && \
    install -m 0755 /tmp/uv/uvx /usr/local/bin/uvx && \
    rm -rf /tmp/uv /tmp/uv.tar.gz

# Install CPython: pinned interpreter, system-wide on the global PATH.
# Available releases: https://github.com/astral-sh/python-build-standalone/releases
ARG PYTHON_VERSION=3.14.6
RUN export UV_PYTHON_INSTALL_DIR=/opt/python && \
    uv python install "${PYTHON_VERSION}" && \
    py="$(uv python find "${PYTHON_VERSION}")" && \
    ln -sf "${py}" /usr/local/bin/python3 && \
    ln -sf python3 /usr/local/bin/python && \
    uv generate-shell-completion bash > /etc/bash_completion.d/uv

# Install Node.js: JavaScript runtime (LTS).
# Available releases: https://nodejs.org/en/about/previous-releases
ARG NODE_VERSION=24.18.0
ARG NODE_SHA256_AMD64=783130984963db7ba9cbd01089eaf2c2efb055c7c1693c943174b967b3050cb8
ARG NODE_SHA256_ARM64=6b4484c2190274175df9aa8f28e2d758a819cb1c1fe6ab481e2f95b463ab8508
RUN case "${TARGETARCH}" in \
      amd64) arch=x64;   sha="${NODE_SHA256_AMD64}" ;; \
      arm64) arch=arm64; sha="${NODE_SHA256_ARM64}" ;; \
      *) echo "unsupported architecture: ${TARGETARCH}" >&2 ; exit 1 ;; \
    esac && \
    asset="node-v${NODE_VERSION}-linux-${arch}.tar.gz" && \
    mkdir -p /opt/nodejs && \
    curl -fsSL -o /tmp/node.tar.gz \
      "https://nodejs.org/dist/v${NODE_VERSION}/${asset}" && \
    printf '%s  /tmp/node.tar.gz\n' "${sha}" | sha256sum -c - && \
    tar -xzf /tmp/node.tar.gz --strip-components=1 -C /opt/nodejs && \
    ln -sf /opt/nodejs/bin/node /usr/local/bin/node && \
    ln -sf /opt/nodejs/bin/npm /usr/local/bin/npm && \
    ln -sf /opt/nodejs/bin/npx /usr/local/bin/npx && \
    /opt/nodejs/bin/npm completion > /etc/bash_completion.d/npm && \
    rm -f /tmp/node.tar.gz
ENV PATH=${PATH}:/opt/nodejs/bin

# Install Docker CLI with plugins.
# The daemon runs as a separate Docker-in-Docker container.
ARG DOCKER_GPG_FINGERPRINT=9DC858229FC7DD38854AE2D88D81803C0EBFCD88
RUN install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc && \
    chmod a+r /etc/apt/keyrings/docker.asc && \
    fpr="$(gpg --show-keys --with-colons /etc/apt/keyrings/docker.asc \
            | awk -F: '/^fpr:/{print $10; exit}')" && \
    [[ "${fpr}" == "${DOCKER_GPG_FINGERPRINT}" ]] || \
      { echo "docker gpg fingerprint mismatch: ${fpr}" >&2 ; exit 1 ; } && \
    codename="$(source /etc/os-release && echo "${VERSION_CODENAME}")" && \
    echo "deb [arch=${TARGETARCH} signed-by=/etc/apt/keyrings/docker.asc] \
          https://download.docker.com/linux/ubuntu ${codename} stable" \
      > /etc/apt/sources.list.d/docker.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
      docker-ce-cli docker-buildx-plugin docker-compose-plugin && \
    rm -rf /var/lib/apt/lists/*

# Install mise: runtime tool-version manager.
# Available releases: https://github.com/jdx/mise/releases
ARG MISE_VERSION=2026.6.14
ARG MISE_SHA256_AMD64=c5bb4546ba2d5154e9c8236e2774bd8289b64c409330ed41cb6d6b8ebc31fb56
ARG MISE_SHA256_ARM64=6d71ba01f20bb7cc7bfffac5214e9e788a08fa517c075eb955a5b31dca667ca7
RUN case "${TARGETARCH}" in \
      amd64) arch=x64;   sha="${MISE_SHA256_AMD64}" ;; \
      arm64) arch=arm64; sha="${MISE_SHA256_ARM64}" ;; \
      *) echo "unsupported architecture: ${TARGETARCH}" >&2 ; exit 1 ;; \
    esac && \
    asset="mise-v${MISE_VERSION}-linux-${arch}.tar.gz" && \
    curl -fsSL -o /tmp/mise.tar.gz \
      "https://github.com/jdx/mise/releases/download/v${MISE_VERSION}/${asset}" && \
    printf '%s  /tmp/mise.tar.gz\n' "${sha}" | sha256sum -c - && \
    mkdir -p /tmp/mise && \
    tar -xzf /tmp/mise.tar.gz --strip-components=2 -C /tmp/mise && \
    install -m 0755 /tmp/mise/mise /usr/local/bin/mise && \
    rm -rf /tmp/mise /tmp/mise.tar.gz
ENV MISE_DATA_DIR=/opt/mise
ENV PATH=/opt/mise/shims:${PATH}

# Configure global mise shell completion.
RUN mise completion bash > /etc/bash_completion.d/mise

# Create the non-root user, replacing Ubuntu's default user.
RUN userdel -r ubuntu && \
    groupadd -g 1000 user && \
    useradd -m -u 1000 -g 1000 -s /bin/bash user && \
    echo "user ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/nopasswd && \
    chmod 0440 /etc/sudoers.d/nopasswd && \
    visudo -c -f /etc/sudoers.d/nopasswd
ENV HOME=/home/user

# Copy code-server settings and shell config.
COPY --chown=user:user files/user/settings.json \
      /home/user/.local/share/code-server/User/settings.json
COPY --chown=user:user files/user/bashrc.local /home/user/.bashrc.local

# Enable the shell config and create the default workspace.
RUN echo 'source ~/.bashrc.local' >> /home/user/.bashrc && \
    mkdir -p /home/user/workspace

# Install the entrypoint.
COPY --chmod=755 files/entrypoint.sh /usr/local/bin/entrypoint.sh

# Copy the code-server helpers and register the base extensions.
COPY --chmod=755 files/bin/ /usr/local/bin/

# Shared build-time scratch for the extension manifest. Owner `user` so the
# unprivileged install step can create and remove it; setgid keeps group root.
RUN install -d -o user -g root -m 2775 /var/run/code-server
# The dir-create above is a distinct setup step from extension registration, so
# it stays its own RUN rather than folding into the heredoc below.
# hadolint ignore=DL3059
RUN <<'EOF'
extensions=(
  pkief.material-icon-theme        # Material Icon Theme — file/folder icons
  redhat.vscode-yaml               # YAML (Red Hat) — schema validation & completion
  ms-python.python                 # Python (Microsoft) — IntelliSense & debugging
  charliermarsh.ruff               # Ruff — fast Python linter & formatter
  esbenp.prettier-vscode           # Prettier — code formatter
  ms-azuretools.vscode-containers  # Container Tools — build & manage containers
)
cs-extension-register "${extensions[@]}"
EOF

# Fix home-directory ownership — a distinct concern from extension registration
# above, so it stays its own step rather than folding into that heredoc.
# hadolint ignore=DL3059
RUN chown -R user:user /home/user

# Switch to the non-root user.
USER user
WORKDIR /home/user/workspace
ENV USER=user

# Install the registered code-server extensions as the unprivileged user.
RUN cs-extension-install

EXPOSE 8080 7681

HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["dumb-init", "/usr/local/bin/entrypoint.sh"]
