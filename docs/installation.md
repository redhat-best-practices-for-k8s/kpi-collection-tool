# Installation

## Download a pre-built binary (recommended)

Go to the [GitHub Releases](https://github.com/redhat-best-practices-for-k8s/kpi-collection-tool/releases) page, download the archive that matches your platform, and extract it.

You can run the binary directly from the extracted directory:

```bash
./kpi-collector --help
```

Or move it to a directory on your PATH so it's available everywhere:

```bash
sudo mv kpi-collector /usr/local/bin/
kpi-collector --help
```

No Go toolchain or build dependencies required.

> [!NOTE]
> All examples in this documentation assume `kpi-collector` is on your
> PATH. If you prefer not to move it, replace `kpi-collector` with the
> full path to the binary wherever it appears.

## Using `go install`

If you have Go 1.26+ installed:

```bash
go install github.com/redhat-best-practices-for-k8s/kpi-collection-tool/cmd/kpi-collector@latest
```

Verify:

```bash
kpi-collector --help
```

If the command is not found, add `~/go/bin` to your PATH:

```bash
export PATH="$HOME/go/bin:$PATH"
```

## From Source

Clone the repo and build locally:

```bash
git clone https://github.com/redhat-best-practices-for-k8s/kpi-collection-tool.git
cd kpi-collection-tool
make build
```

This produces a statically linked `kpi-collector` binary in the project root.
To install it to `~/go/bin`:

```bash
make install
```

## Uninstall

```bash
# If installed via make install
make uninstall

# If installed via go install
rm ~/go/bin/kpi-collector

# If installed manually
rm /usr/local/bin/kpi-collector
```
