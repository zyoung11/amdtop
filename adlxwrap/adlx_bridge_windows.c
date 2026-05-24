#include "adlx_bridge.h"
#include <string.h>

static IADLXSystem*                    sys   = NULL;
static IADLXGPU*                       gpu   = NULL;
static IADLXPerformanceMonitoringServices* perf = NULL;

int adlx_init(void) {
	ADLX_RESULT res = ADLXHelper_Initialize();
	if (ADLX_FAILED(res)) return -1;

	sys = ADLXHelper_GetSystemServices();
	if (!sys) { ADLXHelper_Terminate(); return -1; }

	IADLXGPUList* gpus = NULL;
	res = sys->pVtbl->GetGPUs(sys, &gpus);
	if (ADLX_FAILED(res) || !gpus) { ADLXHelper_Terminate(); return -1; }

	res = gpus->pVtbl->At_GPUList(gpus, 0, &gpu);
	gpus->pVtbl->Release(gpus);
	if (ADLX_FAILED(res) || !gpu) { ADLXHelper_Terminate(); return -1; }

	res = sys->pVtbl->GetPerformanceMonitoringServices(sys, &perf);
	if (ADLX_FAILED(res) || !perf) {
		gpu->pVtbl->Release(gpu); gpu = NULL;
		ADLXHelper_Terminate();
		return -1;
	}
	return 0;
}

int adlx_collect(ADLXGPUData* data) {
	memset(data, 0, sizeof(ADLXGPUData));

	const char* name = NULL;
	if (ADLX_SUCCEEDED(gpu->pVtbl->Name(gpu, &name)) && name) {
		strncpy(data->name, name, sizeof(data->name) - 1);
	}

	adlx_uint vramSize = 0;
	gpu->pVtbl->TotalVRAM(gpu, &vramSize);
	data->vramTotalMB = (int)vramSize;

	IADLXGPUMetrics* metrics = NULL;
	ADLX_RESULT res = perf->pVtbl->GetCurrentGPUMetrics(perf, gpu, &metrics);
	if (ADLX_FAILED(res) || !metrics) return -1;

	adlx_double val;
	if (ADLX_SUCCEEDED(metrics->pVtbl->GPUUsage(metrics, &val)))
		data->gpuUsage = val;

	/* Hotspot temperature, fall back to edge temperature */
	if (ADLX_FAILED(metrics->pVtbl->GPUHotspotTemperature(metrics, &val)))
		metrics->pVtbl->GPUTemperature(metrics, &val);
	data->gpuTemp = val;

	/* Total board power, fall back to GPU chip power */
	if (ADLX_FAILED(metrics->pVtbl->GPUTotalBoardPower(metrics, &val)))
		metrics->pVtbl->GPUPower(metrics, &val);
	data->gpuPower = val;

	adlx_int vram;
	if (ADLX_SUCCEEDED(metrics->pVtbl->GPUVRAM(metrics, &vram)))
		data->vramUsedMB = vram;

	metrics->pVtbl->Release(metrics);

	IADLXGPUMetricsSupport* support = NULL;
	if (ADLX_SUCCEEDED(perf->pVtbl->GetSupportedGPUMetrics(perf, gpu, &support))) {
		adlx_int minP = 0, maxP = 0;
		/* Total board power cap, fall back to GPU chip power cap */
		if (ADLX_FAILED(support->pVtbl->GetGPUTotalBoardPowerRange(support, &minP, &maxP)))
			support->pVtbl->GetGPUPowerRange(support, &minP, &maxP);
		data->gpuPowerCap = (double)maxP;
		support->pVtbl->Release(support);
	}
	return 0;
}

void adlx_close(void) {
	if (perf) { perf->pVtbl->Release(perf); perf = NULL; }
	if (gpu)  { gpu->pVtbl->Release(gpu);  gpu = NULL; }
	sys = NULL;
	ADLXHelper_Terminate();
}
