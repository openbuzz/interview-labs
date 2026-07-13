# Go Web App — Solution

## Objectives

1. Implement a multi-stage build to separate compilation and runtime.
2. Produce a statically linked binary.
3. Use a minimal runtime image such as scratch or distroless.
4. Achieve a final image size under 20 MB.
5. Add a non-root user.
6. Ensure the application still builds and runs correctly.

## Changes

- **Multi-stage build**: Separates the compilation environment (golang:1.26) from the runtime environment. The build stage contains the full Go toolchain needed to compile, while the runtime stage carries only the final binary. This eliminates hundreds of megabytes of unnecessary build tools from the production image.
- **Static linking**: `CGO_ENABLED=0` produces a statically linked binary with no dependency on system C libraries. This is essential for running on distroless or scratch images that lack libc and other shared libraries.
- **Binary stripping**: `-ldflags="-s -w"` removes debug information (`-s`) and symbol tables (`-w`) from the compiled binary. This typically reduces binary size by 20-30% with no impact on runtime behavior.
- **Scratch runtime**: `scratch` is an empty base image — the final image contains nothing but the binary. This is possible because the Go binary is statically linked and has no external dependencies. The result is the smallest possible image with the smallest possible attack surface. An alternative is `gcr.io/distroless/static-debian12`, which adds ca-certificates and tzdata (~2 MB larger) — a better choice when the application makes outbound HTTPS calls or needs timezone support.
- **Module caching**: Copying `go.mod` and running `go mod download` before copying source code means Docker can cache the dependency download layer. Subsequent builds that only change application code skip the dependency download entirely, significantly speeding up rebuilds.
- **Non-root user**: Since `scratch` has no user management tools, a minimal `/etc/passwd` is created in the build stage and copied into the runtime image. The `app` user (UID 1000) runs the process without root privileges.

## Expected result

- Image size: ~5-10 MB (down from ~800+ MB with golang:latest).
- Statically linked binary with no external dependencies.
- Runs as non-root user.
- Minimal attack surface — no shell, no package manager, no OS.
