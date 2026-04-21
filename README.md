# lwt-cloudflared

A customized build of [cloudflare/cloudflared](https://github.com/cloudflare/cloudflared) with **code obfuscation** and **additional features**.

## Features

### 🔒 Code Obfuscation (via [garble](https://github.com/burrowers/garble))
- **Symbol obfuscation**: All function names, variable names, and type names are randomized
- **String literal obfuscation** (`-literals`): All string constants in the binary are encrypted at compile time and decrypted at runtime
- **Runtime info removal** (`-tiny`): Debug and runtime type information stripped
- **Random seed** (`-seed=random`): Each build produces a uniquely obfuscated binary
- **Stripped symbols** (`-ldflags="-s -w"`): Debug symbols and DWARF info removed

### 🌐 Bilingual Help (Chinese / English)
- Automatically detects system language via `LANG`, `LC_ALL`, `LANGUAGE` environment variables
- Displays Chinese help text when the system locale is `zh_*`
- Falls back to English for all other locales
- Covers: top-level commands, `tunnel` subcommands, `access` subcommands

### 🎭 Command Alias
- **`tun`** → alias for **`tunnel`**
  - Use `tun` instead of `tunnel` so that `ps aux` output won't reveal you are running a "tunnel" program
  - Example: `./cfd-linux-amd64-obfuscated tun run --token <TOKEN>`

## Build Requirements

- **Go** >= 1.26.2
- **garble** v0.16.0 (`go install mvdan.cc/garble@v0.16.0`)
- **Linux amd64** (for cross-compilation, set `GOOS=linux GOARCH=amd64`)
- **RAM**: At least 8GB recommended (the `-literals` flag significantly increases memory usage during compilation)

## Build Instructions

### 1. Install Go 1.26.2+

```bash
# Download and install Go
wget https://go.dev/dl/go1.26.2.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.26.2.linux-amd64.tar.gz
export PATH=/usr/local/go/bin:$PATH

# If using GOTOOLCHAIN=local, copy Go to a dedicated directory
cp -r /usr/local/go /usr/local/go126
```

### 2. Install garble

```bash
export GOPROXY=https://goproxy.cn,direct  # Optional: use Chinese mirror
go install mvdan.cc/garble@v0.16.0
```

### 3. Clone and Build

```bash
# Clone this repository
git clone https://github.com/lwtdzh/lwt-cloudflared.git
cd lwt-cloudflared

# Build with full obfuscation
GOPROXY=https://goproxy.cn,direct \
GOTOOLCHAIN=local \
GOROOT=/usr/local/go126 \
garble -literals -tiny -seed=random build \
  -o ~/cfd-linux-amd64-obfuscated \
  -ldflags="-s -w" \
  ./cmd/cloudflared
```

> **Note**: The build with `-literals` takes approximately **45 minutes** and uses ~4GB RAM. Without `-literals`, it takes ~5 minutes.

### 4. Verify

```bash
# Check file type
file ~/cfd-linux-amd64-obfuscated
# Expected: ELF 64-bit LSB executable, x86-64, ... stripped

# Test English help
LANG=en_US.UTF-8 ./cfd-linux-amd64-obfuscated --help

# Test Chinese help
LANG=zh_CN.UTF-8 ./cfd-linux-amd64-obfuscated --help

# Test tunnel alias
./cfd-linux-amd64-obfuscated tun --help

# Verify obfuscation (should return very few matches)
strings ~/cfd-linux-amd64-obfuscated | grep -ic "cloudflare"
```

## Quick Usage

### Temporary Tunnel (No Account Required)

```bash
# Expose a local service on port 8080
./cfd-linux-amd64-obfuscated tun --url http://localhost:8080
```

This generates a random `*.trycloudflare.com` URL.

### Named Tunnel (With Cloudflare Account)

```bash
./cfd-linux-amd64-obfuscated tun login
./cfd-linux-amd64-obfuscated tun create my-tunnel
./cfd-linux-amd64-obfuscated tun route dns my-tunnel my-tunnel.example.com
./cfd-linux-amd64-obfuscated tun run --url http://localhost:8080 my-tunnel
```

### Run with Token (From Dashboard)

```bash
./cfd-linux-amd64-obfuscated tun run --token <YOUR_TOKEN>
```

## Obfuscation Comparison

| Metric | Original cloudflared | This build |
|--------|---------------------|------------|
| `strings \| grep -ic cloudflare` | 100+ matches | **~4 matches** |
| Function/variable names | Readable | **Randomized** |
| String literals | Plain text | **Encrypted** |
| Debug symbols | Present | **Stripped** |
| Binary size | ~25MB | **~80MB** |

## License

This project is based on [cloudflare/cloudflared](https://github.com/cloudflare/cloudflared), licensed under the Apache License 2.0.
