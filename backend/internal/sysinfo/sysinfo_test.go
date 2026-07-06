package sysinfo

import "testing"

func TestParseLoadAvg(t *testing.T) {
	l1, l5, l15 := parseLoadAvg("0.02 0.07 0.27 1/123 4567\n")
	if l1 != 0.02 || l5 != 0.07 || l15 != 0.27 {
		t.Fatalf("load = %v %v %v", l1, l5, l15)
	}
	if a, b, c := parseLoadAvg("garbage"); a != 0 || b != 0 || c != 0 {
		t.Fatalf("expected zeros for bad input, got %v %v %v", a, b, c)
	}
}

func TestParseMemInfo(t *testing.T) {
	sample := `MemTotal:         980432 kB
MemFree:          123456 kB
MemAvailable:     555000 kB
Buffers:           10000 kB
SwapTotal:       2097148 kB
SwapFree:        1990000 kB
`
	total, avail, swapTotal, swapUsed := parseMemInfo(sample)
	if total != 980432*1024 {
		t.Fatalf("mem total = %d", total)
	}
	if avail != 555000*1024 {
		t.Fatalf("mem avail = %d", avail)
	}
	if swapTotal != 2097148*1024 {
		t.Fatalf("swap total = %d", swapTotal)
	}
	if swapUsed != (2097148-1990000)*1024 {
		t.Fatalf("swap used = %d", swapUsed)
	}
}

func TestParseUptime(t *testing.T) {
	if got := parseUptime("356421.42 1234567.00\n"); got != 356421 {
		t.Fatalf("uptime = %d", got)
	}
	if got := parseUptime(""); got != 0 {
		t.Fatalf("empty uptime = %d", got)
	}
}

func TestCollect_Sane(t *testing.T) {
	in := Collect()
	if in.CPUCores < 1 {
		t.Fatalf("cpu cores = %d", in.CPUCores)
	}
	if in.Arch == "" {
		t.Fatal("arch is empty")
	}
	// Disk should be readable on any test host (Linux or macOS CI).
	if in.DiskTotalBytes == 0 {
		t.Fatal("disk total is zero")
	}
	if in.MemUsedBytes > in.MemTotalBytes {
		t.Fatalf("mem used %d exceeds total %d", in.MemUsedBytes, in.MemTotalBytes)
	}
}
