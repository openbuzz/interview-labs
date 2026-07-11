FROM golang:1.26-trixie

# Install bats, shellcheck, tmux: drive and assert the TUI under a real pty.
RUN apt-get update && apt-get install -y --no-install-recommends \
    bats \
    shellcheck \
    tmux \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Warm the module cache before copying the tree for better layer reuse.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /usr/local/bin/interview ./cmd/interview

# Fixed terminal identity so captured frames are reproducible everywhere.
ENV TERM=xterm-256color \
    LANG=C.UTF-8

ENTRYPOINT ["bats"]
CMD ["tests/tui"]
