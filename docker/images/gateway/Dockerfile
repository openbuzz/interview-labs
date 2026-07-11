# Auth gateway proxy for interview-labs.
ARG GO_VERSION=1.26
FROM golang:${GO_VERSION} AS build

WORKDIR /src
COPY go.mod ./
COPY *.go *.html *.css ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /gateway .
# Pre-create the secret dir owned by the runtime uid — scratch has no shell to
# mkdir/chown, so the anonymous volume would otherwise mount root-owned.
RUN install -d -o 65532 -g 65532 /runtime

FROM scratch
COPY --from=build /gateway /gateway
COPY --from=build --chown=65532:65532 /runtime /runtime
USER 65532:65532
# Persist the auto-generated cookie key so sessions survive a container restart.
# GATEWAY_SECRET_FILE is a path to that runtime-generated file, not a baked secret.
# trivy:ignore:AVD-DS-0031
ENV GATEWAY_SECRET_FILE=/runtime/secret
VOLUME /runtime
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/gateway", "-healthcheck"]
ENTRYPOINT ["/gateway"]
