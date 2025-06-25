# kubectl-ball  🎱

I made this initially at Tinder, in BASH. Since I no longer have access to that codebase, I have remade it for the public and improved on it with the help of AI, in Go.

A `kubectl` plugin to operate on **multiple Kubernetes clusters** simultaneously — with fzf-based selection, smart grepping, output formatting, namespace syncing, and kubeconfig merging.

[![CI](https://github.com/robolague/kubectl-ball/actions/workflows/ci.yml/badge.svg)](https://github.com/robolague/kubectl-ball/actions/workflows/ci.yml)
[![Lint](https://github.com/robolague/kubectl-ball/actions/workflows/lint.yml/badge.svg)](https://github.com/robolague/kubectl-ball/actions/workflows/lint.yml)
[![Release](https://github.com/robolague/kubectl-ball/actions/workflows/release.yml/badge.svg)](https://github.com/robolague/kubectl-ball/actions/workflows/release.yml)

---

## ✨ Features

- 🔍 **fzf-based interactive cluster selection** (`--select`)
- 📁 **Shared namespace** across all clusters (`-n`)
- 🧠 **Smart grep filtering** (`--grep`) with per-cluster headers
- 🎨 **Formatted output** (`--format json|yaml|wide|table`)
- 🔀 **Auto kubeconfig merging** (no flag needed)
- ⚡ **Parallel execution** of `kubectl` across clusters
- 📦 **Krew-compatible** plugin layout
- 🐳 **Docker support** with host kubeconfig access

---

## 🛠️ Installation

### Option 1: Via [krew](https://krew.sigs.k8s.io/) (coming soon)

```bash
kubectl krew install ball
```

### Option 2: Manual build
```bash
git clone https://github.com/robolague/kubectl-ball.git
cd kubectl-ball
go build -o kubectl-ball main.go
mv kubectl-ball ~/.krew/bin/   # or any directory in your $PATH
```
Make it executable and run as:
```bash
kubectl ball ...
```

### Option 3: Docker
```bash
# Build the Docker image
make docker-build

# Run kubectl-ball in Docker (mounts your ~/.kube directory)
make docker-run ARGS="--select get pods"

# Or run interactively
make docker-shell
```



## 🚀 Usage
Select clusters interactively (and persist selection)
```bash
kubectl ball --select get pods
```
Reuse previous selection
```bash
kubectl ball get services
```
Sync namespace across clusters
```bash
kubectl ball -n dev get deployments
```
Grep across all clusters
```bash
kubectl ball --grep CrashLoopBackOff get pods -A
```
Format output
```bash
kubectl ball --format yaml get configmaps
```

## 🐳 Docker Usage
The Docker image includes kubectl and fzf, and automatically mounts your host's kubeconfig:

```bash
# Build the image
make docker-build

# Run a command
make docker-run ARGS="get pods"

# Run with cluster selection
make docker-run ARGS="--select get services"

# Interactive shell
make docker-shell
```

The Docker container:
- Mounts your `~/.kube` directory (read-only)
- Sets `KUBECONFIG=/root/.kube/config`
- Mounts current directory as `/workspace`
- Includes kubectl and fzf pre-installed

## 🧪 Output Example
```bash
===== [dev-cluster] =====
kube-system   dns-abc123   0/1   CrashLoopBackOff   2m

===== [prod-cluster] =====
web-team      frontend-xyz   0/1   CrashLoopBackOff   30s
```

## 🔧 Building and Releasing
Build for all supported platforms
```bash
make all
```
Generate SHA256 checksums
```bash
make sha256
```
Clean up build artifacts
```bash
make clean
```
Build Docker image
```bash
make docker-build
```

Release binaries go to release/, ready for upload to GitHub and Krew.

## 🧊 Krew Plugin Manifest (plugin.yaml)
See plugin.yaml for Krew submission format.

To publish:

Fork krew-index

Copy your plugin.yaml into the plugins/ folder

Submit a PR titled: Add kubectl-ball

## 🤝 Contributing
Fork this repo

Make your changes in a feature branch

Test:
```bash
go build -o kubectl-ball main.go
./kubectl-ball --select get pods
```
Submit a pull request 🙌

## 📜 License
This project is licensed under the terms of the GNU General Public License v3.0.
See the LICENSE file for details.
