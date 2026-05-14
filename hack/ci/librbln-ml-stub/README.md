# librbln-ml stub for CI

A tiny shared library that satisfies the linker when the rblnml-flavored
build (`make build-rblnml`, used by `make package-deb` / `package-rpm`)
runs on a host without the Rebellions UMD package installed.

## Why it exists

CGO requires `librbln-ml.so` to be findable by `ld` at link time. CI
runners (and most contributor machines) don't have the driver installed,
which means `make build-rblnml` would fail on `library 'rbln-ml' not
found` before producing a binary. Stubbing the library lets the workflow
exercise the cgo compilation path end-to-end and catch broken signatures
or accidental new symbol calls.

## What it defines

Every rblnml C symbol that `third_party/go-rbln-ml/rblnml/rblnml.go`
wraps with a `C.rblnml...(...)` call — about 26 functions covering
init/shutdown, device handle lookup, device info, version strings, PCIe
info, and hardware monitoring. The four `RsdGroup*` symbols are excluded
because we already trimmed those wrappers from the vendored bindings
(see `third_party/go-rbln-ml/VENDORED.md`).

> We *would* like to stub only the five symbols `internal/topology`
> actually invokes at runtime (`rblnmlInit`, `Shutdown`,
> `DeviceGetCount`, `DeviceGetHandleByIndex`, `GetDeviceInfo`) and rely
> on Go's dead-code elimination to drop the rest. In practice cgo
> registers every wrapper through its runtime tables, which the linker
> can't prove unreachable — so the full surface of the bound package
> ends up as undefined references at link time. The stub mirrors that
> surface.

## What it does *not* do

Every function returns `RBLNML_SUCCESS` with zeroed outputs. Binaries
linked against this stub will exercise their `topology.RsdResolver` code
path and see an empty NPU mapping — fine for "did it compile?" smoke
tests, not fine for any test that asserts real driver behavior. **Never
ship a binary that was linked against this stub** — production builds
must link against the upstream `librbln-ml`.

## Maintenance

When CI fails with `undefined reference to rblnmlXxx`, the toolkit grew a
new librbln-ml call. Add `Xxx` to `stub.c` (parameters and return type
visible in `third_party/go-rbln-ml/include/uapi/rblnml.h`) and bump the
table above. The stub `.c` deliberately mirrors the bundled header, so a
header re-vendor that changes a signature will surface as a compile
error here.

## Building manually

```bash
./hack/ci/build-librbln-ml-stub.sh /tmp/stub
# Output: /tmp/stub/librbln-ml.so.1 and a librbln-ml.so symlink

LIBRARY_PATH=/tmp/stub LD_LIBRARY_PATH=/tmp/stub make build-rblnml
```
