# Cyphernetes Operator Versioning

This directory contains the versioning system for the Cyphernetes Operator Helm charts.

## Overview

The Cyphernetes Operator uses semantic versioning that automatically syncs with git tags. When a new release is created with a git tag (e.g., `v0.19.0`), the Helm charts are automatically updated to use the corresponding versions.

## Scripts

### `update-chart-versions.sh`

This script updates the Helm chart versions based on git tags or a provided version.

**Usage:**
```bash
# Update to a specific version
./scripts/update-chart-versions.sh 0.19.0

# Update to the latest git tag (automatic detection)
./scripts/update-chart-versions.sh
```

**What it updates:**
- `operator/helm/cyphernetes-operator/Chart.yaml` - Main chart version and appVersion
- `operator/helm/cyphernetes-operator/charts/crds/Chart.yaml` - CRD subchart version and appVersion  
- `operator/helm/cyphernetes-operator/values.yaml` - Docker image tag

### `test-versioning.sh`

This script tests that the versioning system works correctly by:
1. Running the update script with a test version
2. Verifying all files are updated correctly
3. Testing that Helm chart packaging still works
4. Restoring original files

**Usage:**
```bash
./scripts/test-versioning.sh
```

## Automated Release Process

The versioning system is integrated into the GitHub Actions release workflow (`.github/workflows/release.yml`):

1. When a new tag is pushed (e.g., `v0.19.0`), the release workflow is triggered
2. The workflow extracts the version from the git tag
3. The `update-chart-versions.sh` script is called to update all chart versions
4. The Docker image is built and tagged with the version
5. The Helm chart is packaged with the correct version
6. Everything is published to registries and Artifact Hub

## Version Format

- **Git tags**: `v{major}.{minor}.{patch}` (e.g., `v0.19.0`)
- **Chart versions**: `{major}.{minor}.{patch}` (e.g., `0.19.0`) 
- **Docker image tags**: `v{major}.{minor}.{patch}` (e.g., `v0.19.0`)

## Manual Usage

To manually update versions during development:

```bash
# Update to a specific version
./scripts/update-chart-versions.sh 0.19.0-dev

# Test the changes
./scripts/test-versioning.sh

# Package the chart
cd operator
helm package helm/cyphernetes-operator
```

## Files Modified

The versioning system updates these files:

1. **Main Chart** (`operator/helm/cyphernetes-operator/Chart.yaml`):
   - `version` field - Chart version
   - `appVersion` field - Application version
   - CRD dependency version in `dependencies` section

2. **CRD Chart** (`operator/helm/cyphernetes-operator/charts/crds/Chart.yaml`):
   - `version` field - Chart version
   - `appVersion` field - Application version

3. **Values** (`operator/helm/cyphernetes-operator/values.yaml`):
   - `image.tag` field - Docker image tag

This ensures that all components use consistent versioning and that Artifact Hub displays the correct version information.