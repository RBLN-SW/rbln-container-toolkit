package rblnml

// Constants matching rblnml.h.
const (
	VersionLength             = 128
	CardNameMaxLen            = 16
	UUIDBufferSize            = 38
	DevicePCIBusIDBufferSize  = 32
)

// Device is an opaque handle identifying a single Rebellions NPU device.
type Device uint64

// HwIpInfo mirrors rblnmlHwIpInfo_t.
type HwIpInfo struct {
	SRAMBaseAddress uint64
	DRAMBaseAddress uint64
	DRAMSize        uint64
	SRAMSize        uint32
	DeviceID        uint32
	CPLDVersion     uint32
	ChipletCnt      uint32
	CPUCPVersion    string
	CardName        string
	RevID           uint32
	DRMMinor        uint32
	DClusterCnt     uint32
}

// DeviceInfo mirrors rblnmlDeviceInfo_t.
type DeviceInfo struct {
	UUID      string
	BusID     string
	SerialID  uint64
	BoardInfo uint32
	DeviceID  uint32
	GroupID   uint32
	NUMANode  uint32
}

// PciInfo mirrors rblnmlPciInfo_t.
type PciInfo struct {
	Bus           uint32
	BusID         string
	Device        uint32
	Domain        uint32
	PCIDeviceID   uint32
	PCISubsystemID uint32
	MaxPayload    uint32
	MaxReadReq    uint32
	LinkGen       uint32
	LinkWidth     uint32
	MaxLinkGen    uint32
	MaxLinkWidth  uint32
	IommuGroup    int32
}

// Bar0Memory mirrors rblnmlBar0Memory_t.
type Bar0Memory struct {
	Free  uint64
	Used  uint64
	Total uint64
}

// ContextInfo mirrors rblnmlContextInfo_t.
type ContextInfo struct {
	TaskNum       uint32
	RsdCtxHandle  uint32
	CtxHandle     uint32
	Priority      uint32
	ASID          uint32
	DRAMPhysMem   uint64
	DoneSeq       uint64
	SubmittedSeq  uint64
	ReqSeq        uint64
	CoreUtilLast  uint32
	Flags         uint32
	NumBuf        uint32
	NumProc       uint32
	NumMain       uint32
	NumSub        uint32
	NumTask       uint32
	Error         uint32
	Destroyed     bool
	Guilty        bool
}
