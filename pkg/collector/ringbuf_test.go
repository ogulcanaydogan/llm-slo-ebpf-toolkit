package collector

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestDecodeBPFEvent(t *testing.T) {
	orig := bpfEvent{
		PID:          1234,
		TID:          1235,
		TimestampNS:  9999999999,
		SignalType:    signalTypeDNSLatency,
		ValueNS:      5000000, // 5ms
		ConnSrcPort:  42424,
		ConnDstPort:  53,
		ConnDstIP:    0x0100007F, // 127.0.0.1
		ErrnoVal:     0,
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, orig); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := decodeBPFEvent(buf.Bytes())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.PID != orig.PID {
		t.Errorf("pid: got %d, want %d", decoded.PID, orig.PID)
	}
	if decoded.SignalType != signalTypeDNSLatency {
		t.Errorf("signal_type: got %d, want %d", decoded.SignalType, signalTypeDNSLatency)
	}
	if decoded.ValueNS != orig.ValueNS {
		t.Errorf("value_ns: got %d, want %d", decoded.ValueNS, orig.ValueNS)
	}
	if decoded.ConnDstPort != 53 {
		t.Errorf("dst_port: got %d, want 53", decoded.ConnDstPort)
	}
}

func TestSignalFromType(t *testing.T) {
	tests := []struct {
		st   uint32
		sig  string
		unit string
	}{
		{signalTypeDNSLatency, "dns_latency_ms", "ms"},
		{signalTypeTCPRetransmit, "tcp_retransmits_total", "count"},
		{signalTypeRunqueueDelay, "runqueue_delay_ms", "ms"},
		{signalTypeConnectLat, "connect_latency_ms", "ms"},
		{signalTypeTLSHandshake, "tls_handshake_ms", "ms"},
		{signalTypeCPUSteal, "cpu_steal_pct", "ns"},
		{signalTypeMemReclaim, "mem_reclaim_latency_ms", "ms"},
		{signalTypeDiskIOLatency, "disk_io_latency_ms", "ms"},
		{signalTypeSyscallLat, "syscall_latency_ms", "ms"},
	}

	for _, tc := range tests {
		sig, unit := signalFromType(tc.st)
		if sig != tc.sig {
			t.Errorf("signalFromType(%d): got sig=%q, want %q", tc.st, sig, tc.sig)
		}
		if unit != tc.unit {
			t.Errorf("signalFromType(%d): got unit=%q, want %q", tc.st, unit, tc.unit)
		}
	}
}

func TestConvertValue(t *testing.T) {
	// DNS: 5_000_000 ns -> 5.0 ms
	if v := convertValue(signalTypeDNSLatency, 5000000); v != 5.0 {
		t.Errorf("dns convert: got %f, want 5.0", v)
	}

	// TCP retransmit: count pass-through
	if v := convertValue(signalTypeTCPRetransmit, 3); v != 3.0 {
		t.Errorf("tcp convert: got %f, want 3.0", v)
	}

	// CPU steal: raw ns pass-through
	if v := convertValue(signalTypeCPUSteal, 50000); v != 50000.0 {
		t.Errorf("cpu convert: got %f, want 50000.0", v)
	}

	// Mem reclaim: 500_000 ns -> 0.5 ms
	if v := convertValue(signalTypeMemReclaim, 500000); v != 0.5 {
		t.Errorf("mem_reclaim convert: got %f, want 0.5", v)
	}

	// Disk I/O: 2_000_000 ns -> 2.0 ms
	if v := convertValue(signalTypeDiskIOLatency, 2000000); v != 2.0 {
		t.Errorf("disk_io convert: got %f, want 2.0", v)
	}

	// Syscall: 5_000_000 ns -> 5.0 ms
	if v := convertValue(signalTypeSyscallLat, 5000000); v != 5.0 {
		t.Errorf("syscall convert: got %f, want 5.0", v)
	}
}

func TestIPFromU32(t *testing.T) {
	// 127.0.0.1 in network byte order
	ip := ipFromU32(0x0100007F)
	if ip != "127.0.0.1" {
		t.Errorf("ipFromU32: got %q, want 127.0.0.1", ip)
	}
}

func TestToProbeEventConnTuple(t *testing.T) {
	c := &RingBufConsumer{
		meta: EventMetadata{
			Node:      "node-1",
			Namespace: "default",
			Pod:       "rag-service-abc",
			Container: "rag",
		},
	}

	event := bpfEvent{
		PID:         1234,
		TID:         1235,
		SignalType:   signalTypeDNSLatency,
		ValueNS:     10000000, // 10ms
		ConnSrcPort: 42424,
		ConnDstPort: 53,
		ConnDstIP:   0x0100007F,
	}

	probe := c.toProbeEvent(event)
	if probe.Signal != "dns_latency_ms" {
		t.Errorf("signal: got %q", probe.Signal)
	}
	if probe.Value != 10.0 {
		t.Errorf("value: got %f, want 10.0", probe.Value)
	}
	if probe.ConnTuple == nil {
		t.Fatal("conn_tuple should not be nil")
	}
	if probe.ConnTuple.DstPort != 53 {
		t.Errorf("dst_port: got %d, want 53", probe.ConnTuple.DstPort)
	}
	if probe.PID != 1234 {
		t.Errorf("pid: got %d, want 1234", probe.PID)
	}
}
