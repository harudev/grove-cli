#!/usr/bin/env bash
set -euo pipefail

REPO="harudev/grove-cli"
INSTALL_DIR="/usr/local/bin"
BINARY="grove"

# OS/Arch 감지
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "지원하지 않는 아키텍처: $ARCH"; exit 1 ;;
esac

if [ "$OS" != "darwin" ]; then
  echo "현재 macOS만 지원합니다."
  exit 1
fi

# 최신 릴리스 태그 조회
echo "최신 릴리스를 확인하는 중..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$TAG" ]; then
  echo "릴리스를 찾을 수 없습니다."
  exit 1
fi
VERSION="${TAG#v}"
echo "최신 버전: ${TAG}"

# 다운로드
ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "다운로드 중: ${ARCHIVE}"
curl -fsSL -o "${TMP_DIR}/${ARCHIVE}" "$URL"

# 설치
echo "설치 중: ${INSTALL_DIR}/${BINARY}"
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"
sudo install -m 755 "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo ""
echo "grove ${TAG} 설치 완료!"
echo ""
echo "초기 설정을 진행하세요:"
echo "  grove setup"
