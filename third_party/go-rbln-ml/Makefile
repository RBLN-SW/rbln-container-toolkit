BIN     := bin
GOFLAGS ?=

# Vendored snapshot — see VENDORED.md for provenance and the upstream's full
# build/test surface. This Makefile is intentionally trimmed: the original
# example/ and test/ directories were dropped during vendoring because they
# exercise RSD group APIs that this snapshot patches out for older driver
# compatibility. Re-vendor the upstream tree if you need those targets.

# Set LIB to the directory containing librbln-ml.so when it is not in a
# standard system path:
#   make LIB=/path/to/librbln-ml/lib build
ifdef LIB
export CGO_LDFLAGS    := -L$(LIB) -Wl,-rpath,$(LIB)
export LD_LIBRARY_PATH := $(LIB)$(if $(LD_LIBRARY_PATH),:$(LD_LIBRARY_PATH))
endif

.PHONY: all build vet fmt clean

all: build

build: $(BIN)
	go build $(GOFLAGS) ./rblnml/

vet:
	go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -rf $(BIN)/

$(BIN):
	mkdir -p $@
