#!/bin/bash
# =============================================================================
# CAPI Provider Rebrand Validation Script
# =============================================================================
# Usage: ./validate.sh [--verbose]
#
# Validates that the rebrand from Kamaji to Steward was successful.
# Returns exit code 0 if all checks pass, 1 if any fail.
# =============================================================================

set -uo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

VERBOSE=false
if [[ "${1:-}" == "--verbose" ]]; then
    VERBOSE=true
fi

ERRORS=0
WARNINGS=0

# Helper function for logging
log_check() {
    echo -e "${BLUE}[CHECK]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((ERRORS++))
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    ((WARNINGS++))
}

log_info() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "       $1"
    fi
}

echo -e "${BLUE}==============================================================================${NC}"
echo -e "${BLUE}  CAPI Control Plane Provider Rebrand Validation${NC}"
echo -e "${BLUE}==============================================================================${NC}"
echo ""

# =============================================================================
# CHECK 1: Verify go.mod module path
# =============================================================================
log_check "Verifying go.mod module path..."

if grep -q "^module github.com/butlerdotdev/cluster-api-control-plane-provider-steward" go.mod; then
    log_pass "Module path is correct"
else
    log_fail "Module path not updated"
    log_info "Expected: module github.com/butlerdotdev/cluster-api-control-plane-provider-steward"
    log_info "Found: $(head -1 go.mod)"
fi

# =============================================================================
# CHECK 2: Verify Steward dependency
# =============================================================================
log_check "Verifying Steward dependency..."

if grep -q "github.com/butlerdotdev/steward" go.mod; then
    log_pass "Steward dependency present"
    if [[ "$VERBOSE" == "true" ]]; then
        log_info "$(grep 'github.com/butlerdotdev/steward' go.mod)"
    fi
else
    log_fail "Steward dependency not found in go.mod"
fi

# =============================================================================
# CHECK 3: No remaining clastix references
# =============================================================================
log_check "Checking for remaining 'clastix' references..."

clastix_refs=$(grep -ri "clastix" --include="*.go" --include="*.yaml" --include="*.yml" . 2>/dev/null | grep -v "NOTICE" | grep -v ".git" || true)
if [[ -z "$clastix_refs" ]]; then
    log_pass "No 'clastix' references found"
else
    log_fail "Found 'clastix' references:"
    echo "$clastix_refs" | head -10
    count=$(echo "$clastix_refs" | wc -l)
    if [[ $count -gt 10 ]]; then
        log_info "... and $((count - 10)) more"
    fi
fi

# =============================================================================
# CHECK 4: No remaining kamaji references (excluding expected ones)
# =============================================================================
log_check "Checking for remaining 'kamaji' references..."

# Exclude NOTICE file, .git, and legitimate Kamaji references in comments about origin
kamaji_refs=$(grep -ri "kamaji" --include="*.go" --include="*.yaml" --include="*.yml" . 2>/dev/null | \
    grep -v "NOTICE" | \
    grep -v ".git" | \
    grep -v "// Based on" | \
    grep -v "// Originally from" | \
    grep -v "# Based on" || true)

if [[ -z "$kamaji_refs" ]]; then
    log_pass "No unexpected 'kamaji' references found"
else
    log_fail "Found 'kamaji' references:"
    echo "$kamaji_refs" | head -10
    count=$(echo "$kamaji_refs" | wc -l)
    if [[ $count -gt 10 ]]; then
        log_info "... and $((count - 10)) more"
    fi
fi

# =============================================================================
# CHECK 5: No remaining Kamaji references (capitalized)
# =============================================================================
log_check "Checking for remaining 'Kamaji' references..."

Kamaji_refs=$(grep -r "Kamaji" --include="*.go" --include="*.yaml" --include="*.yml" . 2>/dev/null | \
    grep -v "NOTICE" | \
    grep -v ".git" | \
    grep -v "README" | \
    grep -v "Based on" || true)

if [[ -z "$Kamaji_refs" ]]; then
    log_pass "No unexpected 'Kamaji' references found"
else
    log_fail "Found 'Kamaji' references:"
    echo "$Kamaji_refs" | head -10
    count=$(echo "$Kamaji_refs" | wc -l)
    if [[ $count -gt 10 ]]; then
        log_info "... and $((count - 10)) more"
    fi
fi

# =============================================================================
# CHECK 6: Verify StewardControlPlane type exists
# =============================================================================
log_check "Verifying StewardControlPlane type exists..."

if grep -rq "type StewardControlPlane struct" --include="*.go" .; then
    log_pass "StewardControlPlane type found"
else
    log_fail "StewardControlPlane type not found"
fi

# =============================================================================
# CHECK 7: Verify StewardControlPlaneTemplate type exists
# =============================================================================
log_check "Verifying StewardControlPlaneTemplate type exists..."

if grep -rq "type StewardControlPlaneTemplate struct" --include="*.go" .; then
    log_pass "StewardControlPlaneTemplate type found"
else
    log_fail "StewardControlPlaneTemplate type not found"
fi

# =============================================================================
# CHECK 8: Verify kubebuilder markers updated
# =============================================================================
log_check "Verifying kubebuilder shortName markers..."

if grep -rq "shortName=stcp" --include="*.go" .; then
    log_pass "shortName=stcp marker found"
else
    log_fail "shortName=stcp marker not found"
fi

if grep -rq "shortName=stcpt" --include="*.go" .; then
    log_pass "shortName=stcpt marker found"
else
    log_fail "shortName=stcpt marker not found"
fi

# =============================================================================
# CHECK 9: Verify categories updated
# =============================================================================
log_check "Verifying kubebuilder categories..."

if grep -rq "categories=cluster-api;steward" --include="*.go" .; then
    log_pass "categories=cluster-api;steward marker found"
else
    log_fail "categories=cluster-api;steward marker not found"
fi

# Check that old categories are gone
if grep -rq "categories=cluster-api;kamaji" --include="*.go" .; then
    log_fail "Old categories=cluster-api;kamaji marker still present"
else
    log_pass "No old categories=cluster-api;kamaji markers"
fi

# =============================================================================
# CHECK 10: Verify steward.butlerlabs.dev labels present
# =============================================================================
log_check "Verifying steward.butlerlabs.dev labels/annotations..."

if grep -rq "steward.butlerlabs.dev" --include="*.go" .; then
    log_pass "steward.butlerlabs.dev references found"
    if [[ "$VERBOSE" == "true" ]]; then
        count=$(grep -r "steward.butlerlabs.dev" --include="*.go" . | wc -l)
        log_info "Found $count references"
    fi
else
    log_fail "No steward.butlerlabs.dev references found"
fi

# =============================================================================
# CHECK 11: Verify no kamaji.clastix.io labels remain
# =============================================================================
log_check "Checking for remaining kamaji.clastix.io labels..."

if grep -rq "kamaji\.clastix\.io" --include="*.go" --include="*.yaml" .; then
    log_fail "Found kamaji.clastix.io references"
    grep -r "kamaji\.clastix\.io" --include="*.go" --include="*.yaml" . | head -5
else
    log_pass "No kamaji.clastix.io references found"
fi

# =============================================================================
# CHECK 12: Verify file renames completed
# =============================================================================
log_check "Verifying file renames..."

# Check that old files don't exist
old_files_exist=false
for f in api/v1alpha1/kamajicontrolplane*.go controllers/kamajicontrolplane*.go; do
    if [[ -f "$f" ]]; then
        log_fail "Old file still exists: $f"
        old_files_exist=true
    fi
done

if [[ "$old_files_exist" == "false" ]]; then
    log_pass "No old kamajicontrolplane*.go files found"
fi

# Check that new files exist
new_files_exist=true
for expected in "api/v1alpha1/stewardcontrolplane_types.go" "controllers/stewardcontrolplane_controller.go"; do
    if [[ ! -f "$expected" ]]; then
        log_fail "Expected file not found: $expected"
        new_files_exist=false
    fi
done

if [[ "$new_files_exist" == "true" ]]; then
    log_pass "New stewardcontrolplane*.go files exist"
fi

# =============================================================================
# CHECK 13: Verify imports are correct
# =============================================================================
log_check "Verifying import statements..."

# Check for stewardv1alpha1 import alias
if grep -rq 'stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"' --include="*.go" .; then
    log_pass "stewardv1alpha1 import alias found"
else
    log_fail "stewardv1alpha1 import alias not found"
fi

# Check for scpv1alpha1 import alias  
if grep -rq 'scpv1alpha1 "github.com/butlerdotdev/cluster-api-control-plane-provider-steward/api/v1alpha1"' --include="*.go" .; then
    log_pass "scpv1alpha1 import alias found"
else
    log_warn "scpv1alpha1 import alias not found (may be using different alias)"
fi

# =============================================================================
# CHECK 14: Verify no double-replacements
# =============================================================================
log_check "Checking for double-replacement artifacts..."

doubles_found=false

if grep -rq "scpscp" --include="*.go" .; then
    log_fail "Found 'scpscp' double-replacement"
    doubles_found=true
fi

if grep -rq "kcpkcp" --include="*.go" .; then
    log_fail "Found 'kcpkcp' double-replacement"
    doubles_found=true
fi

if grep -rq "stewardsteward" --include="*.go" --include="*.yaml" .; then
    log_fail "Found 'stewardsteward' double-replacement"
    doubles_found=true
fi

if grep -rq "StewardSteward" --include="*.go" --include="*.yaml" .; then
    log_fail "Found 'StewardSteward' double-replacement"
    doubles_found=true
fi

if [[ "$doubles_found" == "false" ]]; then
    log_pass "No double-replacement artifacts found"
fi

# =============================================================================
# CHECK 15: Verify RBAC markers updated
# =============================================================================
log_check "Verifying RBAC markers..."

if grep -rq "groups=steward.butlerlabs.dev" --include="*.go" .; then
    log_pass "RBAC markers updated to steward.butlerlabs.dev"
else
    log_warn "No steward.butlerlabs.dev RBAC markers found (may be expected)"
fi

# Check old RBAC markers are gone (except for controlplane.cluster.x-k8s.io which should stay)
if grep -rq "groups=kamaji.clastix.io" --include="*.go" .; then
    log_fail "Old kamaji.clastix.io RBAC markers still present"
else
    log_pass "No old kamaji.clastix.io RBAC markers"
fi

# =============================================================================
# CHECK 16: Verify controlplane.cluster.x-k8s.io API group preserved
# =============================================================================
log_check "Verifying CAPI standard API group preserved..."

if grep -rq 'controlplane.cluster.x-k8s.io' --include="*.go" .; then
    log_pass "controlplane.cluster.x-k8s.io API group preserved (CAPI standard)"
else
    log_fail "controlplane.cluster.x-k8s.io API group not found - this should NOT be changed!"
fi

# =============================================================================
# CHECK 17: Verify copyright updated
# =============================================================================
log_check "Verifying copyright headers..."

# Count files with Butler Labs copyright
butler_copyright=0
while IFS= read -r line; do
    ((butler_copyright++))
done < <(grep -l "Copyright.*Butler Labs" --include="*.go" -r . 2>/dev/null || true)

# Count files with Clastix copyright (excluding NOTICE)
clastix_copyright=0
while IFS= read -r line; do
    if [[ "$line" != *"NOTICE"* ]]; then
        ((clastix_copyright++))
    fi
done < <(grep -l "Copyright.*Clastix" --include="*.go" -r . 2>/dev/null || true)

if [[ $butler_copyright -gt 0 ]]; then
    log_pass "Found $butler_copyright files with Butler Labs copyright"
else
    log_warn "No Butler Labs copyright headers found"
fi

if [[ $clastix_copyright -gt 0 ]]; then
    log_fail "Found $clastix_copyright files still with Clastix copyright"
else
    log_pass "No Clastix copyright headers in code"
fi

# =============================================================================
# CHECK 18: Quick syntax check
# =============================================================================
log_check "Running Go syntax check..."

if command -v go &> /dev/null; then
    if go vet ./... 2>/dev/null; then
        log_pass "Go vet passed"
    else
        log_warn "Go vet reported issues (may need 'go mod tidy' first)"
    fi
else
    log_warn "Go not installed, skipping syntax check"
fi

# =============================================================================
# SUMMARY
# =============================================================================
echo ""
echo -e "${BLUE}==============================================================================${NC}"
echo -e "${BLUE}  VALIDATION SUMMARY${NC}"
echo -e "${BLUE}==============================================================================${NC}"
echo ""

if [[ $ERRORS -eq 0 && $WARNINGS -eq 0 ]]; then
    echo -e "${GREEN}✓ All checks passed!${NC}"
    echo ""
    echo "The rebrand appears to be complete. Next steps:"
    echo "  1. go mod tidy"
    echo "  2. make generate"
    echo "  3. make manifests"
    echo "  4. CGO_ENABLED=0 go build ./..."
    exit 0
elif [[ $ERRORS -eq 0 ]]; then
    echo -e "${YELLOW}⚠ Passed with $WARNINGS warning(s)${NC}"
    echo ""
    echo "Review warnings above, but the rebrand may be functional."
    exit 0
else
    echo -e "${RED}✗ Failed with $ERRORS error(s) and $WARNINGS warning(s)${NC}"
    echo ""
    echo "Please fix the errors above and re-run validation."
    exit 1
fi
