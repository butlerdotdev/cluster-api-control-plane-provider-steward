#!/bin/bash
# =============================================================================
# CAPI Provider Rebrand Script: Kamaji → Steward
# =============================================================================
# Usage: ./rebrand.sh [--dry-run]
#
# Run this script from the root of the cloned cluster-api-control-plane-provider-kamaji repo
# =============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
    DRY_RUN=true
    echo -e "${YELLOW}=== DRY RUN MODE - No changes will be made ===${NC}"
fi

# Helper function for sed (handles macOS vs Linux)
do_sed() {
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  [DRY-RUN] sed -i '$1' $2"
        return
    fi
    
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "$1" "$2"
    else
        sed -i "$1" "$2"
    fi
}

# Helper function for find+sed
do_find_sed() {
    local pattern="$1"
    local file_pattern="$2"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  [DRY-RUN] find . -type f -name '$file_pattern' -exec sed -i '$pattern' {} +"
        return
    fi
    
    if [[ "$OSTYPE" == "darwin"* ]]; then
        find . -type f -name "$file_pattern" -exec sed -i '' "$pattern" {} +
    else
        find . -type f -name "$file_pattern" -exec sed -i "$pattern" {} +
    fi
}

# Helper for multi-pattern find+sed
do_find_sed_multi() {
    local pattern="$1"
    shift
    local file_patterns=("$@")
    
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  [DRY-RUN] find . -type f \\( ${file_patterns[*]} \\) -exec sed -i '$pattern' {} +"
        return
    fi
    
    local find_args=()
    for p in "${file_patterns[@]}"; do
        if [[ ${#find_args[@]} -gt 0 ]]; then
            find_args+=("-o")
        fi
        find_args+=("-name" "$p")
    done
    
    if [[ "$OSTYPE" == "darwin"* ]]; then
        find . -type f \( "${find_args[@]}" \) -exec sed -i '' "$pattern" {} +
    else
        find . -type f \( "${find_args[@]}" \) -exec sed -i "$pattern" {} +
    fi
}

echo -e "${BLUE}==============================================================================${NC}"
echo -e "${BLUE}  CAPI Control Plane Provider Rebrand: Kamaji → Steward${NC}"
echo -e "${BLUE}==============================================================================${NC}"
echo ""

# Verify we're in the right directory
if [[ ! -f "go.mod" ]]; then
    echo -e "${RED}ERROR: go.mod not found. Run this script from the repository root.${NC}"
    exit 1
fi

# Check for either old or new module path (to handle re-runs)
if grep -q "cluster-api-control-plane-provider-kamaji" go.mod; then
    echo -e "${GREEN}✓ Repository verified (original Kamaji provider)${NC}"
elif grep -q "cluster-api-control-plane-provider-steward" go.mod; then
    echo -e "${YELLOW}⚠ Repository already partially/fully rebranded to Steward${NC}"
    echo -e "${YELLOW}  Continuing with rebrand (safe to re-run)...${NC}"
else
    echo -e "${RED}ERROR: This doesn't appear to be the CAPI Kamaji/Steward provider repository.${NC}"
    exit 1
fi
echo ""

# =============================================================================
# PHASE 1: Go Module & Import Path Changes
# =============================================================================
echo -e "${YELLOW}PHASE 1: Module & Import Path Changes${NC}"

echo "  1.1 Updating module path in go.mod..."
do_sed 's|module github.com/clastix/cluster-api-control-plane-provider-kamaji|module github.com/butlerdotdev/cluster-api-control-plane-provider-steward|g' go.mod

echo "  1.2 Updating CAPI provider import paths in Go files..."
do_find_sed 's|github.com/clastix/cluster-api-control-plane-provider-kamaji|github.com/butlerdotdev/cluster-api-control-plane-provider-steward|g' "*.go"

echo "  1.3 Updating Kamaji dependency to Steward..."
do_find_sed_multi 's|github.com/clastix/kamaji|github.com/butlerdotdev/steward|g' "*.go" "go.mod"

echo -e "${GREEN}✓ Phase 1 complete${NC}"
echo ""

# =============================================================================
# PHASE 2: Import Alias Changes
# =============================================================================
echo -e "${YELLOW}PHASE 2: Import Alias Changes${NC}"

echo "  2.1 Renaming kamajiv1alpha1 → stewardv1alpha1..."
do_find_sed 's|kamajiv1alpha1|stewardv1alpha1|g' "*.go"

echo "  2.2 Renaming kcpv1alpha1 → scpv1alpha1..."
do_find_sed 's|kcpv1alpha1|scpv1alpha1|g' "*.go"

echo -e "${GREEN}✓ Phase 2 complete${NC}"
echo ""

# =============================================================================
# PHASE 3: Type Name Changes
# =============================================================================
echo -e "${YELLOW}PHASE 3: Type Name Changes${NC}"

echo "  3.1 Renaming KamajiControlPlane → StewardControlPlane..."
do_find_sed 's|KamajiControlPlane|StewardControlPlane|g' "*.go"

echo "  3.2 Renaming kamajicontrolplane → stewardcontrolplane (lowercase)..."
do_find_sed 's|kamajicontrolplane|stewardcontrolplane|g' "*.go"

echo -e "${GREEN}✓ Phase 3 complete${NC}"
echo ""

# =============================================================================
# PHASE 4: Label & Annotation Changes
# =============================================================================
echo -e "${YELLOW}PHASE 4: Label & Annotation Changes${NC}"

echo "  4.1 Updating API group kamaji.clastix.io → steward.butlerlabs.dev..."
do_find_sed_multi 's|kamaji\.clastix\.io|steward.butlerlabs.dev|g' "*.go" "*.yaml" "*.yml"

echo "  4.2 Updating webhook paths kamaji-clastix-io → steward-butlerlabs-dev..."
do_find_sed 's|kamaji-clastix-io|steward-butlerlabs-dev|g' "*.go"

echo "  4.3 Updating leader election ID..."
do_find_sed 's|kamaji\.controlplane\.cluster\.x-k8s\.io|steward.controlplane.cluster.x-k8s.io|g' "*.go"

echo -e "${GREEN}✓ Phase 4 complete${NC}"
echo ""

# =============================================================================
# PHASE 5: Variable Name Changes
# =============================================================================
echo -e "${YELLOW}PHASE 5: Variable Name Changes${NC}"

echo "  5.1 Renaming kcp variables → scp..."
do_find_sed 's|\bkcp\b|scp|g' "*.go"
do_find_sed 's|&kcp|&scp|g' "*.go"

echo "  5.2 Renaming kamajiCA → stewardCA..."
do_find_sed 's|kamajiCA|stewardCA|g' "*.go"

echo "  5.3 Renaming kamajiAdminKubeconfig → stewardAdminKubeconfig..."
do_find_sed 's|kamajiAdminKubeconfig|stewardAdminKubeconfig|g' "*.go"

echo -e "${GREEN}✓ Phase 5 complete${NC}"
echo ""

# =============================================================================
# PHASE 6: Kubebuilder Marker Changes
# =============================================================================
echo -e "${YELLOW}PHASE 6: Kubebuilder Marker Changes${NC}"

echo "  6.1 Updating shortName markers..."
do_find_sed 's|shortName=ktcp|shortName=stcp|g' "*.go"
do_find_sed 's|shortName=ktcpt|shortName=stcpt|g' "*.go"

echo "  6.2 Updating categories..."
do_find_sed 's|categories=cluster-api;kamaji|categories=cluster-api;steward|g' "*.go"

echo -e "${GREEN}✓ Phase 6 complete${NC}"
echo ""

# =============================================================================
# PHASE 7: Copyright & Comment Changes
# =============================================================================
echo -e "${YELLOW}PHASE 7: Copyright & Comment Changes${NC}"

echo "  7.1 Updating copyright headers..."
do_find_sed_multi 's|Copyright 2023 Clastix Labs|Copyright 2025 Butler Labs|g' "*.go" "*.yaml" "*.yml"
do_find_sed_multi 's|Copyright 2024 Clastix Labs|Copyright 2025 Butler Labs|g' "*.go" "*.yaml" "*.yml"

echo "  7.2 Updating Clastix references..."
do_find_sed_multi 's|Clastix Labs|Butler Labs|g' "*.go" "*.yaml" "*.md"
do_find_sed_multi 's|clastix|butlerlabs|g' "*.go" "*.yaml"

echo "  7.3 Updating Kamaji references in comments..."
do_find_sed_multi 's|Kamaji|Steward|g' "*.go" "*.yaml" "*.md"

echo "  7.4 Updating remaining lowercase kamaji..."
do_find_sed_multi 's|kamaji|steward|g' "*.go" "*.yaml"

echo -e "${GREEN}✓ Phase 7 complete${NC}"
echo ""

# =============================================================================
# PHASE 8: Function/Constant Name Changes in pkg/
# =============================================================================
echo -e "${YELLOW}PHASE 8: pkg/ Function & Constant Renames${NC}"

echo "  8.1 Renaming GenerateKeyNameFromKamaji → GenerateKeyNameFromSteward..."
do_find_sed 's|GenerateKeyNameFromKamaji|GenerateKeyNameFromSteward|g' "*.go"

echo "  8.2 Renaming ParseKamajiControlPlaneUID... → ParseStewardControlPlaneUID......"
do_find_sed 's|ParseKamajiControlPlaneUIDFromTenantControlPlane|ParseStewardControlPlaneUIDFromTenantControlPlane|g' "*.go"

echo "  8.3 Renaming KamajiControlPlaneUIDAnnotation → StewardControlPlaneUIDAnnotation..."
do_find_sed 's|KamajiControlPlaneUIDAnnotation|StewardControlPlaneUIDAnnotation|g' "*.go"

echo "  8.4 Renaming indexer constants..."
do_find_sed 's|ExternalClusterReferenceKamajiControlPlaneField|ExternalClusterReferenceStewardControlPlaneField|g' "*.go"
do_find_sed 's|KamajiControlPlaneUIDField|StewardControlPlaneUIDField|g' "*.go"

echo "  8.5 Updating annotation key kcp-uid → scp-uid..."
do_find_sed 's|/kcp-uid|/scp-uid|g' "*.go"

echo -e "${GREEN}✓ Phase 8 complete${NC}"
echo ""

# =============================================================================
# PHASE 9: Update Steward Dependency Version
# =============================================================================
echo -e "${YELLOW}PHASE 9: Update Steward Dependency Version${NC}"

echo "  9.1 Setting Steward version to v0.1.0-alpha..."
if [[ "$DRY_RUN" == "false" ]]; then
    # Remove any existing version and set to v0.1.0-alpha (the published tag)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' 's|github.com/butlerdotdev/steward v[^ ]*|github.com/butlerdotdev/steward v0.1.0-alpha|g' go.mod
    else
        sed -i 's|github.com/butlerdotdev/steward v[^ ]*|github.com/butlerdotdev/steward v0.1.0-alpha|g' go.mod
    fi
else
    echo "  [DRY-RUN] sed -i 's|github.com/butlerdotdev/steward v[^ ]*|github.com/butlerdotdev/steward v0.1.0-alpha|g' go.mod"
fi

echo -e "${GREEN}✓ Phase 9 complete${NC}"
echo ""

# =============================================================================
# PHASE 10: File Renames
# =============================================================================
echo -e "${YELLOW}PHASE 10: File Renames${NC}"

if [[ "$DRY_RUN" == "true" ]]; then
    echo "  [DRY-RUN] Would rename files containing 'kamaji' to 'steward'"
else
    # Enable nullglob so unmatched patterns expand to nothing
    shopt -s nullglob
    
    # Rename API files
    for f in api/v1alpha1/kamajicontrolplane*.go; do
        newname=$(echo "$f" | sed 's/kamajicontrolplane/stewardcontrolplane/g')
        if [[ ! -f "$newname" ]]; then
            echo "  Renaming: $f → $newname"
            mv "$f" "$newname"
        else
            echo "  Skipping: $newname already exists"
        fi
    done

    # Rename controller files
    for f in controllers/kamajicontrolplane*.go; do
        newname=$(echo "$f" | sed 's/kamajicontrolplane/stewardcontrolplane/g')
        if [[ ! -f "$newname" ]]; then
            echo "  Renaming: $f → $newname"
            mv "$f" "$newname"
        else
            echo "  Skipping: $newname already exists"
        fi
    done

    # Rename pkg/indexers files if they exist
    if [[ -f "pkg/indexers/kamaji_uid.go" && ! -f "pkg/indexers/steward_uid.go" ]]; then
        echo "  Renaming: pkg/indexers/kamaji_uid.go → pkg/indexers/steward_uid.go"
        mv "pkg/indexers/kamaji_uid.go" "pkg/indexers/steward_uid.go"
    fi
    
    if [[ -f "pkg/indexers/ecr_kcp.go" && ! -f "pkg/indexers/ecr_scp.go" ]]; then
        echo "  Renaming: pkg/indexers/ecr_kcp.go → pkg/indexers/ecr_scp.go"
        mv "pkg/indexers/ecr_kcp.go" "pkg/indexers/ecr_scp.go"
    fi

    # Rename CRD patch files
    for f in config/crd/patches/*kamaji*.yaml; do
        newname=$(echo "$f" | sed 's/kamajicontrolplane/stewardcontrolplane/g')
        if [[ ! -f "$newname" ]]; then
            echo "  Renaming: $f → $newname"
            mv "$f" "$newname"
        else
            echo "  Skipping: $newname already exists"
        fi
    done

    # Rename RBAC files
    for f in config/rbac/*kamaji*.yaml; do
        newname=$(echo "$f" | sed 's/kamajicontrolplane/stewardcontrolplane/g')
        if [[ ! -f "$newname" ]]; then
            echo "  Renaming: $f → $newname"
            mv "$f" "$newname"
        else
            echo "  Skipping: $newname already exists"
        fi
    done

    # Rename sample files
    for f in config/samples/*kamaji*.yaml; do
        newname=$(echo "$f" | sed 's/kamajicontrolplane/stewardcontrolplane/g' | sed 's/kamaji/steward/g')
        if [[ ! -f "$newname" ]]; then
            echo "  Renaming: $f → $newname"
            mv "$f" "$newname"
        else
            echo "  Skipping: $newname already exists"
        fi
    done

    # Rename CRD base files
    for f in config/crd/bases/*kamajicontrolplane*.yaml; do
        newname=$(echo "$f" | sed 's/kamajicontrolplane/stewardcontrolplane/g')
        if [[ ! -f "$newname" ]]; then
            echo "  Renaming: $f → $newname"
            mv "$f" "$newname"
        else
            echo "  Skipping: $newname already exists"
        fi
    done

    # Rename template files (using find to handle nested directories)
    while IFS= read -r -d '' f; do
        newname=$(echo "$f" | sed 's/kamaji/steward/g')
        if [[ ! -f "$newname" ]]; then
            echo "  Renaming: $f → $newname"
            mv "$f" "$newname"
        else
            echo "  Skipping: $newname already exists"
        fi
    done < <(find templates -name "*kamaji*.yaml" -print0 2>/dev/null || true)
    
    # Disable nullglob
    shopt -u nullglob
fi

echo -e "${GREEN}✓ Phase 10 complete${NC}"
echo ""

# =============================================================================
# PHASE 11: Update kustomization.yaml files
# =============================================================================
echo -e "${YELLOW}PHASE 11: Update kustomization.yaml References${NC}"

echo "  11.1 Updating kustomization.yaml files..."
do_find_sed 's|kamajicontrolplane|stewardcontrolplane|g' "kustomization.yaml"
do_find_sed 's|kamaji|steward|g' "kustomization.yaml"

echo -e "${GREEN}✓ Phase 11 complete${NC}"
echo ""

# =============================================================================
# PHASE 12: Cleanup any double-replacements
# =============================================================================
echo -e "${YELLOW}PHASE 12: Cleanup Double-Replacements${NC}"

echo "  12.1 Fixing any scpscp → scp..."
do_find_sed 's|scpscp|scp|g' "*.go"

echo "  12.2 Fixing any kcpkcp → kcp..."
do_find_sed 's|kcpkcp|kcp|g' "*.go"

echo "  12.3 Fixing any stewardsteward → steward..."
do_find_sed_multi 's|stewardsteward|steward|g' "*.go" "*.yaml"
do_find_sed_multi 's|StewardSteward|Steward|g' "*.go" "*.yaml"

echo -e "${GREEN}✓ Phase 12 complete${NC}"
echo ""

# =============================================================================
# PHASE 13: Update Dockerfile
# =============================================================================
echo -e "${YELLOW}PHASE 13: Update Dockerfile${NC}"

if [[ -f "Dockerfile" ]]; then
    echo "  13.1 Updating image references..."
    do_sed 's|clastix/cluster-api-control-plane-provider-kamaji|ghcr.io/butlerdotdev/capi-steward|g' Dockerfile
fi

echo -e "${GREEN}✓ Phase 13 complete${NC}"
echo ""

# =============================================================================
# PHASE 14: Update Makefile
# =============================================================================
echo -e "${YELLOW}PHASE 14: Update Makefile${NC}"

if [[ -f "Makefile" ]]; then
    echo "  14.1 Updating image repository..."
    do_sed 's|clastix/cluster-api-control-plane-provider-kamaji|ghcr.io/butlerdotdev/capi-steward|g' Makefile
    do_sed 's|kamaji|steward|g' Makefile
fi

echo -e "${GREEN}✓ Phase 14 complete${NC}"
echo ""

# =============================================================================
# COMPLETE
# =============================================================================
echo -e "${BLUE}==============================================================================${NC}"
echo -e "${GREEN}  REBRAND COMPLETE!${NC}"
echo -e "${BLUE}==============================================================================${NC}"
echo ""
echo "Next steps:"
echo "  1. Run ./validate.sh to verify the rebrand"
echo "  2. Run 'go mod tidy' to update dependencies"
echo "  3. Run 'make generate' to regenerate deepcopy"
echo "  4. Run 'make manifests' to regenerate CRDs"
echo "  5. Run 'CGO_ENABLED=0 go build ./...' to verify compilation"
echo ""
