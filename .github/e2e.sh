#!/usr/bin/env bash

set -euo pipefail

mkdir -p "$HOME/.cache"
tmp=$(mktemp -d "$HOME/.cache/fanout-e2e.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
id=$(basename "$tmp" | tr '[:upper:]' '[:lower:]')
image="fanout-e2e:$id"
network="$id"
upstream="$id-upstream"
fanout="$id-fanout"
query="$id-query"

# Called indirectly by the EXIT trap.
# shellcheck disable=SC2329
cleanup() {
    status=$?
    cleanup_failed=0
    set +e

    if ((status != 0)); then
        echo "upstream CoreDNS logs:" >&2
        docker logs "$upstream" >&2
        echo "fanout CoreDNS logs:" >&2
        docker logs "$fanout" >&2
    fi

    if ! container_names=$(docker container ls --all --format '{{.Names}}'); then
        echo "Failed to inspect E2E containers" >&2
        cleanup_failed=1
    else
        for container in "$query" "$fanout" "$upstream"; do
            if grep -Fxq -- "$container" <<<"$container_names" && ! docker rm -f "$container" >&2; then
                echo "Failed to remove E2E container: $container" >&2
                cleanup_failed=1
            fi
        done
    fi

    if ! network_names=$(docker network ls --format '{{.Name}}'); then
        echo "Failed to inspect E2E network: $network" >&2
        cleanup_failed=1
    elif grep -Fxq -- "$network" <<<"$network_names" && ! docker network rm "$network" >&2; then
        echo "Failed to remove E2E network: $network" >&2
        cleanup_failed=1
    fi

    if ! image_id=$(docker image ls --quiet --filter "reference=$image"); then
        echo "Failed to inspect E2E image: $image" >&2
        cleanup_failed=1
    elif [[ -n "$image_id" ]] && ! docker image rm "$image" >&2; then
        echo "Failed to remove E2E image: $image" >&2
        cleanup_failed=1
    fi

    if ! rm -rf "$tmp"; then
        echo "Failed to remove E2E temporary directory: $tmp" >&2
        cleanup_failed=1
    fi

    if ! container_names=$(docker container ls --all --format '{{.Names}}'); then
        echo "Failed to verify E2E container cleanup" >&2
        cleanup_failed=1
    else
        for container in "$query" "$fanout" "$upstream"; do
            if grep -Fxq -- "$container" <<<"$container_names"; then
                echo "E2E container remains after cleanup: $container" >&2
                cleanup_failed=1
            fi
        done
    fi
    if ! network_names=$(docker network ls --format '{{.Name}}'); then
        echo "Failed to verify E2E network cleanup: $network" >&2
        cleanup_failed=1
    elif grep -Fxq -- "$network" <<<"$network_names"; then
        echo "E2E network remains after cleanup: $network" >&2
        cleanup_failed=1
    fi
    if ! image_id=$(docker image ls --quiet --filter "reference=$image"); then
        echo "Failed to verify E2E image cleanup: $image" >&2
        cleanup_failed=1
    elif [[ -n "$image_id" ]]; then
        echo "E2E image remains after cleanup: $image" >&2
        cleanup_failed=1
    fi
    if [[ -e "$tmp" ]]; then
        echo "E2E temporary directory remains after cleanup: $tmp" >&2
        cleanup_failed=1
    fi

    if ((status == 0 && cleanup_failed != 0)); then
        status=1
    fi
    exit "$status"
}
trap cleanup EXIT

GOOS=linux CGO_ENABLED=0 go -C coredns build -trimpath -o "$tmp/coredns" .
docker build --tag "$image" --file coredns/Dockerfile "$tmp"
docker network create "$network" >/dev/null

cat >"$tmp/upstream.Corefile" <<'EOF'
.:53 {
    hosts {
        192.0.2.42 e2e.fanout.test
    }
}
EOF

docker run --detach \
    --name "$upstream" \
    --network "$network" \
    --volume "$tmp/upstream.Corefile:/Corefile:ro" \
    "$image" -conf /Corefile >/dev/null

upstream_ip=$(docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$upstream")
cat >"$tmp/fanout.Corefile" <<EOF
.:53 {
    fanout . $upstream_ip:53 {
        udp-buffer-size 65535
    }
}
EOF

docker run --detach \
    --name "$fanout" \
    --network "$network" \
    --network-alias fanout \
    --volume "$tmp/fanout.Corefile:/Corefile:ro" \
    "$image" -conf /Corefile >/dev/null

docker pull busybox:1.37.0 >/dev/null
deadline=$((SECONDS + 20))
while :; do
    remaining=$((deadline - SECONDS))
    if ((remaining <= 0)); then
        break
    fi
    timeout "${remaining}s" docker rm -f "$query" >/dev/null 2>&1 || true

    remaining=$((deadline - SECONDS))
    if ((remaining <= 0)); then
        break
    fi
    if output=$(timeout "${remaining}s" docker run --rm \
        --name "$query" \
        --network "$network" \
        busybox:1.37.0 nslookup e2e.fanout.test fanout 2>&1) &&
        awk 'NF == 0 || $1 == "Server:" { answer = 0; next }
             $1 == "Name:" { answer = ($2 == "e2e.fanout.test" || $2 == "e2e.fanout.test."); next }
             answer && $1 == "Address:" && $2 == "192.0.2.42" { found = 1 }
             END { exit !found }' <<<"$output"; then
        printf '%s\n' "$output"
        echo "Docker DNS E2E succeeded"
        exit 0
    fi
    remaining=$((deadline - SECONDS))
    if ((remaining <= 0)); then
        break
    fi
    sleep 1
done

printf '%s\n' "${output:-DNS query did not complete}" >&2
echo "Docker DNS E2E failed after 20 seconds" >&2
exit 1
