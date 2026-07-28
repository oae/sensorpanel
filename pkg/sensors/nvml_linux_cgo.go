//go:build linux && cgo

package sensors

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef void *sp_nvml_device;
typedef int (*sp_init_fn)(void);
typedef int (*sp_handle_fn)(unsigned int, sp_nvml_device *);
typedef int (*sp_name_fn)(sp_nvml_device, char *, unsigned int);
typedef int (*sp_uint_fn)(sp_nvml_device, unsigned int *);
typedef int (*sp_temp_fn)(sp_nvml_device, unsigned int, unsigned int *);
typedef int (*sp_clock_fn)(sp_nvml_device, unsigned int, unsigned int *);

typedef struct {
  unsigned int gpu;
  unsigned int memory;
} sp_nvml_utilization;
typedef int (*sp_util_fn)(sp_nvml_device, sp_nvml_utilization *);

typedef struct {
  unsigned long long total;
  unsigned long long free;
  unsigned long long used;
} sp_nvml_memory;
typedef int (*sp_memory_fn)(sp_nvml_device, sp_nvml_memory *);

typedef struct {
  char name[96];
  unsigned int temperature;
  unsigned int load;
  unsigned long long memory_used;
  unsigned long long memory_total;
  unsigned int power_mw;
  unsigned int fan;
  unsigned int graphics_clock;
  unsigned int memory_clock;
  unsigned int mask;
} sp_nvml_result;

static struct {
  int attempted;
  int ready;
  void *library;
  sp_handle_fn handle;
  sp_name_fn name;
  sp_temp_fn temperature;
  sp_util_fn utilization;
  sp_memory_fn memory;
  sp_uint_fn power;
  sp_uint_fn fan;
  sp_clock_fn clock;
} sp_nvml;

static int sp_nvml_load(void) {
  if (sp_nvml.attempted) return sp_nvml.ready;
  sp_nvml.attempted = 1;
  sp_nvml.library = dlopen("libnvidia-ml.so.1", RTLD_LAZY | RTLD_LOCAL);
  if (!sp_nvml.library) return 0;
  sp_init_fn init = (sp_init_fn)dlsym(sp_nvml.library, "nvmlInit_v2");
  sp_nvml.handle = (sp_handle_fn)dlsym(sp_nvml.library, "nvmlDeviceGetHandleByIndex_v2");
  sp_nvml.name = (sp_name_fn)dlsym(sp_nvml.library, "nvmlDeviceGetName");
  sp_nvml.temperature = (sp_temp_fn)dlsym(sp_nvml.library, "nvmlDeviceGetTemperature");
  sp_nvml.utilization = (sp_util_fn)dlsym(sp_nvml.library, "nvmlDeviceGetUtilizationRates");
  sp_nvml.memory = (sp_memory_fn)dlsym(sp_nvml.library, "nvmlDeviceGetMemoryInfo");
  sp_nvml.power = (sp_uint_fn)dlsym(sp_nvml.library, "nvmlDeviceGetPowerUsage");
  sp_nvml.fan = (sp_uint_fn)dlsym(sp_nvml.library, "nvmlDeviceGetFanSpeed");
  sp_nvml.clock = (sp_clock_fn)dlsym(sp_nvml.library, "nvmlDeviceGetClockInfo");
  if (!init || !sp_nvml.handle || init() != 0) return 0;
  sp_nvml.ready = 1;
  return 1;
}

static int sp_nvml_query(sp_nvml_result *result) {
  memset(result, 0, sizeof(*result));
  if (!sp_nvml_load()) return -1;
  sp_nvml_device device = NULL;
  if (sp_nvml.handle(0, &device) != 0 || !device) return -1;
  if (sp_nvml.name && sp_nvml.name(device, result->name, sizeof(result->name)) == 0)
    result->mask |= 1u << 0;
  if (sp_nvml.temperature && sp_nvml.temperature(device, 0, &result->temperature) == 0)
    result->mask |= 1u << 1;
  sp_nvml_utilization utilization;
  if (sp_nvml.utilization && sp_nvml.utilization(device, &utilization) == 0) {
    result->load = utilization.gpu;
    result->mask |= 1u << 2;
  }
  sp_nvml_memory memory;
  if (sp_nvml.memory && sp_nvml.memory(device, &memory) == 0) {
    result->memory_used = memory.used;
    result->memory_total = memory.total;
    result->mask |= 1u << 3;
  }
  if (sp_nvml.power && sp_nvml.power(device, &result->power_mw) == 0)
    result->mask |= 1u << 4;
  if (sp_nvml.fan && sp_nvml.fan(device, &result->fan) == 0)
    result->mask |= 1u << 5;
  if (sp_nvml.clock && sp_nvml.clock(device, 0, &result->graphics_clock) == 0)
    result->mask |= 1u << 6;
  if (sp_nvml.clock && sp_nvml.clock(device, 2, &result->memory_clock) == 0)
    result->mask |= 1u << 7;
  return 0;
}
*/
import "C"

import (
	"fmt"
	"strings"
)

func nvmlAvailable() bool {
	return C.sp_nvml_load() != 0
}

func queryNVML() (map[string]interface{}, error) {
	var result C.sp_nvml_result
	if C.sp_nvml_query(&result) != 0 {
		return nil, fmt.Errorf("NVML query failed")
	}
	mask := uint32(result.mask)
	data := make(map[string]interface{}, 9)
	if mask&(1<<0) != 0 {
		data["name"] = strings.TrimSpace(C.GoString(&result.name[0]))
	}
	if mask&(1<<1) != 0 {
		data["temperature"] = float64(result.temperature)
	}
	if mask&(1<<2) != 0 {
		data["load"] = float64(result.load)
	}
	if mask&(1<<3) != 0 {
		data["memory_used"] = float64(result.memory_used) / (1024 * 1024)
		data["memory_total"] = float64(result.memory_total) / (1024 * 1024)
	}
	if mask&(1<<4) != 0 {
		data["power"] = float64(result.power_mw) / 1000
	}
	if mask&(1<<5) != 0 {
		data["fan_speed"] = float64(result.fan)
	}
	if mask&(1<<6) != 0 {
		data["clock"] = float64(result.graphics_clock)
	}
	if mask&(1<<7) != 0 {
		data["memory_clock"] = float64(result.memory_clock)
	}
	return data, nil
}
