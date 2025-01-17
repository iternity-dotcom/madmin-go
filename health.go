//
// MinIO Object Storage (c) 2021 MinIO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package madmin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"runtime"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
)

const (
	// HealthInfoVersion0 is version 0
	HealthInfoVersion0 = ""
	// HealthInfoVersion1 is version 1
	HealthInfoVersion1 = "1"
	// HealthInfoVersion is current health info version.
	HealthInfoVersion = HealthInfoVersion1
)

// CPU contains system's CPU information.
type CPU struct {
	VendorID   string   `json:"vendor_id"`
	Family     string   `json:"family"`
	Model      string   `json:"model"`
	Stepping   int32    `json:"stepping"`
	PhysicalID string   `json:"physical_id"`
	ModelName  string   `json:"model_name"`
	Mhz        float64  `json:"mhz"`
	CacheSize  int32    `json:"cache_size"`
	Flags      []string `json:"flags"`
	Microcode  string   `json:"microcode"`
	Cores      int      `json:"cores"` // computed
}

// CPUs contains all CPU information of a node.
type CPUs struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`

	CPUs []CPU `json:"cpus,omitempty"`
}

// GetCPUs returns system's all CPU information.
func GetCPUs(ctx context.Context, addr string) CPUs {
	infos, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return CPUs{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	cpuMap := map[string][]cpu.InfoStat{}
	for _, info := range infos {
		infoStats, found := cpuMap[info.PhysicalID]
		if !found {
			infoStats = []cpu.InfoStat{}
		}
		cpuMap[info.PhysicalID] = append(infoStats, info)
	}

	cpus := []CPU{}
	for _, infoStats := range cpuMap {
		cpus = append(cpus, CPU{
			VendorID:   infoStats[0].VendorID,
			Family:     infoStats[0].Family,
			Model:      infoStats[0].Model,
			Stepping:   infoStats[0].Stepping,
			PhysicalID: infoStats[0].PhysicalID,
			ModelName:  infoStats[0].ModelName,
			Mhz:        infoStats[0].Mhz,
			CacheSize:  infoStats[0].CacheSize,
			Flags:      infoStats[0].Flags,
			Microcode:  infoStats[0].Microcode,
			Cores:      len(infoStats),
		})
	}

	return CPUs{
		Addr: addr,
		CPUs: cpus,
	}
}

// Partition contains disk partition's information.
type Partition struct {
	Error string `json:"error,omitempty"`

	Device       string `json:"device,omitempty"`
	Mountpoint   string `json:"mountpoint,omitempty"`
	FSType       string `json:"fs_type,omitempty"`
	MountOptions string `json:"mount_options,omitempty"`
	MountFSType  string `json:"mount_fs_type,omitempty"`
	SpaceTotal   uint64 `json:"space_total,omitempty"`
	SpaceFree    uint64 `json:"space_free,omitempty"`
	InodeTotal   uint64 `json:"inode_total,omitempty"`
	InodeFree    uint64 `json:"inode_free,omitempty"`
}

// Partitions contains all disk partitions information of a node.
type Partitions struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`

	Partitions []Partition `json:"partitions,omitempty"`
}

// GetPartitions returns all disk partitions information of a node running linux only operating system.
func GetPartitions(ctx context.Context, addr string) Partitions {
	if runtime.GOOS != "linux" {
		return Partitions{
			Addr:  addr,
			Error: "unsupported operating system " + runtime.GOOS,
		}
	}

	parts, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return Partitions{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	partitions := []Partition{}

	for i := range parts {
		usage, err := disk.UsageWithContext(ctx, parts[i].Mountpoint)
		if err != nil {
			partitions = append(partitions, Partition{
				Device: parts[i].Device,
				Error:  err.Error(),
			})
		} else {
			partitions = append(partitions, Partition{
				Device:       parts[i].Device,
				Mountpoint:   parts[i].Mountpoint,
				FSType:       parts[i].Fstype,
				MountOptions: parts[i].Opts,
				MountFSType:  usage.Fstype,
				SpaceTotal:   usage.Total,
				SpaceFree:    usage.Free,
				InodeTotal:   usage.InodesTotal,
				InodeFree:    usage.InodesFree,
			})
		}
	}

	return Partitions{
		Addr:       addr,
		Partitions: partitions,
	}
}

// OSInfo contains operating system's information.
type OSInfo struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`

	Info    host.InfoStat          `json:"info,omitempty"`
	Sensors []host.TemperatureStat `json:"sensors,omitempty"`
}

// GetOSInfo returns linux only operating system's information.
func GetOSInfo(ctx context.Context, addr string) OSInfo {
	if runtime.GOOS != "linux" {
		return OSInfo{
			Addr:  addr,
			Error: "unsupported operating system " + runtime.GOOS,
		}
	}

	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return OSInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	sensors, err := host.SensorsTemperaturesWithContext(ctx)
	if err != nil {
		if _, isWarningErr := err.(*host.Warnings); !isWarningErr {
			return OSInfo{
				Addr:  addr,
				Error: err.Error(),
			}
		}
	}

	return OSInfo{
		Addr:    addr,
		Info:    *info,
		Sensors: sensors,
	}
}

// MemInfo contains system's RAM and swap information.
type MemInfo struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`

	Total          uint64 `json:"total,omitempty"`
	Available      uint64 `json:"available,omitempty"`
	SwapSpaceTotal uint64 `json:"swap_space_total,omitempty"`
	SwapSpaceFree  uint64 `json:"swap_space_free,omitempty"`
}

// GetMemInfo returns system's RAM and swap information.
func GetMemInfo(ctx context.Context, addr string) MemInfo {
	meminfo, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return MemInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	swapinfo, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		return MemInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	return MemInfo{
		Addr:           addr,
		Total:          meminfo.Total,
		Available:      meminfo.Available,
		SwapSpaceTotal: swapinfo.Total,
		SwapSpaceFree:  swapinfo.Free,
	}
}

// ProcInfo contains current process's information.
type ProcInfo struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`

	PID            int32                      `json:"pid,omitempty"`
	IsBackground   bool                       `json:"is_background,omitempty"`
	CPUPercent     float64                    `json:"cpu_percent,omitempty"`
	ChildrenPIDs   []int32                    `json:"children_pids,omitempty"`
	CmdLine        string                     `json:"cmd_line,omitempty"`
	NumConnections int                        `json:"num_connections,omitempty"`
	CreateTime     int64                      `json:"create_time,omitempty"`
	CWD            string                     `json:"cwd,omitempty"`
	ExecPath       string                     `json:"exec_path,omitempty"`
	GIDs           []int32                    `json:"gids,omitempty"`
	IOCounters     process.IOCountersStat     `json:"iocounters,omitempty"`
	NetIOCounters  []net.IOCountersStat       `json:"net_iocounters,omitempty"`
	IsRunning      bool                       `json:"is_running,omitempty"`
	MemInfo        process.MemoryInfoStat     `json:"mem_info,omitempty"`
	MemMaps        []process.MemoryMapsStat   `json:"mem_maps,omitempty"`
	MemPercent     float32                    `json:"mem_percent,omitempty"`
	Name           string                     `json:"name,omitempty"`
	Nice           int32                      `json:"nice,omitempty"`
	NumCtxSwitches process.NumCtxSwitchesStat `json:"num_ctx_switches,omitempty"`
	NumFDs         int32                      `json:"num_fds,omitempty"`
	NumThreads     int32                      `json:"num_threads,omitempty"`
	PageFaults     process.PageFaultsStat     `json:"page_faults,omitempty"`
	PPID           int32                      `json:"ppid,omitempty"`
	Status         string                     `json:"status,omitempty"`
	TGID           int32                      `json:"tgid,omitempty"`
	Times          cpu.TimesStat              `json:"times,omitempty"`
	UIDs           []int32                    `json:"uids,omitempty"`
	Username       string                     `json:"username,omitempty"`
	Rlimit         []process.RlimitStat       `json:"rlimit,omitempty"`
}

// GetProcInfo returns current MinIO process information.
func GetProcInfo(ctx context.Context, addr string) ProcInfo {
	pid := int32(syscall.Getpid())
	proc, err := process.NewProcess(pid)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	isBackground, err := proc.BackgroundWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	cpuPercent, err := proc.CPUPercentWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	children, _ := proc.ChildrenWithContext(ctx)
	childrenPIDs := []int32{}
	for i := range children {
		childrenPIDs = append(childrenPIDs, children[i].Pid)
	}

	cmdLine, err := proc.CmdlineWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	connections, err := proc.ConnectionsWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}
	numConnections := len(connections)

	createTime, err := proc.CreateTimeWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	cwd, err := proc.CwdWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	execPath, err := proc.ExeWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	gids, err := proc.GidsWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	ioCounters, err := proc.IOCountersWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	netIOCounters, err := proc.NetIOCountersWithContext(ctx, true)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	isRunning, err := proc.IsRunningWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	memInfo, err := proc.MemoryInfoWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	memMaps, err := proc.MemoryMapsWithContext(ctx, true)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	memPercent, err := proc.MemoryPercentWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	name, err := proc.NameWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	nice, err := proc.NiceWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	numCtxSwitches, err := proc.NumCtxSwitchesWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	numFDs, err := proc.NumFDsWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	numThreads, err := proc.NumThreadsWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	pageFaults, err := proc.PageFaultsWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	ppid, _ := proc.PpidWithContext(ctx)

	status, err := proc.StatusWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	tgid, err := proc.Tgid()
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	times, err := proc.TimesWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	uids, err := proc.UidsWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	username, err := proc.UsernameWithContext(ctx)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	rlimit, err := proc.RlimitUsageWithContext(ctx, true)
	if err != nil {
		return ProcInfo{
			Addr:  addr,
			Error: err.Error(),
		}
	}

	return ProcInfo{
		Addr:           addr,
		PID:            pid,
		IsBackground:   isBackground,
		CPUPercent:     cpuPercent,
		ChildrenPIDs:   childrenPIDs,
		CmdLine:        cmdLine,
		NumConnections: numConnections,
		CreateTime:     createTime,
		CWD:            cwd,
		ExecPath:       execPath,
		GIDs:           gids,
		IOCounters:     *ioCounters,
		NetIOCounters:  netIOCounters,
		IsRunning:      isRunning,
		MemInfo:        *memInfo,
		MemMaps:        *memMaps,
		MemPercent:     memPercent,
		Name:           name,
		Nice:           nice,
		NumCtxSwitches: *numCtxSwitches,
		NumFDs:         numFDs,
		NumThreads:     numThreads,
		PageFaults:     *pageFaults,
		PPID:           ppid,
		Status:         status,
		TGID:           tgid,
		Times:          *times,
		UIDs:           uids,
		Username:       username,
		Rlimit:         rlimit,
	}
}

// SysInfo - Includes hardware and system information of the MinIO cluster
type SysInfo struct {
	CPUInfo    []CPUs       `json:"cpus,omitempty"`
	Partitions []Partitions `json:"partitions,omitempty"`
	OSInfo     []OSInfo     `json:"osinfo,omitempty"`
	MemInfo    []MemInfo    `json:"meminfo,omitempty"`
	ProcInfo   []ProcInfo   `json:"procinfo,omitempty"`
}

// Latency contains write operation latency in seconds of a disk drive.
type Latency struct {
	Avg          float64 `json:"avg"`
	Max          float64 `json:"max"`
	Min          float64 `json:"min"`
	Percentile50 float64 `json:"percentile_50"`
	Percentile90 float64 `json:"percentile_90"`
	Percentile99 float64 `json:"percentile_99"`
}

// Throughput contains write performance in bytes per second of a disk drive.
type Throughput struct {
	Avg          uint64 `json:"avg"`
	Max          uint64 `json:"max"`
	Min          uint64 `json:"min"`
	Percentile50 uint64 `json:"percentile_50"`
	Percentile90 uint64 `json:"percentile_90"`
	Percentile99 uint64 `json:"percentile_99"`
}

// DrivePerfInfo contains disk drive's performance information.
type DrivePerfInfo struct {
	Error string `json:"error,omitempty"`

	Path       string     `json:"path"`
	Latency    Latency    `json:"latency,omitempty"`
	Throughput Throughput `json:"throughput,omitempty"`
}

// DrivePerfInfos contains all disk drive's performance information of a node.
type DrivePerfInfos struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`

	SerialPerf   []DrivePerfInfo `json:"serial_perf,omitempty"`
	ParallelPerf []DrivePerfInfo `json:"parallel_perf,omitempty"`
}

// PeerNetPerfInfo contains network performance information of a node.
type PeerNetPerfInfo struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`

	Latency    Latency    `json:"latency,omitempty"`
	Throughput Throughput `json:"throughput,omitempty"`
}

// NetPerfInfo contains network performance information of a node to other nodes.
type NetPerfInfo struct {
	Addr  string `json:"addr"`
	Error string `json:"error,omitempty"`

	RemotePeers []PeerNetPerfInfo `json:"remote_peers,omitempty"`
}

// PerfInfo - Includes Drive and Net perf info for the entire MinIO cluster
type PerfInfo struct {
	Drives      []DrivePerfInfos `json:"drives,omitempty"`
	Net         []NetPerfInfo    `json:"net,omitempty"`
	NetParallel NetPerfInfo      `json:"net_parallel,omitempty"`
}

// MinioConfig contains minio configuration of a node.
type MinioConfig struct {
	Error string `json:"error,omitempty"`

	Config interface{} `json:"config,omitempty"`
}

// MinioHealthInfo - Includes MinIO confifuration information
type MinioHealthInfo struct {
	Error string `json:"error,omitempty"`

	Config MinioConfig `json:"config,omitempty"`
	Info   InfoMessage `json:"info,omitempty"`
}

// HealthInfo - MinIO cluster's health Info
type HealthInfo struct {
	Version string `json:"version"`
	Error   string `json:"error,omitempty"`

	TimeStamp time.Time       `json:"timestamp,omitempty"`
	Sys       SysInfo         `json:"sys,omitempty"`
	Perf      PerfInfo        `json:"perf,omitempty"`
	Minio     MinioHealthInfo `json:"minio,omitempty"`
}

func (info HealthInfo) String() string {
	data, err := json.Marshal(info)
	if err != nil {
		panic(err) // This never happens.
	}
	return string(data)
}

// JSON returns this structure as JSON formatted string.
func (info HealthInfo) JSON() string {
	data, err := json.MarshalIndent(info, " ", "    ")
	if err != nil {
		panic(err) // This never happens.
	}
	return string(data)
}

// HealthDataType - Typed Health data types
type HealthDataType string

// HealthDataTypes
const (
	HealthDataTypePerfDrive   HealthDataType = "perfdrive"
	HealthDataTypePerfNet     HealthDataType = "perfnet"
	HealthDataTypeMinioInfo   HealthDataType = "minioinfo"
	HealthDataTypeMinioConfig HealthDataType = "minioconfig"
	HealthDataTypeSysCPU      HealthDataType = "syscpu"
	HealthDataTypeSysDriveHw  HealthDataType = "sysdrivehw"
	HealthDataTypeSysDocker   HealthDataType = "sysdocker" // is this really needed?
	HealthDataTypeSysOsInfo   HealthDataType = "sysosinfo"
	HealthDataTypeSysLoad     HealthDataType = "sysload" // provides very little info. Making it TBD
	HealthDataTypeSysMem      HealthDataType = "sysmem"
	HealthDataTypeSysNet      HealthDataType = "sysnet"
	HealthDataTypeSysProcess  HealthDataType = "sysprocess"
)

// HealthDataTypesMap - Map of Health datatypes
var HealthDataTypesMap = map[string]HealthDataType{
	"perfdrive":   HealthDataTypePerfDrive,
	"perfnet":     HealthDataTypePerfNet,
	"minioinfo":   HealthDataTypeMinioInfo,
	"minioconfig": HealthDataTypeMinioConfig,
	"syscpu":      HealthDataTypeSysCPU,
	"sysdrivehw":  HealthDataTypeSysDriveHw,
	"sysdocker":   HealthDataTypeSysDocker,
	"sysosinfo":   HealthDataTypeSysOsInfo,
	"sysload":     HealthDataTypeSysLoad,
	"sysmem":      HealthDataTypeSysMem,
	"sysnet":      HealthDataTypeSysNet,
	"sysprocess":  HealthDataTypeSysProcess,
}

// HealthDataTypesList - List of Health datatypes
var HealthDataTypesList = []HealthDataType{
	HealthDataTypePerfDrive,
	HealthDataTypePerfNet,
	HealthDataTypeMinioInfo,
	HealthDataTypeMinioConfig,
	HealthDataTypeSysCPU,
	HealthDataTypeSysDriveHw,
	HealthDataTypeSysDocker,
	HealthDataTypeSysOsInfo,
	HealthDataTypeSysLoad,
	HealthDataTypeSysMem,
	HealthDataTypeSysNet,
	HealthDataTypeSysProcess,
}

type healthInfoVersion struct {
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ServerHealthInfo - Connect to a minio server and call Health Info Management API
// to fetch server's information represented by HealthInfo structure
func (adm *AdminClient) ServerHealthInfo(ctx context.Context, types []HealthDataType, deadline time.Duration) (*http.Response, string, error) {
	v := url.Values{}
	v.Set("deadline", deadline.Truncate(1*time.Second).String())
	for _, d := range HealthDataTypesList { // Init all parameters to false.
		v.Set(string(d), "false")
	}
	for _, d := range types {
		v.Set(string(d), "true")
	}

	resp, err := adm.executeMethod(
		ctx, "GET", requestData{
			relPath:     adminAPIPrefix + "/healthinfo",
			queryValues: v,
		},
	)

	if err != nil {
		closeResponse(resp)
		return nil, "", err
	}

	if resp.StatusCode != http.StatusOK {
		closeResponse(resp)
		return nil, "", httpRespToErrorResponse(resp)
	}

	decoder := json.NewDecoder(resp.Body)
	var version healthInfoVersion
	if err = decoder.Decode(&version); err != nil {
		closeResponse(resp)
		return nil, "", err
	}

	if version.Error != "" {
		closeResponse(resp)
		return nil, "", errors.New(version.Error)
	}

	switch version.Version {
	case "", HealthInfoVersion:
	default:
		closeResponse(resp)
		return nil, "", errors.New("Upgrade Minio Client to support health info version " + version.Version)
	}

	return resp, version.Version, nil
}
