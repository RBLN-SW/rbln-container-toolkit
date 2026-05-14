// Package rblnml provides Go bindings for the RBLNML (Rebellions Management
// Library) C API via cgo. It wraps librbln-ml.so and exposes all library
// functions using idiomatic Go conventions: output parameters become return
// values and rblnmlReturn_t is converted to a Go error.
//
// Build requirements:
//   - librbln-ml.so (or librbln-ml.a) must be findable by the linker.
//     Install system-wide or set CGO_LDFLAGS=-L/path/to/lib before building.
//   - rblnml.h is bundled under include/uapi/ in this repository.
//
// Example:
//
//	r, err := rblnml.New()
//	if err != nil { log.Fatal(err) }
//	defer r.Shutdown()
//
//	count, err := r.DeviceGetCount()
//	if err != nil { log.Fatal(err) }
//	fmt.Printf("Found %d devices\n", count)
package rblnml

/*
#cgo CFLAGS: -I${SRCDIR}/../include
#cgo LDFLAGS: -lrbln-ml
#include <uapi/rblnml.h>
#include <stdlib.h>
*/
import "C"
import (
	"unsafe"
)

// Rblnml is the main handle for interacting with the library. Create one with
// New() or NewWithDevlist() and call Shutdown() when done.
type Rblnml struct{}

// New initializes the rblnml library by opening all /dev/rbln* devices.
func New() (*Rblnml, error) {
	rc := ReturnCode(C.rblnmlInit())
	if err := toError(rc); err != nil {
		return nil, err
	}
	return &Rblnml{}, nil
}

// NewForRsdGroupOnly returns an Rblnml handle without calling rblnmlInit.
// Originally meant to gate calls to the RSD group management API
// (RsdGroupCreate / RsdGroupDestroy / RsdGroupCreateAll / RsdGroupDestroyAll)
// without opening /dev/rbln* — those wrappers are patched out of this
// vendored snapshot for older-driver compatibility (see VENDORED.md), so
// the constructor is currently dead code. Kept for upstream parity; remove
// once a future re-vendor restores the group API.
func NewForRsdGroupOnly() *Rblnml {
	return &Rblnml{}
}

// NewWithDevlist initializes the library with the specified device indices
// (e.g. []uint32{0, 1} opens rbln0 and rbln1).
func NewWithDevlist(devices []uint32) (*Rblnml, error) {
	if len(devices) == 0 {
		return nil, toError(ErrorInvalidArg)
	}
	rc := ReturnCode(C.rblnmlInitWithDevlist(
		(*C.uint32_t)(unsafe.Pointer(&devices[0])),
		C.size_t(len(devices)),
	))
	if err := toError(rc); err != nil {
		return nil, err
	}
	return &Rblnml{}, nil
}

// Shutdown closes all open device handles and shuts down the library.
func (r *Rblnml) Shutdown() error {
	return toError(ReturnCode(C.rblnmlShutdown()))
}

// --- Device Management ---

// DeviceGetCount returns the number of devices found during initialization.
func (r *Rblnml) DeviceGetCount() (uint32, error) {
	var count C.uint
	rc := ReturnCode(C.rblnmlDeviceGetCount(&count))
	return uint32(count), toError(rc)
}

// DeviceGetHandleByIndex returns the device handle for the given 0-based index.
func (r *Rblnml) DeviceGetHandleByIndex(index uint32) (Device, error) {
	var dev C.rblnmlDevice_t
	rc := ReturnCode(C.rblnmlDeviceGetHandleByIndex(C.uint(index), &dev))
	return Device(dev), toError(rc)
}

// DeviceGetHandleByPciBusId returns the device handle for the given PCI bus ID
// string (e.g. "0000:01:00.0").
func (r *Rblnml) DeviceGetHandleByPciBusId(pciBusID string) (Device, error) {
	cs := C.CString(pciBusID)
	defer C.free(unsafe.Pointer(cs))
	var dev C.rblnmlDevice_t
	rc := ReturnCode(C.rblnmlDeviceGetHandleByPciBusId(cs, &dev))
	return Device(dev), toError(rc)
}

// DeviceGetHandleByUuid returns the device handle for the given UUID string.
func (r *Rblnml) DeviceGetHandleByUuid(uuid string) (Device, error) {
	cs := C.CString(uuid)
	defer C.free(unsafe.Pointer(cs))
	var dev C.rblnmlDevice_t
	rc := ReturnCode(C.rblnmlDeviceGetHandleByUuid(cs, &dev))
	return Device(dev), toError(rc)
}

// DeviceGetHandleBySerialId returns the device handle for the given serial ID
// string (16-digit hex).
func (r *Rblnml) DeviceGetHandleBySerialId(serialID string) (Device, error) {
	cs := C.CString(serialID)
	defer C.free(unsafe.Pointer(cs))
	var dev C.rblnmlDevice_t
	rc := ReturnCode(C.rblnmlDeviceGetHandleBySerialId(cs, &dev))
	return Device(dev), toError(rc)
}

// --- Device Information ---

// GetDeviceHWIPInfo returns hardware IP information for the given device.
func (r *Rblnml) GetDeviceHWIPInfo(dev Device) (HwIpInfo, error) {
	var info C.rblnmlHwIpInfo_t
	rc := ReturnCode(C.rblnmlGetDeviceHWIPInfo(C.rblnmlDevice_t(dev), &info))
	if err := toError(rc); err != nil {
		return HwIpInfo{}, err
	}
	return HwIpInfo{
		SRAMBaseAddress: uint64(info.sram_base_address),
		DRAMBaseAddress: uint64(info.dram_base_address),
		DRAMSize:        uint64(info.dram_size),
		SRAMSize:        uint32(info.sram_size),
		DeviceID:        uint32(info.device_id),
		CPLDVersion:     uint32(info.cpld_version),
		ChipletCnt:      uint32(info.chiplet_cnt),
		CPUCPVersion:    C.GoString((*C.char)(unsafe.Pointer(&info.cpucp_version[0]))),
		CardName:        C.GoString((*C.char)(unsafe.Pointer(&info.card_name[0]))),
		RevID:           uint32(info.rev_id),
		DRMMinor:        uint32(info.drm_minor),
		DClusterCnt:     uint32(info.dcluster_cnt),
	}, nil
}

// GetDeviceInfo returns device identification information.
func (r *Rblnml) GetDeviceInfo(dev Device) (DeviceInfo, error) {
	var info C.rblnmlDeviceInfo_t
	rc := ReturnCode(C.rblnmlGetDeviceInfo(C.rblnmlDevice_t(dev), &info))
	if err := toError(rc); err != nil {
		return DeviceInfo{}, err
	}
	return DeviceInfo{
		UUID:      C.GoString(&info.uuid[0]),
		BusID:     C.GoString(&info.bus_id[0]),
		SerialID:  uint64(info.serial_id),
		BoardInfo: uint32(info.board_info),
		DeviceID:  uint32(info.device_id),
		GroupID:   uint32(info.group_id),
		NUMANode:  uint32(info.numa_node),
	}, nil
}

// --- Version Information ---

// GetKernelVersion returns the kernel driver version string for the device.
func (r *Rblnml) GetKernelVersion(dev Device) (string, error) {
	buf := make([]byte, VersionLength)
	rc := ReturnCode(C.rblnmlGetKernelVersion(
		C.rblnmlDevice_t(dev),
		(*C.char)(unsafe.Pointer(&buf[0])),
		C.size_t(len(buf)),
	))
	if err := toError(rc); err != nil {
		return "", err
	}
	return C.GoString((*C.char)(unsafe.Pointer(&buf[0]))), nil
}

// GetFwVersion returns the firmware version string for the device.
func (r *Rblnml) GetFwVersion(dev Device) (string, error) {
	buf := make([]byte, VersionLength)
	rc := ReturnCode(C.rblnmlGetFwVersion(
		C.rblnmlDevice_t(dev),
		(*C.char)(unsafe.Pointer(&buf[0])),
		C.size_t(len(buf)),
	))
	if err := toError(rc); err != nil {
		return "", err
	}
	return C.GoString((*C.char)(unsafe.Pointer(&buf[0]))), nil
}

// GetSmcVersion returns the SMC version string for the device.
func (r *Rblnml) GetSmcVersion(dev Device) (string, error) {
	buf := make([]byte, VersionLength)
	rc := ReturnCode(C.rblnmlGetSmcVersion(
		C.rblnmlDevice_t(dev),
		(*C.char)(unsafe.Pointer(&buf[0])),
		C.size_t(len(buf)),
	))
	if err := toError(rc); err != nil {
		return "", err
	}
	return C.GoString((*C.char)(unsafe.Pointer(&buf[0]))), nil
}

// --- PCI Information ---

// GetPciInfo returns PCI topology information for the device.
func (r *Rblnml) GetPciInfo(dev Device) (PciInfo, error) {
	var info C.rblnmlPciInfo_t
	rc := ReturnCode(C.rblnmlGetPciInfo(C.rblnmlDevice_t(dev), &info))
	if err := toError(rc); err != nil {
		return PciInfo{}, err
	}
	return PciInfo{
		Bus:            uint32(info.bus),
		BusID:          C.GoString(&info.bus_id[0]),
		Device:         uint32(info.device),
		Domain:         uint32(info.domain),
		PCIDeviceID:    uint32(info.pci_device_id),
		PCISubsystemID: uint32(info.pci_subsystem_id),
		MaxPayload:     uint32(info.max_payload),
		MaxReadReq:     uint32(info.max_readreq),
		LinkGen:        uint32(info.link_gen),
		LinkWidth:      uint32(info.link_width),
		MaxLinkGen:     uint32(info.max_link_gen),
		MaxLinkWidth:   uint32(info.max_link_width),
		IommuGroup:     int32(info.iommu_group),
	}, nil
}

// GetPcieMPS returns the PCIe Maximum Payload Size in bytes.
func (r *Rblnml) GetPcieMPS(dev Device) (uint32, error) {
	var mps C.uint
	rc := ReturnCode(C.rblnmlGetPcieMPS(C.rblnmlDevice_t(dev), &mps))
	return uint32(mps), toError(rc)
}

// GetPcieMRR returns the PCIe Maximum Read Request size in bytes.
func (r *Rblnml) GetPcieMRR(dev Device) (uint32, error) {
	var mrr C.uint
	rc := ReturnCode(C.rblnmlGetPcieMRR(C.rblnmlDevice_t(dev), &mrr))
	return uint32(mrr), toError(rc)
}

// GetPcieLinkCurSpeed returns the current PCIe link speed in GB/s.
func (r *Rblnml) GetPcieLinkCurSpeed(dev Device) (float64, error) {
	var speed C.double
	rc := ReturnCode(C.rblnmlGetPcieLinkCurSpeed(C.rblnmlDevice_t(dev), &speed))
	return float64(speed), toError(rc)
}

// GetPcieLinkCurWidth returns the current PCIe link width (number of lanes).
func (r *Rblnml) GetPcieLinkCurWidth(dev Device) (uint32, error) {
	var width C.uint
	rc := ReturnCode(C.rblnmlGetPcieLinkCurWidth(C.rblnmlDevice_t(dev), &width))
	return uint32(width), toError(rc)
}

// GetPcieLinkMaxSpeed returns the maximum PCIe link speed in GB/s.
func (r *Rblnml) GetPcieLinkMaxSpeed(dev Device) (float64, error) {
	var speed C.double
	rc := ReturnCode(C.rblnmlGetPcieLinkMaxSpeed(C.rblnmlDevice_t(dev), &speed))
	return float64(speed), toError(rc)
}

// GetPcieLinkMaxWidth returns the maximum PCIe link width (number of lanes).
func (r *Rblnml) GetPcieLinkMaxWidth(dev Device) (uint32, error) {
	var width C.uint
	rc := ReturnCode(C.rblnmlGetPcieLinkMaxWidth(C.rblnmlDevice_t(dev), &width))
	return uint32(width), toError(rc)
}

// --- Memory Information ---

// DeviceGetBAR0MemoryInfo returns BAR0 memory usage information.
func (r *Rblnml) DeviceGetBAR0MemoryInfo(dev Device) (Bar0Memory, error) {
	var info C.rblnmlBar0Memory_t
	rc := ReturnCode(C.rblnmlDeviceGetBAR0MemoryInfo(C.rblnmlDevice_t(dev), &info))
	if err := toError(rc); err != nil {
		return Bar0Memory{}, err
	}
	return Bar0Memory{
		Free:  uint64(info.bar0_free),
		Used:  uint64(info.bar0_used),
		Total: uint64(info.bar0_total),
	}, nil
}

// --- Hardware Monitoring ---

// DeviceGetTemperature returns the device temperature in degrees Celsius.
func (r *Rblnml) DeviceGetTemperature(dev Device) (uint32, error) {
	var temp C.uint32_t
	rc := ReturnCode(C.rblnmlDeviceGetTemperature(C.rblnmlDevice_t(dev), &temp))
	return uint32(temp), toError(rc)
}

// DeviceGetPowerUsage returns the device power usage in microwatts (uW).
func (r *Rblnml) DeviceGetPowerUsage(dev Device) (uint32, error) {
	var power C.uint32_t
	rc := ReturnCode(C.rblnmlDeviceGetPowerUsage(C.rblnmlDevice_t(dev), &power))
	return uint32(power), toError(rc)
}

// DeviceGetPowerState returns the device power state.
func (r *Rblnml) DeviceGetPowerState(dev Device) (uint32, error) {
	var pstate C.uint32_t
	rc := ReturnCode(C.rblnmlDeviceGetPowerState(C.rblnmlDevice_t(dev), &pstate))
	return uint32(pstate), toError(rc)
}

// DeviceGetUtilization returns the device utilization value.
func (r *Rblnml) DeviceGetUtilization(dev Device) (uint32, error) {
	var util C.uint32_t
	rc := ReturnCode(C.rblnmlDeviceGetUtilization(C.rblnmlDevice_t(dev), &util))
	return uint32(util), toError(rc)
}

// DeviceGetContextInfo returns exactly count context entries for the device.
// The caller must pass the number of active contexts; entries beyond the
// actual active count are zero-valued. The C API does not report the actual
// filled count, so the caller is responsible for passing the correct value.
func (r *Rblnml) DeviceGetContextInfo(dev Device, count uint32) ([]ContextInfo, error) {
	if count == 0 {
		return nil, toError(ErrorInvalidArg)
	}
	raw := make([]C.rblnmlContextInfo_t, count)
	rc := ReturnCode(C.rblnmlDeviceGetContextInfo(
		C.rblnmlDevice_t(dev),
		&raw[0],
		C.size_t(count),
	))
	if err := toError(rc); err != nil {
		return nil, err
	}
	out := make([]ContextInfo, count)
	for i, c := range raw {
		out[i] = ContextInfo{
			TaskNum:      uint32(c.task_num),
			RsdCtxHandle: uint32(c.rsd_ctx_handle),
			CtxHandle:    uint32(c.ctx_handle),
			Priority:     uint32(c.priority),
			ASID:         uint32(c.asid),
			DRAMPhysMem:  uint64(c.dram_phys_mem),
			DoneSeq:      uint64(c.done_seq),
			SubmittedSeq: uint64(c.submitted_seq),
			ReqSeq:       uint64(c.req_seq),
			CoreUtilLast: uint32(c.core_util_last),
			Flags:        uint32(c.flags),
			NumBuf:       uint32(c.num_buf),
			NumProc:      uint32(c.num_proc),
			NumMain:      uint32(c.num_main),
			NumSub:       uint32(c.num_sub),
			NumTask:      uint32(c.num_task),
			Error:        uint32(c.error),
			Destroyed:    c.destroyed != 0,
			Guilty:       c.guilty != 0,
		}
	}
	return out, nil
}

// --- RSD Group Management ---
//
// Removed in this vendored copy: RsdGroupCreate, RsdGroupCreateAll,
// RsdGroupDestroy, RsdGroupDestroyAll. The container toolkit only needs
// GetDeviceInfo (for the NPU→GroupID resolver); the group-management
// surface introduces link-time references to rblnmlRsdGroup* symbols that
// older librbln-ml.so builds don't ship. Trimming the wrappers here lets
// the toolkit build against driver versions that predate the group API
// without losing the resolver functionality. See VENDORED.md "Patches".
