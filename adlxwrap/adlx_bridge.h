#ifndef ADLX_BRIDGE_H
#define ADLX_BRIDGE_H

#include "ADLXHelper.h"
#include "IPerformanceMonitoring.h"

typedef struct {
	char  name[256];
	double gpuUsage;
	double gpuTemp;
	double gpuPower;
	int    vramUsedMB;
	int    vramTotalMB;
	double gpuPowerCap;
} ADLXGPUData;

int  adlx_init(void);
int  adlx_collect(ADLXGPUData* data);
void adlx_close(void);

#endif
