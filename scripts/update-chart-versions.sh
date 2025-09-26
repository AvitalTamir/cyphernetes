#!/bin/bash

# Script to update Helm chart versions based on git tags
# Usage: ./scripts/update-chart-versions.sh [version]
# If no version is provided, it will use the latest git tag

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Function to get the latest git tag or use provided version
get_version() {
    if [ -n "$1" ]; then
        echo "$1"
    else
        # Get the latest git tag from origin, removing the 'v' prefix if present
        git tag -l --sort=-v:refname | head -1 | sed 's/^v//' || echo "0.1.0"
    fi
}

# Function to update Chart.yaml version and appVersion
update_chart_yaml() {
    local chart_file="$1"
    local version="$2"
    
    if [ ! -f "$chart_file" ]; then
        echo "Warning: Chart file $chart_file not found"
        return 1
    fi
    
    echo "Updating $chart_file with version $version"
    
    # Update version field
    sed -i "s/^version: .*/version: $version/" "$chart_file"
    
    # Update appVersion field
    sed -i "s/^appVersion: .*/appVersion: \"$version\"/" "$chart_file"
    
    # If this is the main chart, also update the CRD dependency version
    if [[ "$chart_file" == *"cyphernetes-operator/Chart.yaml" ]]; then
        # Update the CRD dependency version
        sed -i "/name: crds/,/condition: installCRDs/ s/version: .*/version: \"$version\"/" "$chart_file"
    fi
}

# Function to update values.yaml image tag
update_values_yaml() {
    local values_file="$1"
    local version="$2"
    
    if [ ! -f "$values_file" ]; then
        echo "Warning: Values file $values_file not found"
        return 1
    fi
    
    echo "Updating $values_file with image tag v$version"
    
    # Update the image tag to the version (handles both "latest" and existing version tags)
    sed -i "s/tag: \".*\"/tag: \"v$version\"/" "$values_file"
}

# Main script
main() {
    VERSION=$(get_version "$1")
    
    echo "Updating Helm charts to version: $VERSION"
    
    # Update main operator chart
    OPERATOR_CHART="$REPO_ROOT/operator/helm/cyphernetes-operator/Chart.yaml"
    update_chart_yaml "$OPERATOR_CHART" "$VERSION"
    
    # Update CRD subchart  
    CRD_CHART="$REPO_ROOT/operator/helm/cyphernetes-operator/charts/crds/Chart.yaml"
    update_chart_yaml "$CRD_CHART" "$VERSION"
    
    # Update values.yaml image tag
    VALUES_FILE="$REPO_ROOT/operator/helm/cyphernetes-operator/values.yaml"
    update_values_yaml "$VALUES_FILE" "$VERSION"
    
    echo "Chart version update completed successfully!"
    echo "Updated files:"
    echo "  - $OPERATOR_CHART"
    echo "  - $CRD_CHART" 
    echo "  - $VALUES_FILE"
}

# Run main function with all arguments
main "$@"