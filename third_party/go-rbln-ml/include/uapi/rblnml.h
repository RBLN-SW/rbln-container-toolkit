#ifndef __RBLNML_H_
#define __RBLNML_H_

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stddef.h>

/* Constants for buffer sizes */
#define RBLNML_VERSION_LENGTH		128
#define RBLNML_CARD_NAME_MAX_LEN	16
#define RBLNML_UUID_BUFFER_SIZE		38
#define RBLNML_DEVICE_PCI_BUS_ID_BUFFER_SIZE	32

/* device handle */
typedef uint64_t rblnmlDevice_t;

/* Return types */
typedef enum {
	RBLNML_SUCCESS = 0,			/* The operation was successful. */
	RBLNML_ERROR_UNINITIALIZED = 1,		/* rblnml was not initialized. */
	RBLNML_ERROR_INVALID_ARGUMENT = 2,	/* A supplied argument is invalid. */
	RBLNML_ERROR_NO_PERMISSION = 3,		/* No permission for operation. */
	RBLNML_ERROR_NOT_FOUND = 4,		/* A query to find an object was unsuccessful. */
	RBLNML_ERROR_IOCTL_FAILED = 5,		/* ioctl call failed. */
	RBLNML_ERROR_UNKNOWN = 999		/* An internal error occurred. */
} rblnmlReturn_t;

typedef struct rblnmlHwIpInfo_st {
	uint64_t sram_base_address;
	uint64_t dram_base_address;
	uint64_t dram_size;
	uint32_t sram_size;
	uint32_t device_id; /* PCI Device ID */
	uint32_t cpld_version;
	uint32_t chiplet_cnt;
	uint8_t cpucp_version[RBLNML_VERSION_LENGTH];
	uint8_t card_name[RBLNML_CARD_NAME_MAX_LEN];
	uint32_t rev_id;
	uint32_t drm_minor;
	uint32_t dcluster_cnt;
	uint64_t reserved2[4];
} rblnmlHwIpInfo_t;

typedef struct rblnmlDeviceInfo_st {
	char uuid[RBLNML_UUID_BUFFER_SIZE];
	char bus_id[RBLNML_DEVICE_PCI_BUS_ID_BUFFER_SIZE];	/* domain:bus:device:function */
	uint64_t serial_id;
	uint32_t board_info;
	uint32_t device_id;	/* rbln.. */
	uint32_t group_id;
	uint32_t numa_node;
} rblnmlDeviceInfo_t;

typedef struct rblnmlPciInfo_st {
	unsigned int bus;	/* the bus on which the device resides, 0 to 255 */
	char bus_id[RBLNML_DEVICE_PCI_BUS_ID_BUFFER_SIZE];	/* domain:bus:device:function */
	unsigned int device;	/* device id on the bus, 0 to 31 */
	unsigned int domain;	/* the domain on which the device resides, 0 to 0xffffffff */
	unsigned int pci_device_id;	/* PCI Device ID */
	unsigned int pci_subsystem_id;	/* PCI Subsystem ID */
	unsigned int max_payload;	/* maximum payload size in bytes */
	unsigned int max_readreq;	/* maximum read request size in bytes */
	unsigned int link_gen;		/* current PCIe link generation (enum pcie_link_gen) */
	unsigned int link_width;	/* current PCIe link width (number of lanes) */
	unsigned int max_link_gen;	/* maximum PCIe link generation (enum pcie_link_gen) */
	unsigned int max_link_width;	/* maximum PCIe link width (number of lanes) */
	int iommu_group;		/* IOMMU group ID, -1 if not available */
} rblnmlPciInfo_t;

typedef struct rblnmlVersion_st {
	char kernel_version[RBLNML_VERSION_LENGTH];
	char fw_version[RBLNML_VERSION_LENGTH];
	char smc_version[RBLNML_VERSION_LENGTH];
} rblnmlVersion_t;

typedef struct rblnmlBar0Memory_st {
	uint64_t bar0_free;
	uint64_t bar0_used;
	uint64_t bar0_total;
} rblnmlBar0Memory_t;

typedef struct rblnmlContextInfo_st {
	uint32_t task_num;
	uint32_t rsd_ctx_handle;
	uint32_t ctx_handle;
	uint32_t priority;
	uint32_t asid;
	uint64_t dram_phys_mem;
	uint64_t done_seq;
	uint64_t submitted_seq;
	uint64_t req_seq;
	uint32_t core_util_last;
	uint32_t flags;
	uint32_t num_buf;
	uint32_t num_proc;
	uint32_t num_main;
	uint32_t num_sub;
	uint32_t num_task;
	uint32_t error;
	uint8_t destroyed;
	uint8_t guilty;
} rblnmlContextInfo_t;

typedef uint32_t rblnmlTemperature_t;
typedef uint32_t rblnmlPower_t;
typedef uint32_t rblnmlPState_t;

/* API functions */

// Initialize rblnml library and open all /dev/rbln* devices
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlInit(void);

// Initialize rblnml library and open specified /dev/rbln* devices
// device_list: array of device numbers (e.g., [0, 1, 2] for rbln0, rbln1, rbln2)
// count: number of devices in the array
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlInitWithDevlist(const uint32_t *device_list, size_t count);

// Shutdown rblnml library and close all device handles
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlShutdown(void);

// Get the number of devices found during initialization
// count: pointer to store the count of devices
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetCount(unsigned int *count);

// Get device handle by index (0-based)
// device: pointer to store device handle (rblnmlDevice_t)
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetHandleByIndex(unsigned int index, rblnmlDevice_t *device);

// Get device handle by PCI bus ID (e.g., "0000:01:00.0")
// device: pointer to store device handle (rblnmlDevice_t)
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetHandleByPciBusId(const char *pci_bus_id, rblnmlDevice_t *device);

// Get device handle by UUID
// device: pointer to store device handle (rblnmlDevice_t)
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetHandleByUuid(const char *uuid, rblnmlDevice_t *device);

// Get device handle by Serial ID
// device: pointer to store device handle (rblnmlDevice_t)
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetHandleBySerialId(const char *serial_id, rblnmlDevice_t *device);

// Get device hardware IP information
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// info: pointer to rblnmlHwIpInfo_t structure to be filled
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetDeviceHWIPInfo(rblnmlDevice_t device, rblnmlHwIpInfo_t *info);

// Get device information (uuid, bus_id, serial_id, board_info, device_id, group_id, numa_node)
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// info: pointer to rblnmlDeviceInfo_t structure to be filled
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetDeviceInfo(rblnmlDevice_t device, rblnmlDeviceInfo_t *info);

// Get kernel version
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// buffer: pointer to buffer to store kernel version string
// size: size of the buffer
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetKernelVersion(rblnmlDevice_t device, char *buffer, size_t size);

// Get firmware version
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// buffer: pointer to buffer to store firmware version string
// size: size of the buffer
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetFwVersion(rblnmlDevice_t device, char *buffer, size_t size);

// Get SMC version
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// buffer: pointer to buffer to store SMC version string
// size: size of the buffer
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetSmcVersion(rblnmlDevice_t device, char *buffer, size_t size);

// Get PCI information
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// info: pointer to rblnmlPciInfo_t structure to be filled
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetPciInfo(rblnmlDevice_t device, rblnmlPciInfo_t *info);

// Get PCIe Maximum Payload Size (MPS)
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// mps: pointer to store MPS value in bytes
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetPcieMPS(rblnmlDevice_t device, unsigned int *mps);

// Get PCIe Maximum Read Request (MRR)
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// mrr: pointer to store MRR value in bytes
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetPcieMRR(rblnmlDevice_t device, unsigned int *mrr);

// Get PCIe Link Current Speed
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// speed: pointer to store current link speed in GB/s (calculated as GT/s * link_width / 8)
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetPcieLinkCurSpeed(rblnmlDevice_t device, double *speed);

// Get PCIe Link Current Width
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// width: pointer to store current link width (number of lanes)
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetPcieLinkCurWidth(rblnmlDevice_t device, unsigned int *width);

// Get PCIe Link Maximum Speed
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// speed: pointer to store maximum link speed in GB/s (calculated as GT/s * max_link_width / 8)
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetPcieLinkMaxSpeed(rblnmlDevice_t device, double *speed);

// Get PCIe Link Maximum Width
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// width: pointer to store maximum link width (number of lanes)
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlGetPcieLinkMaxWidth(rblnmlDevice_t device, unsigned int *width);

// Get BAR0 memory information
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// info: pointer to rblnmlBar0Memory_t structure to be filled
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetBAR0MemoryInfo(rblnmlDevice_t device, rblnmlBar0Memory_t *info);

// Get device temperature
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// temperature: pointer to rblnmlTemperature_t to store temperature value
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetTemperature(rblnmlDevice_t device, rblnmlTemperature_t *temperature);

// Get device power usage (uW)
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// power: pointer to rblnmlPower_t to store power usage value
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetPowerUsage(rblnmlDevice_t device, rblnmlPower_t *power);

// Get device power state
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// pstate: pointer to rblnmlPState_t to store power state value
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetPowerState(rblnmlDevice_t device, rblnmlPState_t *pstate);

// Get device utilization
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// utilization: pointer to uint32_t to store utilization value
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetUtilization(rblnmlDevice_t device, uint32_t *utilization);

// Get device context information
// device: device handle obtained from rblnmlDeviceGetHandleByIndex()
// info: pointer to array of rblnmlContextInfo_t structures to be filled
// count: size of the array (number of rblnmlContextInfo_t structures that can be stored in info)
// Returns RBLNML_SUCCESS on success, error code on failure
rblnmlReturn_t rblnmlDeviceGetContextInfo(rblnmlDevice_t device, rblnmlContextInfo_t *info,
					  size_t count);

// Create an RSD group and assign devices to it
// group_id: numeric ID of the RSD group to create
// device_list: array of NPU device indices to assign to the group
// count: number of entries in device_list
// Returns RBLNML_SUCCESS on success
// Returns RBLNML_ERROR_INVALID_ARGUMENT if device_list is NULL or count is 0
// Returns RBLNML_ERROR_NOT_FOUND if the RSD driver is not loaded (/dev/rsd absent)
// Returns RBLNML_ERROR_NO_PERMISSION if the sysfs write fails
rblnmlReturn_t rblnmlRsdGroupCreate(uint32_t group_id, const uint32_t *device_list,
				    size_t count);

// Destroy a single RSD group by its ID
// group_id: numeric ID of the RSD group to destroy
// Returns RBLNML_SUCCESS on success
// Returns RBLNML_ERROR_NOT_FOUND if the RSD driver is not loaded (/dev/rsd absent)
// Returns RBLNML_ERROR_NO_PERMISSION if the sysfs write fails
rblnmlReturn_t rblnmlRsdGroupDestroy(uint32_t group_id);

// Destroy all existing RSD groups
// Iterates group IDs 1..63 and destroys each one that exists in sysfs
// Returns RBLNML_SUCCESS if all groups were destroyed successfully
// Returns RBLNML_ERROR_NOT_FOUND if the RSD driver is not loaded (/dev/rsd absent)
// Returns RBLNML_ERROR_NO_PERMISSION if any sysfs write fails
rblnmlReturn_t rblnmlRsdGroupDestroyAll(void);

// Create one RSD group per NPU device found in the system.
// Device 0 is excluded; each device N (N >= 1) is assigned to group N.
// Destroys all existing groups before creating new ones.
// Returns RBLNML_SUCCESS on success, or if fewer than 2 devices are present.
// Returns RBLNML_ERROR_NOT_FOUND if the RSD driver is not loaded (/dev/rsd absent)
// Returns RBLNML_ERROR_NO_PERMISSION if any sysfs write fails
rblnmlReturn_t rblnmlRsdGroupCreateAll(void);

#ifdef __cplusplus
}
#endif

#endif /* __RBLNML_H_ */

