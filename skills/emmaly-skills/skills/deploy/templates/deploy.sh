#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# deploy.sh — Deploy a podman-compose stack to a remote server over SSH
#
# Usage:
#   ./deploy.sh deploy [--dry-run]   Build, transfer, and start the stack
#   ./deploy.sh status               Show container states and deployed version
#   ./deploy.sh logs [service]       Stream logs (all services or one)
#   ./deploy.sh teardown             Stop and remove the stack
#   ./deploy.sh rollback             Swap to previous deployment
#   ./deploy.sh preflight            Check remote prerequisites
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DRY_RUN=false
LOCAL_TARBALLS=()
VERSION_TMP=""

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# --- Output helpers ---
info()    { echo -e "${GREEN}✓${NC} $*"; }
# warn goes to stderr: get_version() warns from inside a command substitution,
# and on stdout the warning text would be captured as part of the version.
warn()    { echo -e "${YELLOW}!${NC} $*" >&2; }
error()   { echo -e "${RED}✗${NC} $*" >&2; }
step()    { echo -e "${CYAN}→${NC} $*"; }

# --- Configuration ---
load_config() {
    local config_file="${SCRIPT_DIR}/deploy.conf"
    if [[ ! -f "$config_file" ]]; then
        error "deploy.conf not found in ${SCRIPT_DIR}"
        error "Copy deploy.conf.example to deploy.conf and fill in values"
        exit 1
    fi
    # shellcheck source=/dev/null
    source "$config_file"

    for var in DEPLOY_HOST DEPLOY_USER PROJECT_NAME; do
        if [[ -z "${!var:-}" ]]; then
            error "Required variable ${var} not set in deploy.conf"
            exit 1
        fi
    done

    SECRETS_FILE="${SCRIPT_DIR}/.secrets/.env"

    # Resolve the remote $HOME with a direct ssh call, not the remote() wrapper:
    # under --dry-run the wrapper echoes its command instead of running it, which
    # would make REMOTE_HOME the echoed string and corrupt every remote path.
    REMOTE_HOME=""
    if [[ "$DRY_RUN" != true ]]; then
        REMOTE_HOME="$(ssh -o BatchMode=yes -o ConnectTimeout=10 \
            "${DEPLOY_USER}@${DEPLOY_HOST}" 'echo $HOME' 2>/dev/null)" || true
    fi
    if [[ -z "$REMOTE_HOME" ]]; then
        REMOTE_HOME="/home/${DEPLOY_USER}"
        if [[ "$DRY_RUN" != true ]]; then
            warn "Could not resolve remote \$HOME, assuming ${REMOTE_HOME}"
        fi
    fi
    REMOTE_BASE="${REMOTE_HOME}/deploy/${PROJECT_NAME}"
    REMOTE_CURRENT="${REMOTE_BASE}/current"
    REMOTE_PREVIOUS="${REMOTE_BASE}/previous"
}

# --- SSH helper ---
remote() {
    if [[ "$DRY_RUN" == true ]]; then
        echo "[dry-run] ssh ${DEPLOY_USER}@${DEPLOY_HOST} $*"
        return 0
    fi
    ssh -o BatchMode=yes -o ConnectTimeout=10 "${DEPLOY_USER}@${DEPLOY_HOST}" "$@"
}

remote_scp() {
    if [[ "$DRY_RUN" == true ]]; then
        echo "[dry-run] scp $*"
        return 0
    fi
    scp -o BatchMode=yes -o ConnectTimeout=10 "$@"
}

# --- Cleanup trap ---
cleanup() {
    if [[ ${#LOCAL_TARBALLS[@]} -gt 0 ]]; then
        for tarball in "${LOCAL_TARBALLS[@]}"; do
            rm -f "$tarball"
        done
    fi
    # Only remove the version stamp this run created — never a VERSION file
    # that belongs to the project.
    if [[ -n "$VERSION_TMP" ]]; then
        rm -f "$VERSION_TMP"
    fi
}
trap cleanup EXIT

# --- Version stamping ---
get_version() {
    local version
    if git -C "$SCRIPT_DIR" rev-parse --is-inside-work-tree &>/dev/null; then
        version="$(git -C "$SCRIPT_DIR" rev-parse --short=8 HEAD 2>/dev/null || echo "unknown")"
        if ! git -C "$SCRIPT_DIR" diff --quiet 2>/dev/null || ! git -C "$SCRIPT_DIR" diff --cached --quiet 2>/dev/null; then
            version="${version}-dirty"
            warn "Git working tree is dirty — version will be marked as dirty"
        fi
    else
        version="unknown-$(date +%Y%m%d%H%M%S)"
        warn "Not a git repository — using timestamp as version"
    fi
    echo "$version"
}

# --- Discover locally-built images ---
get_local_images() {
    # Parse podman-compose.yml for services that have a 'build' key.
    # These are the images we need to save and transfer.
    # Services without 'build' (like cloudflared) are pulled on remote.
    local compose_file="${SCRIPT_DIR}/podman-compose.yml"
    if [[ ! -f "$compose_file" ]]; then
        error "podman-compose.yml not found in ${SCRIPT_DIR}"
        exit 1
    fi

    # Buffer each service block and emit at its end, so `image:` and `build:`
    # may appear in either order (the template lists image first).
    local services_with_build
    services_with_build=$(awk -v proj="${PROJECT_NAME}" '
        # YAML scalars may be quoted; podman save would not find an image name
        # with literal quote characters in it. sprintf("%c", 39) is a single
        # quote — it cannot be written literally inside this awk program.
        function unquote(s,   q) {
            q = substr(s, 1, 1)
            if (length(s) > 1 && (q == "\"" || q == sprintf("%c", 39)) &&
                substr(s, length(s), 1) == q) {
                return substr(s, 2, length(s) - 2)
            }
            return s
        }
        function flush(   img) {
            if (svc != "" && has_build) {
                if (image == "") {
                    print "! service \"" svc "\" has build: but no image: — guessing" \
                          " localhost/" svc ":latest; set image: explicitly" > "/dev/stderr"
                }
                img = (image != "") ? image : ("localhost/" svc ":latest")
                gsub(/\$\{PROJECT_NAME\}/, proj, img)
                gsub(/\$PROJECT_NAME/, proj, img)
                print img
            }
            svc = ""; image = ""; has_build = 0
        }
        # Any top-level key closes the current service and the services block
        /^[^[:space:]#]/ {
            flush()
            in_services = ($0 ~ /^services:[[:space:]]*$/)
            next
        }
        !in_services { next }
        # Service name: exactly two spaces of indent, nothing after the colon
        /^  [^[:space:]#][^:]*:[[:space:]]*$/ {
            flush()
            svc = $1; sub(/:$/, "", svc)
            next
        }
        /^[[:space:]]+build:/ { has_build = 1; next }
        /^[[:space:]]+image:[[:space:]]/ { image = unquote($2); next }
        END { flush() }
    ' "$compose_file")

    if [[ -z "$services_with_build" ]]; then
        # Fallback: assume the project name as the image
        echo "localhost/${PROJECT_NAME}:latest"
    else
        echo "$services_with_build"
    fi
}

# =============================================================================
# Commands
# =============================================================================

cmd_preflight() {
    step "Checking remote prerequisites on ${DEPLOY_USER}@${DEPLOY_HOST}..."

    echo ""
    step "SSH connectivity..."
    if remote "echo 'SSH connection successful'" 2>/dev/null; then
        info "SSH connection works"
    else
        error "Cannot SSH to ${DEPLOY_USER}@${DEPLOY_HOST}"
        error "Ensure SSH key auth is configured"
        return 1
    fi

    echo ""
    step "Checking podman..."
    if remote "command -v podman &>/dev/null"; then
        local podman_version
        podman_version=$(remote "podman --version")
        info "podman installed: ${podman_version}"
    else
        error "podman not found on remote"
        return 1
    fi

    step "Checking podman-compose..."
    if remote "command -v podman-compose &>/dev/null"; then
        info "podman-compose installed"
    else
        error "podman-compose not found on remote"
        return 1
    fi

    # Check for external networks referenced in the compose file
    echo ""
    step "Checking external networks..."
    local compose_file="${SCRIPT_DIR}/podman-compose.yml"
    local has_external_nets=false
    if [[ -f "$compose_file" ]]; then
        # Find networks marked as external: true
        # Two-indent-level parse: network names at 2-space indent, attributes at 4+
        local external_nets
        external_nets=$(awk '
            /^networks:/ { in_nets = 1; next }
            /^[a-zA-Z]/ && !/^networks:/ { in_nets = 0 }
            in_nets && /^  [a-zA-Z_-]+:/ && !/^    / {
                net = $1; gsub(/:/, "", net); current_net = net
            }
            in_nets && /^    external:[[:space:]]*true/ {
                print current_net
            }
        ' "$compose_file")

        if [[ -n "$external_nets" ]]; then
            has_external_nets=true
            while IFS= read -r net; do
                if remote "podman network inspect ${net} &>/dev/null"; then
                    info "External network '${net}' exists"
                else
                    warn "External network '${net}' does not exist"
                    warn "Create it with: ssh ${DEPLOY_USER}@${DEPLOY_HOST} podman network create ${net}"
                fi
            done <<< "$external_nets"
        fi
    fi
    if [[ "$has_external_nets" == false ]]; then
        info "No external networks required (companion container pattern)"
    fi

    echo ""
    info "Preflight checks complete"
}

cmd_deploy() {
    step "Starting deployment of ${PROJECT_NAME}..."

    # Preflight
    if [[ ! -f "$SECRETS_FILE" ]]; then
        error ".secrets/.env not found at ${SECRETS_FILE}"
        error "Create it with the required environment variables (see env.example)"
        exit 1
    fi

    if [[ ! -s "$SECRETS_FILE" ]]; then
        error ".secrets/.env is empty"
        exit 1
    fi

    local version
    version="$(get_version)"
    # Stamped into a temp file, not the project root — a project may have its
    # own VERSION file, and this one is only needed for the transfer.
    VERSION_TMP="$(mktemp)"
    echo "$version" > "$VERSION_TMP"
    info "Version: ${version}"

    # Build
    step "Building images..."
    if [[ "$DRY_RUN" == true ]]; then
        echo "[dry-run] podman-compose -f ${SCRIPT_DIR}/podman-compose.yml build"
    else
        podman-compose -f "${SCRIPT_DIR}/podman-compose.yml" build
    fi
    info "Build complete"

    # Save images
    step "Saving images to tarballs..."
    local images
    images="$(get_local_images)"
    while IFS= read -r image; do
        [[ -z "$image" ]] && continue
        local safe_name
        safe_name="$(echo "$image" | tr '/:' '_')"
        local tarball="${SCRIPT_DIR}/${safe_name}.tar"
        LOCAL_TARBALLS+=("$tarball")
        step "  Saving ${image} → ${safe_name}.tar"
        if [[ "$DRY_RUN" == true ]]; then
            echo "[dry-run] podman save -o ${tarball} ${image}"
        else
            podman save -o "$tarball" "$image"
        fi
    done <<< "$images"
    info "Images saved"

    # Rotate on remote
    step "Preparing remote directories..."
    remote "
        rm -rf ${REMOTE_PREVIOUS} && \
        if [ -d ${REMOTE_CURRENT} ]; then
            mv ${REMOTE_CURRENT} ${REMOTE_PREVIOUS}
        fi && \
        mkdir -p ${REMOTE_CURRENT}
    "
    info "Remote directories ready"

    # Transfer
    step "Transferring files to remote..."
    remote_scp "${SCRIPT_DIR}/podman-compose.yml" "${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_CURRENT}/"
    remote_scp "${SECRETS_FILE}" "${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_CURRENT}/.env"
    remote_scp "$VERSION_TMP" "${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_CURRENT}/VERSION"

    for tarball in "${LOCAL_TARBALLS[@]}"; do
        local basename
        basename="$(basename "$tarball")"
        step "  Transferring ${basename}..."
        remote_scp "$tarball" "${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_CURRENT}/"
    done

    # podman-compose interpolates ${PROJECT_NAME} in the compose file from the
    # .env in its working directory. deploy.conf never goes to the remote, so
    # the value has to be carried in the .env or container_name and image both
    # resolve to empty on the remote.
    step "Injecting PROJECT_NAME into remote .env..."
    remote "
        cd ${REMOTE_CURRENT} && \
        if ! grep -q '^PROJECT_NAME=' .env; then
            if [ -n \"\$(tail -c1 .env)\" ]; then echo >> .env; fi
            echo 'PROJECT_NAME=${PROJECT_NAME}' >> .env
        fi
    "

    # Secure the .env file
    remote "chmod 600 ${REMOTE_CURRENT}/.env"
    info "Files transferred"

    # Load images and start. The tarballs stay in the deployment directory so
    # `rollback` can reload them — the remote image store may have been pruned
    # since. Only two snapshots exist at a time (current + previous), so this
    # costs at most two copies of the images.
    step "Loading images on remote..."
    remote "
        cd ${REMOTE_CURRENT} && \
        for f in *.tar; do
            if [ -f \"\$f\" ]; then podman load -i \"\$f\"; fi
        done
    "
    info "Images loaded"

    step "Starting stack..."
    remote "cd ${REMOTE_CURRENT} && podman-compose up -d"
    info "Stack started"

    # Health check
    step "Waiting for containers to stabilize..."
    if [[ "$DRY_RUN" != true ]]; then
        sleep 5
    fi

    echo ""
    step "Container status:"
    remote "cd ${REMOTE_CURRENT} && podman-compose ps" || true

    echo ""
    info "Deployed ${PROJECT_NAME} version ${version} to ${DEPLOY_HOST}"
}

cmd_status() {
    step "Checking status of ${PROJECT_NAME} on ${DEPLOY_HOST}..."

    echo ""
    # Version
    local version
    version=$(remote "cat ${REMOTE_CURRENT}/VERSION 2>/dev/null" || echo "unknown")
    info "Deployed version: ${version}"

    echo ""
    step "Container status:"
    remote "cd ${REMOTE_CURRENT} && podman-compose ps" 2>/dev/null || {
        warn "No deployment found at ${REMOTE_CURRENT}"
        return 1
    }

    echo ""
    step "Resource usage:"
    remote "cd ${REMOTE_CURRENT} && podman-compose ps -q | xargs -r podman stats --no-stream" 2>/dev/null || true

    # Check if previous deployment exists
    if remote "[ -d ${REMOTE_PREVIOUS} ]" 2>/dev/null; then
        local prev_version
        prev_version=$(remote "cat ${REMOTE_PREVIOUS}/VERSION 2>/dev/null" || echo "unknown")
        echo ""
        info "Previous version available for rollback: ${prev_version}"
    fi
}

cmd_logs() {
    local service="${1:-}"
    if [[ -n "$service" ]]; then
        step "Streaming logs for ${service} on ${DEPLOY_HOST}..."
        remote "cd ${REMOTE_CURRENT} && podman-compose logs -f ${service}"
    else
        step "Streaming all logs on ${DEPLOY_HOST}..."
        remote "cd ${REMOTE_CURRENT} && podman-compose logs -f"
    fi
}

cmd_teardown() {
    step "Tearing down ${PROJECT_NAME} on ${DEPLOY_HOST}..."

    read -rp "Are you sure you want to tear down the deployment? [y/N] " confirm
    if [[ "$confirm" != [yY] ]]; then
        warn "Teardown cancelled"
        return 0
    fi

    remote "cd ${REMOTE_CURRENT} && podman-compose down" 2>/dev/null || true
    info "Stack stopped and removed"

    read -rp "Also remove deployment files from remote? [y/N] " confirm_files
    if [[ "$confirm_files" == [yY] ]]; then
        remote "rm -rf ${REMOTE_BASE}"
        info "Deployment files removed"
    else
        warn "Deployment files left in place at ${REMOTE_BASE}"
    fi
}

cmd_rollback() {
    step "Rolling back ${PROJECT_NAME} on ${DEPLOY_HOST}..."

    # Check previous exists
    if ! remote "[ -d ${REMOTE_PREVIOUS} ]" 2>/dev/null; then
        error "No previous deployment found — nothing to roll back to"
        exit 1
    fi

    local current_version prev_version
    current_version=$(remote "cat ${REMOTE_CURRENT}/VERSION 2>/dev/null" || echo "unknown")
    prev_version=$(remote "cat ${REMOTE_PREVIOUS}/VERSION 2>/dev/null" || echo "unknown")

    step "Current version: ${current_version}"
    step "Rolling back to: ${prev_version}"

    # Verify the previous snapshot can actually be restored BEFORE touching the
    # running stack. Deployments made before image tarballs were retained have
    # nothing to reload, and the images they need may have been pruned since.
    if ! remote "ls ${REMOTE_PREVIOUS}/*.tar >/dev/null 2>&1"; then
        error "The previous deployment (${prev_version}) has no image tarballs"
        error "Nothing to restore from — re-deploy the desired version instead."
        exit 1
    fi

    # Stop current
    step "Stopping current stack..."
    remote "cd ${REMOTE_CURRENT} && podman-compose down" 2>/dev/null || true

    # Swap current <-> previous
    step "Swapping deployments..."
    remote "
        cd ${REMOTE_BASE} && \
        mv current tmp_rollback && \
        mv previous current && \
        mv tmp_rollback previous
    "

    # Load images from restored deployment. Failures here are fatal: starting
    # the stack against whatever happens to be in the image store would
    # silently "roll back" to the wrong build.
    step "Loading images from restored deployment..."
    if ! remote "
        cd ${REMOTE_CURRENT} && \
        for f in *.tar; do
            podman load -i \"\$f\" || exit 1
        done
    "; then
        error "Failed to load images for ${prev_version} — stack not started"
        error "The deployment directories have been swapped; ${REMOTE_CURRENT}"
        error "now holds ${prev_version}. Fix the images and run 'up -d' there,"
        error "or re-deploy."
        exit 1
    fi

    # Start restored stack
    step "Starting restored stack..."
    remote "cd ${REMOTE_CURRENT} && podman-compose up -d"

    echo ""
    info "Rolled back ${PROJECT_NAME} from ${current_version} to ${prev_version}"
}

# =============================================================================
# Main
# =============================================================================

usage() {
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  deploy [--dry-run]   Build, transfer, and start the stack"
    echo "  status               Show container states and deployed version"
    echo "  logs [service]       Stream logs (all services or one)"
    echo "  teardown             Stop and remove the stack"
    echo "  rollback             Swap to previous deployment"
    echo "  preflight            Check remote prerequisites"
}

main() {
    if [[ $# -lt 1 ]]; then
        usage
        exit 1
    fi

    local command="$1"
    shift

    # Parse global flags
    for arg in "$@"; do
        case "$arg" in
            --dry-run)
                DRY_RUN=true
                warn "Dry-run mode — no changes will be made"
                ;;
        esac
    done

    load_config

    case "$command" in
        deploy)    cmd_deploy ;;
        status)    cmd_status ;;
        logs)      cmd_logs "${1:-}" ;;
        teardown)  cmd_teardown ;;
        rollback)  cmd_rollback ;;
        preflight) cmd_preflight ;;
        help|-h|--help) usage ;;
        *)
            error "Unknown command: ${command}"
            usage
            exit 1
            ;;
    esac
}

main "$@"
