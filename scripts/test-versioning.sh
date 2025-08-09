#!/bin/bash

# Test script for update-chart-versions.sh
# This script validates that the versioning script works correctly

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Test version to use
TEST_VERSION="0.99.9"

echo "Testing chart versioning script..."

# Save original files
cp "$REPO_ROOT/operator/helm/cyphernetes-operator/Chart.yaml" "$REPO_ROOT/operator/helm/cyphernetes-operator/Chart.yaml.backup"
cp "$REPO_ROOT/operator/helm/cyphernetes-operator/charts/crds/Chart.yaml" "$REPO_ROOT/operator/helm/cyphernetes-operator/charts/crds/Chart.yaml.backup"
cp "$REPO_ROOT/operator/helm/cyphernetes-operator/values.yaml" "$REPO_ROOT/operator/helm/cyphernetes-operator/values.yaml.backup"

# Function to restore original files
restore_files() {
    mv "$REPO_ROOT/operator/helm/cyphernetes-operator/Chart.yaml.backup" "$REPO_ROOT/operator/helm/cyphernetes-operator/Chart.yaml"
    mv "$REPO_ROOT/operator/helm/cyphernetes-operator/charts/crds/Chart.yaml.backup" "$REPO_ROOT/operator/helm/cyphernetes-operator/charts/crds/Chart.yaml"
    mv "$REPO_ROOT/operator/helm/cyphernetes-operator/values.yaml.backup" "$REPO_ROOT/operator/helm/cyphernetes-operator/values.yaml"
}

# Set up cleanup on exit
trap restore_files EXIT

# Test the script
echo "Running versioning script with test version $TEST_VERSION..."
"$REPO_ROOT/scripts/update-chart-versions.sh" "$TEST_VERSION"

# Verify changes
echo "Verifying changes..."

# Check main chart version
MAIN_VERSION=$(grep "^version:" "$REPO_ROOT/operator/helm/cyphernetes-operator/Chart.yaml" | cut -d' ' -f2)
MAIN_APP_VERSION=$(grep "^appVersion:" "$REPO_ROOT/operator/helm/cyphernetes-operator/Chart.yaml" | cut -d' ' -f2 | tr -d '"')
CRD_DEP_VERSION=$(grep -A3 "name: crds" "$REPO_ROOT/operator/helm/cyphernetes-operator/Chart.yaml" | grep "version:" | cut -d' ' -f6 | tr -d '"')

# Check CRD chart version
CRD_VERSION=$(grep "^version:" "$REPO_ROOT/operator/helm/cyphernetes-operator/charts/crds/Chart.yaml" | cut -d' ' -f2)
CRD_APP_VERSION=$(grep "^appVersion:" "$REPO_ROOT/operator/helm/cyphernetes-operator/charts/crds/Chart.yaml" | cut -d' ' -f2 | tr -d '"')

# Check values.yaml image tag
IMAGE_TAG=$(grep "tag:" "$REPO_ROOT/operator/helm/cyphernetes-operator/values.yaml" | cut -d' ' -f4 | tr -d '"')

# Validate results
ERRORS=0

if [ "$MAIN_VERSION" != "$TEST_VERSION" ]; then
    echo "ERROR: Main chart version is $MAIN_VERSION, expected $TEST_VERSION"
    ERRORS=$((ERRORS + 1))
fi

if [ "$MAIN_APP_VERSION" != "$TEST_VERSION" ]; then
    echo "ERROR: Main chart appVersion is $MAIN_APP_VERSION, expected $TEST_VERSION"
    ERRORS=$((ERRORS + 1))
fi

if [ "$CRD_DEP_VERSION" != "$TEST_VERSION" ]; then
    echo "ERROR: CRD dependency version is $CRD_DEP_VERSION, expected $TEST_VERSION"
    ERRORS=$((ERRORS + 1))
fi

if [ "$CRD_VERSION" != "$TEST_VERSION" ]; then
    echo "ERROR: CRD chart version is $CRD_VERSION, expected $TEST_VERSION"
    ERRORS=$((ERRORS + 1))
fi

if [ "$CRD_APP_VERSION" != "$TEST_VERSION" ]; then
    echo "ERROR: CRD chart appVersion is $CRD_APP_VERSION, expected $TEST_VERSION"
    ERRORS=$((ERRORS + 1))
fi

if [ "$IMAGE_TAG" != "v$TEST_VERSION" ]; then
    echo "ERROR: Image tag is $IMAGE_TAG, expected v$TEST_VERSION"
    ERRORS=$((ERRORS + 1))
fi

if [ $ERRORS -eq 0 ]; then
    echo "✅ All tests passed! Versioning script works correctly."
    
    # Test Helm packaging
    echo "Testing Helm chart packaging..."
    cd "$REPO_ROOT/operator"
    if helm package helm/cyphernetes-operator --destination /tmp > /dev/null; then
        echo "✅ Helm chart packaging successful with version $TEST_VERSION"
        rm -f "/tmp/cyphernetes-operator-$TEST_VERSION.tgz"
    else
        echo "❌ Helm chart packaging failed"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "❌ $ERRORS test(s) failed!"
fi

exit $ERRORS