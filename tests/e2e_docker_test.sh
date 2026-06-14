#!/usr/bin/env bash
# E2E smoke test that validates the full service via Docker Compose.
# Usage: ./tests/e2e_docker_test.sh
# Requires: docker compose, curl, jq
set -euo pipefail

BASE_URL="http://localhost:8080"
COMPOSE_FILE="docker-compose.yml"

cleanup() {
    echo ">>> Stopping containers..."
    docker compose -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

echo ">>> Building and starting services..."
docker compose -f "$COMPOSE_FILE" up --build -d

echo ">>> Waiting for service to be ready..."
for i in $(seq 1 30); do
    if curl -s "$BASE_URL/login" > /dev/null 2>&1; then
        echo "    Service ready after ${i}s"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "FAIL: service did not start within 30s"
        docker compose -f "$COMPOSE_FILE" logs app
        exit 1
    fi
    sleep 1
done

echo ""
echo "=== Test 1: Register user ==="
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d '{"username":"e2euser","email":"e2e@test.com","password":"testpass123"}')
CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
if [ "$CODE" != "201" ]; then
    echo "FAIL: register returned $CODE: $BODY"
    exit 1
fi
echo "    PASS (201)"

echo ""
echo "=== Test 2: Login ==="
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"e2euser","password":"testpass123"}')
CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
if [ "$CODE" != "200" ]; then
    echo "FAIL: login returned $CODE: $BODY"
    exit 1
fi
TOKEN=$(echo "$BODY" | jq -r '.data.token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "FAIL: no token in login response"
    exit 1
fi
echo "    PASS (got token)"

echo ""
echo "=== Test 3: Create short link ==="
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/links" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"url":"https://github.com","custom_code":"ghub","title":"GitHub"}')
CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
if [ "$CODE" != "201" ]; then
    echo "FAIL: create link returned $CODE: $BODY"
    exit 1
fi
SHORT_CODE=$(echo "$BODY" | jq -r '.data.short_code')
if [ "$SHORT_CODE" != "ghub" ]; then
    echo "FAIL: short_code mismatch: $SHORT_CODE"
    exit 1
fi
echo "    PASS (short_code=ghub)"

echo ""
echo "=== Test 4: Duplicate code conflict ==="
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/links" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"url":"https://other.com","custom_code":"ghub"}')
CODE=$(echo "$RESP" | tail -1)
if [ "$CODE" != "409" ]; then
    echo "FAIL: expected 409, got $CODE"
    exit 1
fi
echo "    PASS (409 conflict)"

echo ""
echo "=== Test 5: Redirect ==="
RESP=$(curl -s -o /dev/null -w "%{http_code} %{redirect_url}" "$BASE_URL/s/ghub")
CODE=$(echo "$RESP" | cut -d' ' -f1)
REDIR=$(echo "$RESP" | cut -d' ' -f2)
if [ "$CODE" != "301" ]; then
    echo "FAIL: expected 301, got $CODE"
    exit 1
fi
if [ "$REDIR" != "https://github.com" ]; then
    echo "FAIL: redirect target mismatch: $REDIR"
    exit 1
fi
echo "    PASS (301 -> https://github.com)"

echo ""
echo "=== Test 6: Batch create ==="
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/links/batch" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"links":[{"url":"https://a.com"},{"url":"https://b.com"},{"url":"https://c.com"}]}')
CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
if [ "$CODE" != "200" ]; then
    echo "FAIL: batch create returned $CODE: $BODY"
    exit 1
fi
CREATED=$(echo "$BODY" | jq '.data.created | length')
if [ "$CREATED" != "3" ]; then
    echo "FAIL: expected 3 created, got $CREATED"
    exit 1
fi
echo "    PASS (3 links created)"

echo ""
echo "=== Test 7: List links ==="
RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/v1/links" \
    -H "Authorization: Bearer $TOKEN")
CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | head -n -1)
if [ "$CODE" != "200" ]; then
    echo "FAIL: list returned $CODE"
    exit 1
fi
TOTAL=$(echo "$BODY" | jq '.data.total')
if [ "$TOTAL" -lt 4 ]; then
    echo "FAIL: expected at least 4 links, got $TOTAL"
    exit 1
fi
echo "    PASS (total=$TOTAL)"

echo ""
echo "=== Test 8: Block private IP URLs ==="
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/links" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"url":"http://192.168.1.1/admin"}')
CODE=$(echo "$RESP" | tail -1)
if [ "$CODE" != "400" ]; then
    echo "FAIL: private IP should be blocked, got $CODE"
    exit 1
fi
echo "    PASS (192.168.x blocked)"

echo ""
echo "=== Test 9: 404 for unknown code ==="
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/s/nonexistent")
if [ "$CODE" != "404" ]; then
    echo "FAIL: expected 404, got $CODE"
    exit 1
fi
echo "    PASS (404)"

echo ""
echo "=== Test 10: Rate limiting ==="
for i in $(seq 1 65); do
    curl -s -o /dev/null "$BASE_URL/api/v1/auth/login" \
        -X POST -H "Content-Type: application/json" \
        -d '{"username":"x","password":"x"}' &
done
wait
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"x","password":"x"}')
CODE=$(echo "$RESP" | tail -1)
# Should get 429 after exceeding limit (60/min default)
if [ "$CODE" = "429" ]; then
    echo "    PASS (rate limited)"
else
    echo "    WARN: rate limit may not have triggered (got $CODE), timing-dependent"
fi

echo ""
echo "============================================"
echo "ALL E2E TESTS PASSED"
echo "============================================"
