#!/usr/bin/env bash
set -euo pipefail

IMAGE="eliyip/rss-zero"
PLATFORM="linux/amd64"
TAG=""
PUSH=0

usage() {
  cat <<'EOF'
Usage: scripts/build-docker.sh [--tag TAG] [--image IMAGE] [--platform PLATFORM] [--push]

Build Docker images matching the Gitea GoReleaser workflow:
  eliyip/rss-zero:latest
  eliyip/rss-zero:{TAG}

Options:
  --tag TAG            Image/version tag. Defaults to the exact current git tag.
  --image IMAGE        Image repository. Defaults to eliyip/rss-zero.
  --platform PLATFORM  Docker build platform. Defaults to linux/amd64.
  --push               Push both tags after building.
  -h, --help           Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag)
      TAG="${2:?missing value for --tag}"
      shift 2
      ;;
    --image)
      IMAGE="${2:?missing value for --image}"
      shift 2
      ;;
    --platform)
      PLATFORM="${2:?missing value for --platform}"
      shift 2
      ;;
    --push)
      PUSH=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$TAG" ]]; then
  if ! TAG="$(git describe --tags --exact-match 2>/dev/null)"; then
    echo "No --tag provided and HEAD is not exactly on a git tag." >&2
    echo "Pass --tag TAG, for example: scripts/build-docker.sh --tag 26.6.0" >&2
    exit 1
  fi
fi

if [[ "$TAG" == "latest" ]]; then
  echo "TAG must not be 'latest'; it is reserved for the rolling image tag." >&2
  exit 1
fi

ROOT_DIR="$(git rev-parse --show-toplevel)"
GOOS="${PLATFORM%%/*}"
GOARCH="${PLATFORM#*/}"
GOARCH="${GOARCH%%/*}"
DIST_DIR="${ROOT_DIR}/dist/server_${GOOS}_${GOARCH}_v1"
BINARY="${DIST_DIR}/rss-zero-server"

cd "$ROOT_DIR"

if [[ "$GOOS" != "linux" || "$GOARCH" != "amd64" ]]; then
  echo "Only linux/amd64 is supported by the Gitea GoReleaser Docker workflow." >&2
  exit 1
fi

mkdir -p "$DIST_DIR"

echo "Building ${BINARY} with version ${TAG}..."
CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" GOPROXY=https://goproxy.cn,direct go build \
  -ldflags "-w -s -X github.com/eli-yip/rss-zero/internal/version.Version=${TAG}" \
  -o "$BINARY" \
  ./cmd/server

echo "Building ${IMAGE}:${TAG} and ${IMAGE}:latest for ${PLATFORM}..."
docker build \
  --platform="$PLATFORM" \
  --build-arg="VERSION=${TAG}" \
  --tag "${IMAGE}:${TAG}" \
  --tag "${IMAGE}:latest" \
  --file docker/Dockerfile.goreleaser \
  "$DIST_DIR"

if [[ "$PUSH" -eq 1 ]]; then
  echo "Pushing ${IMAGE}:${TAG} and ${IMAGE}:latest..."
  docker push "${IMAGE}:${TAG}"
  docker push "${IMAGE}:latest"
fi

echo "Done:"
echo "  ${IMAGE}:${TAG}"
echo "  ${IMAGE}:latest"
