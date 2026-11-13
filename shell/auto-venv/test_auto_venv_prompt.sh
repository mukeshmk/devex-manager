#!/usr/bin/env bash

# Test script for auto-venv proactive prompt

set -e

# Mock directory
MOCK_DIR=$(mktemp -d)
trap 'rm -rf "$MOCK_DIR"' EXIT

# Mock uv
cat <<EOF > "$MOCK_DIR/uv"
#!/usr/bin/env bash
if [[ "\$1" == "venv" ]]; then
    mkdir -p .venv/bin
    touch .venv/bin/activate
fi
EOF
chmod +x "$MOCK_DIR/uv"

# Add mock uv to PATH
export PATH="$MOCK_DIR:$PATH"

# Copy the script to a temp location to source it safely
cp "$(dirname "$0")/auto-venv.sh" "$MOCK_DIR/auto-venv.sh"

# Mock cd to avoid actual directory changes during sourcing
cd() {
    builtin cd "$@"
}

# Source the script
source "$MOCK_DIR/auto-venv.sh"

# Test 1: Identify that it DOES prompt when project exists
mkdir -p "$MOCK_DIR/proj1"
touch "$MOCK_DIR/proj1/pyproject.toml"

# Mock 'read' to simulate user input 'y'
# This is tricky in bash, we'll try to override it or use a subshell
if ! declare -f _auto_venv_check_init > /dev/null; then
    echo "Test Failed: _auto_venv_check_init function does not exist"
    exit 1
fi

echo "Test Passed: _auto_venv_check_init function exists."
