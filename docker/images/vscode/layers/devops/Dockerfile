# Devops profile — devops engineer toolchain on top of the parent image.
# `parent` is supplied by bake (contexts = { parent = "target:base" }).
# trivy:ignore:DS-0001 — parent is a bake named context, not a registry ref; no tag applies
FROM parent AS devops
USER root
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

COPY mise.toml /etc/mise/config.toml
RUN --mount=type=secret,id=github_token,env=GITHUB_TOKEN mise install

COPY setup.sh /tmp/profile.sh
COPY settings.json /tmp/profile.settings.json
RUN bash /tmp/profile.sh && rm -f /tmp/profile.sh /tmp/profile.settings.json

USER user
RUN cs-extension-install
