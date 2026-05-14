# go-rbln-ml

Go bindings for the **RBLNML** (Rebellions Management Library) C API via `cgo`.
Provides a Go-idiomatic interface to manage and query Rebellions AI NPU devices.

> **⚠️ Vendored snapshot.** This is a trimmed copy of the upstream module
> brought in-tree under `third_party/`. See `VENDORED.md` for provenance and
> the list of patches/dropped paths. Refer to the upstream repository for
> the full unmodified tree, the `example/`/`test/` directories, and the
> RSD group management API (`RsdGroupCreate`/`Destroy`/`CreateAll`/
> `DestroyAll`) that's been removed here for older-driver compatibility.

## Repository layout (vendored)

```
third_party/go-rbln-ml/
├── rblnml/                 # package rblnml — cgo bindings (RsdGroup* removed)
│   ├── rblnml.go
│   ├── errors.go
│   └── types.go
├── include/uapi/rblnml.h   # bundled C header (public API contract)
├── Makefile                # trimmed: build only, no example/test targets
├── go.mod                  # module github.com/RBLN-SW/go-rbln-ml
├── VENDORED.md             # snapshot metadata + patch log
└── README.md
```

> **Note:** `include/uapi/rblnml.h` is the canonical public API header for
> the Rebellions Management Library. It must be kept in sync with the
> upstream C library whenever the API changes.

## Prerequisites

| Dependency | Notes |
|---|---|
| Go 1.18+ | `cgo` enabled (default) |
| `librbln-ml.so` | Installed system-wide, or specify path via `LIB=` (see below) |
| C compiler (`gcc` / `clang`) | Required by `cgo` |

### Verifying librbln-ml installation

Check whether the library is already available on your system:

```bash
ldconfig -p | grep rbln-ml
```

If `librbln-ml.so` appears in the output, the library is installed and ready
to use. If not, contact your system administrator or refer to the Rebellions
device software distribution for your platform.

## Build

### System-wide library (recommended)

If `librbln-ml.so` is installed in a standard system path (e.g. `/usr/lib`,
`/usr/local/lib`):

```bash
make build
```

This compiles the `rblnml/` package as a compilation check. The upstream's
`example/` build target was removed during vendoring (see VENDORED.md). Run
`make clean` to remove `bin/`.

### Library in a custom path

Pass the directory containing `librbln-ml.so` via the `LIB` variable.
The Makefile sets `CGO_LDFLAGS` and `LD_LIBRARY_PATH` automatically:

```bash
make LIB=/path/to/librbln-ml/lib build
```

### Use as a dependency (replace directive)

In your project's `go.mod`:

```go
module your-app

go 1.18

require github.com/RBLN-SW/go-rbln-ml v0.0.0

replace github.com/RBLN-SW/go-rbln-ml => /path/to/go-rbln-ml
```

## Usage

### Quick start

```go
package main

import (
    "fmt"
    "log"
    "github.com/RBLN-SW/go-rbln-ml/rblnml"
)

func main() {
    r, err := rblnml.New()
    if err != nil {
        log.Fatal(err)
    }
    defer r.Shutdown()

    count, err := r.DeviceGetCount()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Found %d devices\n", count)

    dev, err := r.DeviceGetHandleByIndex(0)
    if err != nil {
        log.Fatal(err)
    }

    info, err := r.GetDeviceInfo(dev)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("UUID:  %s\n", info.UUID)
    fmt.Printf("BusID: %s\n", info.BusID)
}
```

### RSD group APIs

> **Not available in this vendored snapshot.** The upstream package exposes
> `NewForRsdGroupOnly()`, `RsdGroupCreate`, `RsdGroupCreateAll`,
> `RsdGroupDestroy`, and `RsdGroupDestroyAll`. Those wrappers were removed
> during vendoring because the symbols they reference are missing from
> older `librbln-ml.so` builds the container toolkit must support. See
> VENDORED.md for the rationale and the restoration criterion.

### Running the example

The upstream's `example/` directory was dropped during vendoring; refer to
the upstream repository for runnable example code.

### Running tests

The upstream's `test/` directory (RSD group integration tests) was dropped
during vendoring. The container toolkit covers the trimmed surface via its
own `internal/topology` test suite.

## API Reference

### Initialization

| Function | Description |
|---|---|
| `New() (*Rblnml, error)` | Initialize library, open all `/dev/rbln*` devices |
| `NewWithDevlist([]uint32) (*Rblnml, error)` | Initialize with specific device indices |
| `(*Rblnml).Shutdown() error` | Close all devices and shut down |

> `NewForRsdGroupOnly()` exists in the upstream tree; in this vendored
> snapshot it remains callable but the RSD group methods it gates are
> patched out (see below) and the constructor is effectively dead code.

### Device Management

| Function | Description |
|---|---|
| `DeviceGetCount() (uint32, error)` | Number of initialized devices |
| `DeviceGetHandleByIndex(uint32) (Device, error)` | Handle by 0-based index |
| `DeviceGetHandleByPciBusId(string) (Device, error)` | Handle by PCI bus ID |
| `DeviceGetHandleByUuid(string) (Device, error)` | Handle by UUID |
| `DeviceGetHandleBySerialId(string) (Device, error)` | Handle by serial ID |

### Device Information

| Function | Description |
|---|---|
| `GetDeviceInfo(Device) (DeviceInfo, error)` | UUID, BusID, SerialID, GroupID, NUMANode |
| `GetDeviceHWIPInfo(Device) (HwIpInfo, error)` | SRAM/DRAM sizes, card name, versions |

### Version Information

| Function | Description |
|---|---|
| `GetKernelVersion(Device) (string, error)` | Kernel driver version |
| `GetFwVersion(Device) (string, error)` | Firmware version |
| `GetSmcVersion(Device) (string, error)` | SMC version |

### PCI Information

| Function | Description |
|---|---|
| `GetPciInfo(Device) (PciInfo, error)` | Full PCI topology |
| `GetPcieMPS(Device) (uint32, error)` | Max Payload Size (bytes) |
| `GetPcieMRR(Device) (uint32, error)` | Max Read Request size (bytes) |
| `GetPcieLinkCurSpeed(Device) (float64, error)` | Current link speed (GB/s) |
| `GetPcieLinkCurWidth(Device) (uint32, error)` | Current link width (lanes) |
| `GetPcieLinkMaxSpeed(Device) (float64, error)` | Maximum link speed (GB/s) |
| `GetPcieLinkMaxWidth(Device) (uint32, error)` | Maximum link width (lanes) |

### Memory Information

| Function | Description |
|---|---|
| `DeviceGetBAR0MemoryInfo(Device) (Bar0Memory, error)` | BAR0 free/used/total |

### Hardware Monitoring

| Function | Description |
|---|---|
| `DeviceGetTemperature(Device) (uint32, error)` | Device temperature (degrees Celsius) |
| `DeviceGetPowerUsage(Device) (uint32, error)` | Power usage (uW) |
| `DeviceGetPowerState(Device) (uint32, error)` | Power state |
| `DeviceGetUtilization(Device) (uint32, error)` | Device utilization |
| `DeviceGetContextInfo(Device, uint32) ([]ContextInfo, error)` | Active context list |

### RSD Group Management

> **Not available in this vendored snapshot.** Upstream exposes
> `RsdGroupCreate`, `RsdGroupCreateAll`, `RsdGroupDestroy`, and
> `RsdGroupDestroyAll`; the wrappers were trimmed to keep the container
> toolkit linkable against older `librbln-ml.so` builds. See VENDORED.md
> for the upstream commit and the patch list.

## Error Handling

All functions return a Go `error`. A non-nil error is `*RblnmlError` carrying
a `Code ReturnCode` field for programmatic inspection:

```go
dev, err := r.DeviceGetHandleByIndex(99)
if err != nil {
    var rblnErr *rblnml.RblnmlError
    if errors.As(err, &rblnErr) {
        fmt.Println(rblnErr.Code) // e.g. RBLNML_ERROR_NOT_FOUND
    }
}
```

## Header synchronization

`include/uapi/rblnml.h` in this repository is the public API contract for
`librbln-ml`. When the upstream C library updates its API, this header must
be updated together with the Go bindings.
