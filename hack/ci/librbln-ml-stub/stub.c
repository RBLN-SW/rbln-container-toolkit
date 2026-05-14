/*
 * Copyright 2026 Rebellions Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/*
 * Minimal stub for librbln-ml symbols the container toolkit references at
 * link time. Used by CI runners that don't have the Rebellions UMD
 * package installed — the resulting `librbln-ml.so` lets `make build-rblnml`
 * reach the end of cgo + ld so the workflow can catch signature/wiring
 * regressions, but the produced binaries aren't runnable: every function
 * returns RBLNML_SUCCESS with zeroed outputs. Production binaries link
 * against the real library on the packaging host (or the runtime host
 * via the DEB/RPM dependency).
 *
 * Why so many functions?
 * In theory Go's dead-code eliminator should drop wrappers that
 * `internal/topology/rblnml.go` doesn't transitively call. In practice
 * cgo registers each wrapper through the cgo runtime tables, which the
 * linker can't prove unreachable — so every C symbol referenced by any
 * Go wrapper in the bound package ends up as an undefined reference at
 * link time, even when only a handful are actually invoked at runtime.
 * Stub the whole surface that `third_party/go-rbln-ml/rblnml/rblnml.go`
 * wraps (RsdGroup* are excluded because we already trimmed them from
 * the vendored bindings).
 *
 * Maintenance contract: if `make build-rblnml` on CI starts failing with
 * `undefined reference to rblnmlXxx`, add a stub for Xxx here and bump
 * the matching note in README.md.
 */

#include "uapi/rblnml.h"

/* --- Initialization -------------------------------------------------- */

rblnmlReturn_t rblnmlInit(void) {
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlInitWithDevlist(const uint32_t *device_list, size_t count) {
	(void)device_list;
	(void)count;
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlShutdown(void) {
	return RBLNML_SUCCESS;
}

/* --- Device handle lookup ------------------------------------------- */

rblnmlReturn_t rblnmlDeviceGetCount(unsigned int *count) {
	if (count != NULL) {
		*count = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlDeviceGetHandleByIndex(unsigned int index, rblnmlDevice_t *device) {
	(void)index;
	if (device != NULL) {
		*device = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlDeviceGetHandleByPciBusId(const char *pci_bus_id, rblnmlDevice_t *device) {
	(void)pci_bus_id;
	if (device != NULL) {
		*device = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlDeviceGetHandleByUuid(const char *uuid, rblnmlDevice_t *device) {
	(void)uuid;
	if (device != NULL) {
		*device = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlDeviceGetHandleBySerialId(const char *serial_id, rblnmlDevice_t *device) {
	(void)serial_id;
	if (device != NULL) {
		*device = 0;
	}
	return RBLNML_SUCCESS;
}

/* --- Device info ----------------------------------------------------- */

rblnmlReturn_t rblnmlGetDeviceInfo(rblnmlDevice_t device, rblnmlDeviceInfo_t *info) {
	(void)device;
	(void)info;
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlGetDeviceHWIPInfo(rblnmlDevice_t device, rblnmlHwIpInfo_t *info) {
	(void)device;
	(void)info;
	return RBLNML_SUCCESS;
}

/* --- Versions -------------------------------------------------------- */

rblnmlReturn_t rblnmlGetKernelVersion(rblnmlDevice_t device, char *buffer, size_t size) {
	(void)device;
	if (buffer != NULL && size > 0) {
		buffer[0] = '\0';
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlGetFwVersion(rblnmlDevice_t device, char *buffer, size_t size) {
	(void)device;
	if (buffer != NULL && size > 0) {
		buffer[0] = '\0';
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlGetSmcVersion(rblnmlDevice_t device, char *buffer, size_t size) {
	(void)device;
	if (buffer != NULL && size > 0) {
		buffer[0] = '\0';
	}
	return RBLNML_SUCCESS;
}

/* --- PCIe info ------------------------------------------------------- */

rblnmlReturn_t rblnmlGetPciInfo(rblnmlDevice_t device, rblnmlPciInfo_t *info) {
	(void)device;
	(void)info;
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlGetPcieMPS(rblnmlDevice_t device, unsigned int *mps) {
	(void)device;
	if (mps != NULL) {
		*mps = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlGetPcieMRR(rblnmlDevice_t device, unsigned int *mrr) {
	(void)device;
	if (mrr != NULL) {
		*mrr = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlGetPcieLinkCurSpeed(rblnmlDevice_t device, double *speed) {
	(void)device;
	if (speed != NULL) {
		*speed = 0.0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlGetPcieLinkCurWidth(rblnmlDevice_t device, unsigned int *width) {
	(void)device;
	if (width != NULL) {
		*width = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlGetPcieLinkMaxSpeed(rblnmlDevice_t device, double *speed) {
	(void)device;
	if (speed != NULL) {
		*speed = 0.0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlGetPcieLinkMaxWidth(rblnmlDevice_t device, unsigned int *width) {
	(void)device;
	if (width != NULL) {
		*width = 0;
	}
	return RBLNML_SUCCESS;
}

/* --- Memory & monitoring -------------------------------------------- */

rblnmlReturn_t rblnmlDeviceGetBAR0MemoryInfo(rblnmlDevice_t device, rblnmlBar0Memory_t *info) {
	(void)device;
	(void)info;
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlDeviceGetTemperature(rblnmlDevice_t device, rblnmlTemperature_t *temperature) {
	(void)device;
	if (temperature != NULL) {
		*temperature = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlDeviceGetPowerUsage(rblnmlDevice_t device, rblnmlPower_t *power) {
	(void)device;
	if (power != NULL) {
		*power = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlDeviceGetPowerState(rblnmlDevice_t device, rblnmlPState_t *pstate) {
	(void)device;
	if (pstate != NULL) {
		*pstate = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlDeviceGetUtilization(rblnmlDevice_t device, uint32_t *utilization) {
	(void)device;
	if (utilization != NULL) {
		*utilization = 0;
	}
	return RBLNML_SUCCESS;
}

rblnmlReturn_t rblnmlDeviceGetContextInfo(rblnmlDevice_t device, rblnmlContextInfo_t *info,
					  size_t count) {
	(void)device;
	(void)info;
	(void)count;
	return RBLNML_SUCCESS;
}
