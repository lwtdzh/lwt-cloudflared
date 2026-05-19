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

### ⚡ Quick Temporary Tunnel (`temp` command)
- One-line shortcut to create a temporary tunnel with simple syntax
- Format: `./cfd-linux-amd64-obfuscated temp <addr> <protocol> <port>`
- **`local`** is a special alias for **`localhost`**
- Examples:
  ```bash
  # Expose local SSH (tcp://localhost:22)
  ./cfd-linux-amd64-obfuscated temp local tcp 22

  # Expose local HTTP server (http://localhost:8080)
  ./cfd-linux-amd64-obfuscated temp local http 8080

  # Expose a remote service (https://1.2.3.4:443)
  ./cfd-linux-amd64-obfuscated temp 1.2.3.4 https 443
  ```

### 🔒 Outbound Proxy Support (`--proxy` / `-x`)
- Route all cloudflared-to-Cloudflare-Edge connections through a SOCKS5 or HTTP proxy
- **Requires `--protocol http2`** (SOCKS5/HTTP proxies only support TCP, not UDP/QUIC)
- Format: `socks5://host:port`, `socks5://user:password@host:port`, or `http://host:port`
- Examples:
  ```bash
  # Use SOCKS5 proxy
  ./cfd-linux-amd64-obfuscated temp local http 8080 --proxy socks5://127.0.0.1:1080 --protocol http2

  # Use SOCKS5 proxy with authentication
  ./cfd-linux-amd64-obfuscated temp local tcp 22 --proxy socks5://user:pass@127.0.0.1:1080 --protocol http2

  # Use HTTP proxy
  ./cfd-linux-amd64-obfuscated temp local http 8080 --proxy http://proxy.example.com:8080 --protocol http2

  # With named tunnel
  ./cfd-linux-amd64-obfuscated tunnel run --token <TOKEN> --proxy socks5://127.0.0.1:1080 --protocol http2
  ```
- **Error handling**: If `--proxy` is used without `--protocol http2`, the program will exit with an error message

## Build Requirements

- **Go** >= 1.26.2
- **garble** v0.16.0 (`go install mvdan.cc/garble@v0.16.0`)
- **Linux amd64** (for cross-compilation, set `GOOS=linux GOARCH=amd64`)
- **RAM**: At least 8GB recommended (the `-literals` flag significantly increases memory usage during compilation)

## Build Instructions

### 1. Install Go 1.26.2+

Download Go from [https://go.dev/dl/](https://go.dev/dl/) for your platform:

| Platform | Download |
|----------|----------|
| **Linux amd64** | `go1.26.2.linux-amd64.tar.gz` |
| **Linux arm64** | `go1.26.2.linux-arm64.tar.gz` |
| **macOS Intel** | `go1.26.2.darwin-amd64.tar.gz` |
| **macOS Apple Silicon** | `go1.26.2.darwin-arm64.tar.gz` |
| **Windows amd64** | `go1.26.2.windows-amd64.zip` |

```bash
# Linux / macOS example:
wget https://go.dev/dl/go1.26.2.<OS>-<ARCH>.tar.gz
tar -C /usr/local -xzf go1.26.2.<OS>-<ARCH>.tar.gz
export PATH=/usr/local/go/bin:$PATH

# macOS via Homebrew:
brew install go
```

### 2. Install garble

```bash
export GOPROXY=https://goproxy.cn,direct  # Optional: use Chinese mirror for faster download
go install mvdan.cc/garble@v0.16.0
```

### 3. Clone and Build

```bash
git clone https://github.com/lwtdzh/lwt-cloudflared.git
cd lwt-cloudflared
```

#### Build for current platform (native)

```bash
CGO_ENABLED=0 garble -literals -tiny -seed=random build \
  -o cfd-obfuscated \
  -ldflags="-s -w" \
  ./cmd/cloudflared
```

#### Cross-compile for other platforms

Set `GOOS` and `GOARCH` to target any platform:

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 garble -literals -tiny -seed=random build \
  -o cfd-linux-amd64 -ldflags="-s -w" ./cmd/cloudflared

# Linux arm64
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 garble -literals -tiny -seed=random build \
  -o cfd-linux-arm64 -ldflags="-s -w" ./cmd/cloudflared

# macOS Intel
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 garble -literals -tiny -seed=random build \
  -o cfd-darwin-amd64 -ldflags="-s -w" ./cmd/cloudflared

# macOS Apple Silicon (M1/M2/M3/M4)
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 garble -literals -tiny -seed=random build \
  -o cfd-darwin-arm64 -ldflags="-s -w" ./cmd/cloudflared

# Windows amd64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 garble -literals -tiny -seed=random build \
  -o cfd-windows-amd64.exe -ldflags="-s -w" ./cmd/cloudflared
```

#### Supported `GOOS`/`GOARCH` combinations

| GOOS | GOARCH | Output |
|------|--------|--------|
| `linux` | `amd64` | ELF x86-64 binary |
| `linux` | `arm64` | ELF ARM64 binary |
| `darwin` | `amd64` | Mach-O x86-64 binary (Intel Mac) |
| `darwin` | `arm64` | Mach-O ARM64 binary (Apple Silicon) |
| `windows` | `amd64` | PE32+ x86-64 `.exe` |

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

### ⚡ Fastest Way: `temp` Command

```bash
# Expose local SSH
./cfd-linux-amd64-obfuscated temp local tcp 22

# Expose local HTTP server
./cfd-linux-amd64-obfuscated temp local http 8080

# Expose a remote service
./cfd-linux-amd64-obfuscated temp 1.2.3.4 https 443
```

### Temporary Tunnel (No Account Required)

```bash
# Expose a local service on port 8080
./cfd-linux-amd64-obfuscated tun --url http://localhost:8080
```

Both methods generate a random `*.trycloudflare.com` URL.

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

## 🛡️ Binary Wrapper (Privacy Protection)

Wrap the compiled binary into an encrypted, self-contained script that:
- **Completely hides** the original binary content from file scanners
- **Executes in memory** — never writes the real binary to disk
- **No residual files** — even if killed with `kill -9`
- **No OpenSSL dependency** — uses only Python 2.7+ built-in functions
- **Real-time progress** indicator during decryption

### How It Works

```
┌─────────────────────────────────────────────────────┐
│  Wrapper File (appears as bash script to scanners)  │
├─────────────────────────────────────────────────────┤
│  ┌─────────────┐    ┌──────────────────────────┐   │
│  │ Decrypt Key │    │ XOR + Base64 Encrypted   │   │
│  │ (embedded)  │    │ Binary Payload           │   │
│  └─────────────┘    └──────────────────────────┘   │
└─────────────────────────────────────────────────────┘
                         │
                    On Execute:
                         │
                         ▼
         ┌───────────────────────────────┐
         │ 1. Read payload with progress │
         │ 2. Base64 decode              │
         │ 3. XOR decrypt                │
         │ 4. memfd_create (fileless)    │
         │    or /dev/shm + rm (no disk) │
         │ 5. exec via fd                │
         └───────────────────────────────┘
```

### Execution Methods (in priority order)

| Method | How | Residual Files | Kernel Requirement |
|--------|-----|:--------------:|:------------------:|
| `memfd_create` | Anonymous memory fd | **None** | Linux 3.17+ |
| `/dev/shm` + immediate delete | RAM fs → open fd → delete → exec via fd | **None** | Any Linux |

### Create a Wrapper

```bash
cd wrapper/

# Basic usage
python create_wrapper.py <input_binary> [output_name]

# Examples
python create_wrapper.py ../cfd-linux-amd64 wrapped_cfd
python create_wrapper.py /path/to/binary ./my_wrapped_binary
```

### Output Example

```
[*] Creating fileless memory-execution wrapper...
[+] Original binary size: 28905598 bytes (27.6 MB)
[+] Generated encryption key
[*] Encrypting binary...
[*] Encoding payload...
============================================================
[+] Wrapper created successfully!
============================================================
[+] Output: wrapped_cfd
[+] Size: 39050230 bytes (37.2 MB)
```

### Run the Wrapper

```bash
# Run like a normal binary - all arguments are passed through
./wrapped_cfd --version
./wrapped_cfd temp local tcp 22
./wrapped_cfd tun run --token <TOKEN>
```

### Runtime Output

```
[*] Loading encrypted payload...
[*] Reading payload... 37 MB
[*] Decoding payload... 100%
[*] Decrypting payload... 100%
[+] Ready! Launching...
cloudflared version DEV (built unknown)
```

### File Comparison

| Property | Original Binary | Wrapped Binary |
|----------|:-:|:-:|
| File type | `ELF 64-bit executable` | `Bourne-Again shell script` |
| Content visible | ✅ Yes (binary patterns) | ❌ No (encrypted Base64) |
| Scanner detection | ⚠️ Identifiable | ✅ Hidden |
| SHA256 match | — | Completely different |
| Disk footprint at runtime | On disk | **In memory only** |

### Requirements (on the target machine)

- **Python 2.7+** (for decryption) — pre-installed on virtually all Linux systems
- **Perl** (for memfd_create) — falls back to /dev/shm if unavailable
- **No OpenSSL** required
- **No root** required

## License

This project is based on [cloudflare/cloudflared](https://github.com/cloudflare/cloudflared), licensed under the Apache License 2.0.
