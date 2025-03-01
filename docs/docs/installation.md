---
sidebar_position: 2
hide_table_of_contents: true
---

# Installation

Cyphernetes can be installed in multiple ways depending on your operating system and preferences.

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
<TabItem value="homebrew" label="Homebrew" default>

The easiest way to install Cyphernetes on macOS or Linux is via Homebrew:

```bash
brew install cyphernetes
```

</TabItem>
<TabItem value="goGet" label="Using go get">
```bash
   go install github.com/avitaltamir/cyphernetes/cmd/cyphernetes@latest
```
</TabItem>
<TabItem value="binary" label="Binary Download">

You can download the pre-compiled binary for your operating system:

### Linux
```bash
# For AMD64
curl -LO https://github.com/avitaltamir/cyphernetes/releases/latest/download/cyphernetes-linux-amd64
chmod +x cyphernetes-linux-amd64
sudo mv cyphernetes-linux-amd64 /usr/local/bin/cyphernetes

# For ARM64
curl -LO https://github.com/avitaltamir/cyphernetes/releases/latest/download/cyphernetes-linux-arm64
chmod +x cyphernetes-linux-arm64
sudo mv cyphernetes-linux-arm64 /usr/local/bin/cyphernetes
```

### macOS
```bash
# For AMD64
curl -LO https://github.com/avitaltamir/cyphernetes/releases/latest/download/cyphernetes-darwin-amd64
chmod +x cyphernetes-darwin-amd64
sudo mv cyphernetes-darwin-amd64 /usr/local/bin/cyphernetes

# For ARM64 (Apple Silicon)
curl -LO https://github.com/avitaltamir/cyphernetes/releases/latest/download/cyphernetes-darwin-arm64
chmod +x cyphernetes-darwin-arm64
sudo mv cyphernetes-darwin-arm64 /usr/local/bin/cyphernetes
```

### Windows
Download the latest Windows binary from our [releases page](https://github.com/avitaltamir/cyphernetes/releases/latest).

</TabItem>
<TabItem value="source" label="Build from Source">

To build Cyphernetes from source, you'll need:

- Go (Latest)
- Make
- NodeJS (Latest)
- pnpm (9+)

```bash
# Clone the repository
git clone https://github.com/avitaltamir/cyphernetes.git

# Navigate to the project directory
cd cyphernetes

# Build the project
make

# The binary will be available in the dist/ directory
sudo mv dist/cyphernetes /usr/local/bin/cyphernetes
```

</TabItem>
</Tabs>

## Verifying the Installation

After installation, verify that Cyphernetes is working correctly:

```bash
cyphernetes --version
```

## Running Cyphernetes

There are multiple ways to run Cyphernetes:

1. **Web Interface**
   ```bash
   cyphernetes web
   ```
   Then visit `http://localhost:8080` in your browser.

2. **Interactive Shell**
   ```bash
   cyphernetes shell
   ```

3. **Single Query**
   ```bash
   cyphernetes query "MATCH (p:Pod) RETURN p"
   ``` 