#!/bin/sh
# Commit Tool Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/dsswift/commit/main/scripts/install.sh | sh

set -e

# Configuration
REPO="dsswift/commit"
INSTALL_DIR="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.commit-tool"

# Colors (if terminal supports them)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Detect OS and architecture
detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            echo "${RED}Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac

    case "$OS" in
        darwin|linux)
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            echo "${RED}Unsupported OS: $OS${NC}"
            exit 1
            ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    echo "Detected platform: ${PLATFORM}"
}

# Get latest release version
get_latest_version() {
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "${RED}Failed to get latest version${NC}"
        exit 1
    fi
    echo "Latest version: ${VERSION}"
}

# Download binary
download_binary() {
    EXT=""
    if [ "$OS" = "windows" ]; then
        EXT=".exe"
    fi

    FILENAME="commit-${PLATFORM}${EXT}"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

    echo "Downloading from: ${URL}"

    TEMP_FILE=$(mktemp)
    if ! curl -fsSL "$URL" -o "$TEMP_FILE"; then
        echo "${RED}Download failed${NC}"
        rm -f "$TEMP_FILE"
        exit 1
    fi

    chmod +x "$TEMP_FILE"
    DOWNLOADED_FILE="$TEMP_FILE"
}

# Install binary
install_binary() {
    mkdir -p "$INSTALL_DIR"

    BINARY_NAME="commit"
    if [ "$OS" = "windows" ]; then
        BINARY_NAME="commit.exe"
    fi

    TARGET="${INSTALL_DIR}/${BINARY_NAME}"

    mv "$DOWNLOADED_FILE" "$TARGET"
    chmod +x "$TARGET"

    echo "${GREEN}Installed to: ${TARGET}${NC}"
}

# Create config directory and template
create_config() {
    mkdir -p "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"

    ENV_FILE="${CONFIG_DIR}/.env"
    if [ ! -f "$ENV_FILE" ]; then
        cat > "$ENV_FILE" << 'EOF'
# Commit Tool Configuration
# Documentation: https://github.com/dsswift/commit#configuration

# ═══════════════════════════════════════════════════════════════════════════════
# PROVIDER SELECTION (required)
# ═══════════════════════════════════════════════════════════════════════════════
# Choose one: anthropic | openai | grok | gemini | azure-foundry
COMMIT_PROVIDER=

# ═══════════════════════════════════════════════════════════════════════════════
# PUBLIC CLOUD API KEYS (use one matching your provider)
# ═══════════════════════════════════════════════════════════════════════════════
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
GROK_API_KEY=
GEMINI_API_KEY=

# ═══════════════════════════════════════════════════════════════════════════════
# AZURE AI FOUNDRY (private cloud - optional)
# ═══════════════════════════════════════════════════════════════════════════════
# For organizations using Azure-hosted models
AZURE_FOUNDRY_ENDPOINT=
AZURE_FOUNDRY_API_KEY=
AZURE_FOUNDRY_DEPLOYMENT=

# ═══════════════════════════════════════════════════════════════════════════════
# OPTIONAL SETTINGS
# ═══════════════════════════════════════════════════════════════════════════════
# Override default model for your provider
# COMMIT_MODEL=claude-3-5-sonnet

# Always preview without committing (useful for testing)
# COMMIT_DRY_RUN=true
EOF
        chmod 600 "$ENV_FILE"
        echo "${GREEN}Created config template: ${ENV_FILE}${NC}"
    else
        echo "${YELLOW}Config file already exists: ${ENV_FILE}${NC}"
    fi
}

# Check PATH
check_path() {
    case ":$PATH:" in
        *":$INSTALL_DIR:"*)
            echo "${GREEN}${INSTALL_DIR} is already in PATH${NC}"
            ;;
        *)
            echo "${YELLOW}Add ${INSTALL_DIR} to your PATH${NC}"
            echo ""
            echo "Add this line to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
            echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
            ;;
    esac
}

# Main installation
main() {
    echo "🚀 Installing Commit Tool..."
    echo ""

    detect_platform
    get_latest_version
    download_binary
    install_binary
    create_config
    echo ""
    check_path

    echo ""
    echo "${GREEN}✅ Installation complete!${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Edit ${CONFIG_DIR}/.env to configure your LLM provider"
    echo "  2. Run 'commit --version' to verify installation"
    echo "  3. Run 'commit' in a git repository to create smart commits"
    echo ""
    echo "Documentation: https://github.com/${REPO}#readme"
}

main
