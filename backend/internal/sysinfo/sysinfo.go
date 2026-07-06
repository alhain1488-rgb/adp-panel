// Package sysinfo reports host-level resource metrics (CPU load, memory, swap,
// disk, uptime) for the machine the panel runs on. Values come from /proc and
// statfs; on Linux those /proc files reflect the host kernel even when the panel
// runs inside a container, so the numbers describe the VPS, not the container.
package sysinfo

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// Info is a point-in-time snapshot of host resource usage.
type Info struct {
	Kernel   string `json:"kernel"`
	Arch     string `json:"arch"`
	CPUCores int    `json:"cpu_cores"`

	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`

	MemTotalBytes uint64 `json:"mem_total_bytes"`
	MemUsedBytes  uint64 `json:"mem_used_bytes"`
	MemAvailBytes uint64 `json:"mem_available_bytes"`

	SwapTotalBytes uint64 `json:"swap_total_bytes"`
	SwapUsedBytes  uint64 `json:"swap_used_bytes"`

	DiskTotalBytes uint64 `json:"disk_total_bytes"`
	DiskUsedBytes  uint64 `json:"disk_used_bytes"`
	DiskFreeBytes  uint64 `json:"disk_free_bytes"`

	UptimeSeconds int64 `json:"uptime_seconds"`
}

// Collect gathers the current host metrics. Fields that can't be read (e.g. a
// missing /proc on non-Linux dev machines) are left at their zero value.
func Collect() Info {
	in := Info{
		Arch:     runtime.GOARCH,
		CPUCores: runtime.NumCPU(),
		Kernel:   strings.TrimSpace(readFile("/proc/sys/kernel/osrelease")),
	}
	in.Load1, in.Load5, in.Load15 = parseLoadAvg(readFile("/proc/loadavg"))
	in.MemTotalBytes, in.MemAvailBytes, in.SwapTotalBytes, in.SwapUsedBytes = parseMemInfo(readFile("/proc/meminfo"))
	if in.MemTotalBytes >= in.MemAvailBytes {
		in.MemUsedBytes = in.MemTotalBytes - in.MemAvailBytes
	}
	in.UptimeSeconds = parseUptime(readFile("/proc/uptime"))
	in.DiskTotalBytes, in.DiskFreeBytes = diskUsage("/")
	if in.DiskTotalBytes >= in.DiskFreeBytes {
		in.DiskUsedBytes = in.DiskTotalBytes - in.DiskFreeBytes
	}
	return in
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// parseLoadAvg reads the first three fields of /proc/loadavg.
func parseLoadAvg(s string) (l1, l5, l15 float64) {
	f := strings.Fields(s)
	if len(f) < 3 {
		return 0, 0, 0
	}
	l1, _ = strconv.ParseFloat(f[0], 64)
	l5, _ = strconv.ParseFloat(f[1], 64)
	l15, _ = strconv.ParseFloat(f[2], 64)
	return l1, l5, l15
}

// parseMemInfo pulls MemTotal, MemAvailable, SwapTotal and SwapFree (all in kB in
// /proc/meminfo) and returns them as bytes, with swap used derived.
func parseMemInfo(s string) (memTotal, memAvail, swapTotal, swapUsed uint64) {
	var swapFree uint64
	haveSwapFree := false
	for _, line := range strings.Split(s, "\n") {
		key, valKB, ok := memInfoLine(line)
		if !ok {
			continue
		}
		switch key {
		case "MemTotal":
			memTotal = valKB * 1024
		case "MemAvailable":
			memAvail = valKB * 1024
		case "SwapTotal":
			swapTotal = valKB * 1024
		case "SwapFree":
			swapFree = valKB * 1024
			haveSwapFree = true
		}
	}
	if haveSwapFree && swapTotal >= swapFree {
		swapUsed = swapTotal - swapFree
	}
	return memTotal, memAvail, swapTotal, swapUsed
}

// memInfoLine parses "Key:   1234 kB" into (key, valueInKB).
func memInfoLine(line string) (string, uint64, bool) {
	key, rest, ok := strings.Cut(line, ":")
	if !ok {
		return "", 0, false
	}
	f := strings.Fields(rest)
	if len(f) == 0 {
		return "", 0, false
	}
	v, err := strconv.ParseUint(f[0], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return strings.TrimSpace(key), v, true
}

// parseUptime reads the first field of /proc/uptime (seconds since boot).
func parseUptime(s string) int64 {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0
	}
	secs, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0
	}
	return int64(secs)
}

// diskUsage returns the total and available bytes of the filesystem at path.
func diskUsage(path string) (total, free uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	bsize := uint64(st.Bsize) //nolint:unconvert // Bsize is int64 on Linux, uint32 on darwin
	return st.Blocks * bsize, st.Bavail * bsize
}
