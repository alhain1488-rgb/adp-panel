package servers

import (
	"strconv"
	"strings"
)

// Stats are host metrics collected over SSH.
type Stats struct {
	CPUPercent    float64
	MemPercent    float64
	MemUsedMB     float64
	MemTotalMB    float64
	DiskPercent   float64
	DiskUsedGB    float64
	DiskTotalGB   float64
	UptimeSeconds int64
}

// metricsCmd samples CPU twice and reads memory/disk/uptime in one shot.
const metricsCmd = `c1=$(head -n1 /proc/stat); sleep 1; c2=$(head -n1 /proc/stat); ` +
	`echo "CPU1 $c1"; echo "CPU2 $c2"; ` +
	`echo "MEM $(free -m | awk 'NR==2{print $3, $2}')"; ` +
	`echo "DISK $(df -k / | awk 'NR==2{print $3, $2}')"; ` +
	`echo "UPTIME $(cut -d. -f1 /proc/uptime)"`

func round1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}

// cpuTotals returns (idle+iowait, total) for a /proc/stat "cpu ..." line.
func cpuTotals(fields []string) (idle, total float64) {
	// fields: ["cpu", user, nice, system, idle, iowait, irq, softirq, ...]
	for i, f := range fields {
		if i == 0 {
			continue
		}
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			continue
		}
		total += v
		if i == 4 || i == 5 { // idle, iowait
			idle += v
		}
	}
	return idle, total
}

// parseStats parses the output of metricsCmd.
func parseStats(out string) Stats {
	var st Stats
	var cpu1, cpu2 []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "CPU1":
			cpu1 = f[1:]
		case "CPU2":
			cpu2 = f[1:]
		case "MEM":
			if len(f) >= 3 {
				used, _ := strconv.ParseFloat(f[1], 64)
				total, _ := strconv.ParseFloat(f[2], 64)
				st.MemUsedMB, st.MemTotalMB = used, total
				if total > 0 {
					st.MemPercent = round1(used / total * 100)
				}
			}
		case "DISK":
			if len(f) >= 3 {
				usedKB, _ := strconv.ParseFloat(f[1], 64)
				totalKB, _ := strconv.ParseFloat(f[2], 64)
				st.DiskUsedGB = round1(usedKB / 1024 / 1024)
				st.DiskTotalGB = round1(totalKB / 1024 / 1024)
				if totalKB > 0 {
					st.DiskPercent = round1(usedKB / totalKB * 100)
				}
			}
		case "UPTIME":
			if len(f) >= 2 {
				st.UptimeSeconds, _ = strconv.ParseInt(f[1], 10, 64)
			}
		}
	}
	if len(cpu1) > 4 && len(cpu2) > 4 {
		idle1, total1 := cpuTotals(cpu1)
		idle2, total2 := cpuTotals(cpu2)
		dTotal := total2 - total1
		dIdle := idle2 - idle1
		if dTotal > 0 {
			st.CPUPercent = round1((dTotal - dIdle) / dTotal * 100)
		}
	}
	return st
}
