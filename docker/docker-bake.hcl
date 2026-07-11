# Composition graph for the interview stack images.
# Each vscode stack is one target naming its parent via `contexts`.
# Canonical layer order: backend, devops, ai.

variable "TAG" { default = "local" }

IMAGE = "interview-labs-vscode"

# Everything the vscode build/test cycle needs, base included.
group "local" {
  targets = ["base", "backend", "devops", "backend-ai", "devops-ai"]
}

target "gateway" {
  context = "images/gateway"
  tags    = ["interview-labs-gateway:${TAG}"]
}

target "base" {
  context = "images/vscode/layers/base"
  tags    = ["${IMAGE}:base-${TAG}"]
}

target "backend" {
  context  = "images/vscode/layers/backend"
  contexts = { parent = "target:base" }
  tags     = ["${IMAGE}:backend-${TAG}"]
}

target "devops" {
  context  = "images/vscode/layers/devops"
  contexts = { parent = "target:base" }
  tags     = ["${IMAGE}:devops-${TAG}"]
}

target "backend-ai" {
  context  = "images/vscode/layers/ai"
  contexts = { parent = "target:backend" }
  tags     = ["${IMAGE}:backend-ai-${TAG}"]
}

target "devops-ai" {
  context  = "images/vscode/layers/ai"
  contexts = { parent = "target:devops" }
  tags     = ["${IMAGE}:devops-ai-${TAG}"]
}
