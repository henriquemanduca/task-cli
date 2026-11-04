#!/usr/bin/env bash
set -euo pipefail

APP_NAME="task-cli"
GITHUB_REPO="seu-usuario/task-cli"
PREFIX="${PREFIX:-$HOME/.local}"
INSTALL_DIR="${INSTALL_DIR:-$PREFIX/bin}"
ACTION="install"

# Processa argumentos de linha de comando
for arg in "$@"; do
  case "$arg" in
    --system)
      INSTALL_DIR="/usr/local/bin"
      ;;
    --user)
      INSTALL_DIR="$HOME/.local/bin"
      ;;
    --uninstall)
      ACTION="uninstall"
      ;;
    -h|--help)
      echo "Uso: $0 [OPÇÕES]"
      echo ""
      echo "Opções:"
      echo "  --user        Instala em ~/.local/bin (padrão para usuários comuns)"
      echo "  --system      Instala em /usr/local/bin (requer sudo)"
      echo "  --uninstall   Remove o binário do diretório de instalação"
      echo "  -h, --help    Exibe esta ajuda"
      echo ""
      echo "Variáveis de ambiente suportadas:"
      echo "  INSTALL_DIR   Diretório de destino personalizado"
      echo "  BASE_URL      URL base para download do binário (para servidor local ou espelho)"
      echo "  VERSION       Versão específica para download (padrão: latest)"
      exit 0
      ;;
  esac
done

# Se executado como root e INSTALL_DIR não foi customizado, padrão é /usr/local/bin
if [ "$EUID" -eq 0 ] && [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
fi

TARGET_BIN="${INSTALL_DIR}/${APP_NAME}"

# Desinstalação
if [ "$ACTION" = "uninstall" ]; then
  echo "==> Removendo ${APP_NAME} de ${INSTALL_DIR}..."
  if [ -f "$TARGET_BIN" ]; then
    if [ -w "$INSTALL_DIR" ] && [ -w "$TARGET_BIN" ]; then
      rm -f "$TARGET_BIN"
    else
      echo "--> Requer permissões de superusuário para remover de ${INSTALL_DIR}..."
      sudo rm -f "$TARGET_BIN"
    fi
    echo "✔ ${APP_NAME} desinstalado com sucesso."
  else
    echo "--> ${APP_NAME} não foi encontrado em ${INSTALL_DIR}."
  fi
  exit 0
fi

# Detecta Arquitetura
ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  armv7l)        ARCH="armv7" ;;
  *)
    echo "Erro: arquitetura '$ARCH_RAW' não suportada." >&2
    exit 1
    ;;
esac

# Detecta Sistema Operacional
OS_RAW="$(uname -s)"
case "$OS_RAW" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)
    echo "Erro: sistema operacional '$OS_RAW' não suportado." >&2
    exit 1
    ;;
esac

BIN_NAME="${APP_NAME}-${OS}-${ARCH}"

echo "==> Instalando/Atualizando ${APP_NAME} em ${INSTALL_DIR}..."

# Localiza ou baixa o binário
TMP_FILE=""
CLEANUP_TMP=false

if [ -f "dist/${BIN_NAME}" ]; then
  echo "--> Usando binário local: dist/${BIN_NAME}"
  SOURCE_BIN="dist/${BIN_NAME}"
elif [ -f "dist/${APP_NAME}" ]; then
  echo "--> Usando binário local: dist/${APP_NAME}"
  SOURCE_BIN="dist/${APP_NAME}"
elif [ -f "./${APP_NAME}" ]; then
  echo "--> Usando binário local: ./${APP_NAME}"
  SOURCE_BIN="./${APP_NAME}"
else
  # Modo Remoto (curl | bash ou repositório remoto)
  VERSION="${VERSION:-latest}"
  if [ -n "${BASE_URL:-}" ]; then
    DOWNLOAD_URL="${BASE_URL%/}/${BIN_NAME}"
  elif [ "$VERSION" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/${BIN_NAME}"
  else
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${BIN_NAME}"
  fi

  echo "--> Baixando ${APP_NAME} (${OS}/${ARCH}) de: ${DOWNLOAD_URL}"
  TMP_FILE="$(mktemp -t task-cli-XXXXXX)"
  CLEANUP_TMP=true
  trap 'rm -f "$TMP_FILE"' EXIT

  if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"; then
    echo "Erro: falha ao baixar o binário de $DOWNLOAD_URL" >&2
    exit 1
  fi
  SOURCE_BIN="$TMP_FILE"
fi

# Garante que o diretório de destino existe
if [ ! -d "$INSTALL_DIR" ]; then
  if mkdir -p "$INSTALL_DIR" 2>/dev/null; then
    :
  else
    sudo mkdir -p "$INSTALL_DIR"
  fi
fi

# Instalação atômica
if [ -w "$INSTALL_DIR" ]; then
  install -m 755 "$SOURCE_BIN" "$TARGET_BIN"
else
  echo "--> Requer privilégios de superusuário para instalar em ${INSTALL_DIR}..."
  sudo install -m 755 "$SOURCE_BIN" "$TARGET_BIN"
fi

if [ "$CLEANUP_TMP" = true ]; then
  rm -f "$TMP_FILE"
  trap - EXIT
fi

echo "✔ ${APP_NAME} instalado com sucesso em ${TARGET_BIN}"

# Exibe versão instalada
if command -v "${APP_NAME}" >/dev/null 2>&1; then
  echo "--> Versão ativa: $("${APP_NAME}" --version 2>/dev/null || echo 'ok')"
else
  echo "--> Versão instalada: $("${TARGET_BIN}" --version 2>/dev/null || echo 'ok')"
  if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo ""
    echo "⚠️  Aviso: O diretório '${INSTALL_DIR}' não está no seu PATH."
    echo "   Para usar o comando 'task-cli' diretamente em qualquer terminal, adicione:"
    echo "   export PATH=\"\$PATH:${INSTALL_DIR}\""
    echo "   no seu arquivo ~/.bashrc ou ~/.zshrc."
  fi
fi

