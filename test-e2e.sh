#!/usr/bin/env bash
set -euo pipefail

SERVER_URL="${1:-http://localhost:8090}"
PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }
check() { if [ "$1" = "$2" ]; then pass "$3"; else fail "$3 (got $1, want $2)"; fi }

echo "=== Annotation E2E Smoke Test ==="
echo "Server: ${SERVER_URL}"
echo ""

# 1. Health check
echo "--- Health Check ---"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${SERVER_URL}/health")
check "$STATUS" "200" "GET /health returns 200"

# 2. Create annotation with image
echo "--- Create Annotation ---"
ANNOTATION_JSON='{
  "body": [{"type":"TextualBody","value":"E2E smoke test annotation","purpose":"commenting"}],
  "target": {"source":"http://test.example.com/page","selector":[{"type":"FragmentSelector","conformsTo":"http://www.w3.org/TR/media-frags/","value":"xywh=10,20,100,50"}],"state":{"type":"HttpRequestState","value":"viewport=1024x768"}},
  "motivation": "commenting",
  "creator": {"type":"Person","name":"smoke-test"}
}'

# Create a minimal 1x1 PNG (67 bytes)
TEST_PNG=$(mktemp /tmp/test-XXXXX.png)
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82' > "$TEST_PNG"

CREATE_RESP=$(curl -s -w '\n%{http_code}' "${SERVER_URL}/api/annotations" \
  -F "annotation=${ANNOTATION_JSON}" \
  -F "image=@${TEST_PNG};type=image/png;filename=screenshot.png")
CREATE_STATUS=$(echo "$CREATE_RESP" | tail -1)
CREATE_BODY=$(echo "$CREATE_RESP" | sed '$d')
check "$CREATE_STATUS" "201" "POST /api/annotations returns 201"

ID=$(echo "$CREATE_BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -z "$ID" ]; then
  fail "Could not extract annotation ID from response"
  rm -f "$TEST_PNG"
  echo ""
  echo "=== Results: ${PASS} passed, ${FAIL} failed ==="
  exit 1
fi
pass "Response contains annotation ID: ${ID}"

# 3. List annotations
echo "--- List Annotations ---"
LIST_RESP=$(curl -s -w '\n%{http_code}' "${SERVER_URL}/api/annotations?domain=test.example.com")
LIST_STATUS=$(echo "$LIST_RESP" | tail -1)
LIST_BODY=$(echo "$LIST_RESP" | sed '$d')
check "$LIST_STATUS" "200" "GET /api/annotations returns 200"

if echo "$LIST_BODY" | grep -q "$ID"; then
  pass "List contains created annotation"
else
  fail "List does not contain created annotation"
fi

# 4. Get single annotation
echo "--- Get Annotation ---"
GET_RESP=$(curl -s -w '\n%{http_code}' "${SERVER_URL}/api/annotations/${ID}")
GET_STATUS=$(echo "$GET_RESP" | tail -1)
GET_BODY=$(echo "$GET_RESP" | sed '$d')
check "$GET_STATUS" "200" "GET /api/annotations/${ID} returns 200"

if echo "$GET_BODY" | grep -q '"state":"open"'; then
  pass "Annotation state is open"
else
  fail "Annotation state is not open"
fi

# 5. Get screenshot
echo "--- Get Screenshot ---"
IMG_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${SERVER_URL}/api/annotations/${ID}/image")
check "$IMG_STATUS" "200" "GET /api/annotations/${ID}/image returns 200"

IMG_CT=$(curl -s -o /dev/null -w '%{content_type}' "${SERVER_URL}/api/annotations/${ID}/image")
if echo "$IMG_CT" | grep -q "image/png"; then
  pass "Image content type is image/png"
else
  fail "Image content type is not image/png (got ${IMG_CT})"
fi

# 6. Update annotation
echo "--- Update Annotation ---"
UPDATE_RESP=$(curl -s -w '\n%{http_code}' -X PUT "${SERVER_URL}/api/annotations/${ID}" \
  -H "Content-Type: application/json" \
  -d '{"annotation":{"body":[{"type":"TextualBody","value":"Updated by E2E test","purpose":"commenting"}]}}')
UPDATE_STATUS=$(echo "$UPDATE_RESP" | tail -1)
check "$UPDATE_STATUS" "200" "PUT /api/annotations/${ID} returns 200"

# 7. Resolve annotation
echo "--- Resolve Annotation ---"
RESOLVE_RESP=$(curl -s -w '\n%{http_code}' -X POST "${SERVER_URL}/api/annotations/${ID}/resolve" \
  -H "Content-Type: application/json" \
  -d '{"resolution":{"resolved_by":"smoke-test","note":"E2E verified"}}')
RESOLVE_STATUS=$(echo "$RESOLVE_RESP" | tail -1)
RESOLVE_BODY=$(echo "$RESOLVE_RESP" | sed '$d')
check "$RESOLVE_STATUS" "200" "POST /api/annotations/${ID}/resolve returns 200"

if echo "$RESOLVE_BODY" | grep -q '"state":"resolved"'; then
  pass "Annotation resolved"
else
  fail "Annotation not resolved"
fi

# 8. Delete annotation
echo "--- Delete Annotation ---"
DELETE_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "${SERVER_URL}/api/annotations/${ID}")
check "$DELETE_STATUS" "204" "DELETE /api/annotations/${ID} returns 204"

# 9. Verify deletion
echo "--- Verify Deletion ---"
GONE_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${SERVER_URL}/api/annotations/${ID}")
check "$GONE_STATUS" "404" "GET deleted annotation returns 404"

# Cleanup
rm -f "$TEST_PNG"

echo ""
echo "=== Results: ${PASS} passed, ${FAIL} failed ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
