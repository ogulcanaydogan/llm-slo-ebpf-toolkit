package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cilium/ebpf/ringbuf"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

// EventMetadata is the workload identity passed to the ring buffer consumer.
// Defined here to avoid importing the signals package (which would create
// an import cycle since signals imports collector).
type EventMetadata struct {
	Node      string
	Namespace string
	Pod       string
	Container string
	TraceID   string
	SpanID    string
}

// signalType mirrors the enum llm_slo_signal_type from llm_slo_event.h.
const (
	signalTypeDNSLatency    uint32 = 1
	signalTypeTCPRetransmit uint32 = 2
	signalTypeRunqueueDelay uint32 = 3
	signalTypeConnectLat    uint32 = 4
	signalTypeTLSHandshake  uint32 = 5
	signalTypeCPUSteal      uint32 = 6
	signalTypeMemReclaim    uint32 = 7
	signalTypeDiskIOLatency uint32 = 8
	signalTypeSyscallLat    uint32 = 9
)

// bpfEvent matches the packed struct llm_slo_event from llm_slo_event.h.
type bpfEvent struct {
	PID          uint32
	TID          uint32
	TimestampNS  uint64
	SignalType   uint32
	ValueNS      uint64
	ConnSrcPort  uint16
	ConnDstPort  uint16
	ConnDstIP    uint32
	ErrnoVal     int32
}

// RingBufConsumer reads llm_slo_event entries from eBPF ring buffers
// and converts them to schema.ProbeEventV1 on a channel.
type RingBufConsumer struct {
	mu      sync.Mutex
	readers []*ringbuf.Reader
	events  chan schema.ProbeEventV1
	done    chan struct{}
	meta    EventMetadata
}

// NewRingBufConsumer creates a consumer. Call AddReader for each probe's
// ring buffer, then Start to begin reading.
func NewRingBufConsumer(bufSize int, meta EventMetadata) *RingBufConsumer {
	if bufSize < 1 {
		bufSize = 256
	}
	return &RingBufConsumer{
		events: make(chan schema.ProbeEventV1, bufSize),
		done:   make(chan struct{}),
		meta:   meta,
	}
}

// AddReader registers a ring buffer reader for consumption.
func (c *RingBufConsumer) AddReader(r *ringbuf.Reader) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readers = append(c.readers, r)
}

// Events returns the channel of decoded probe events.
func (c *RingBufConsumer) Events() <-chan schema.ProbeEventV1 {
	return c.events
}

// Start begins reading from all registered ring buffers. Each reader
// gets its own goroutine. Blocks until ctx is cancelled.
func (c *RingBufConsumer) Start(ctx context.Context) {
	c.mu.Lock()
	readers := make([]*ringbuf.Reader, len(c.readers))
	copy(readers, c.readers)
	c.mu.Unlock()

	var wg sync.WaitGroup
	for _, r := range readers {
		wg.Add(1)
		go func(reader *ringbuf.Reader) {
			defer wg.Done()
			c.readLoop(ctx, reader)
		}(r)
	}

	<-ctx.Done()
	for _, r := range readers {
		r.Close()
	}
	wg.Wait()
	close(c.events)
	close(c.done)
}

// Done returns a channel closed when all readers have stopped.
func (c *RingBufConsumer) Done() <-chan struct{} {
	return c.done
}

func (c *RingBufConsumer) readLoop(ctx context.Context, reader *ringbuf.Reader) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		record, err := reader.Read()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("ringbuf read error: %v", err)
			continue
		}

		event, err := decodeBPFEvent(record.RawSample)
		if err != nil {
			log.Printf("ringbuf decode error: %v", err)
			continue
		}

		probeEvent := c.toProbeEvent(event)
		select {
		case c.events <- probeEvent:
		case <-ctx.Done():
			return
		}
	}
}

func decodeBPFEvent(data []byte) (bpfEvent, error) {
	var event bpfEvent
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.LittleEndian, &event); err != nil {
		return event, fmt.Errorf("decode bpf event: %w", err)
	}
	return event, nil
}

func (c *RingBufConsumer) toProbeEvent(e bpfEvent) schema.ProbeEventV1 {
	sig, unit := signalFromType(e.SignalType)
	value := convertValue(e.SignalType, e.ValueNS)

	event := schema.ProbeEventV1{
		TSUnixNano: time.Now().UnixNano(),
		Signal:     sig,
		Node:       c.meta.Node,
		Namespace:  c.meta.Namespace,
		Pod:        c.meta.Pod,
		Container:  c.meta.Container,
		PID:        int(e.PID),
		TID:        int(e.TID),
		Value:      value,
		Unit:       unit,
		Status:     "ok",
		TraceID:    c.meta.TraceID,
		SpanID:     c.meta.SpanID,
	}

	if e.ConnSrcPort != 0 || e.ConnDstPort != 0 {
		event.ConnTuple = &schema.ConnTuple{
			SrcIP:    "0.0.0.0", // enriched by probe manager with actual IP
			DstIP:    ipFromU32(e.ConnDstIP),
			SrcPort:  int(e.ConnSrcPort),
			DstPort:  int(e.ConnDstPort),
			Protocol: "tcp",
		}
	}

	if e.ErrnoVal != 0 {
		errno := int(e.ErrnoVal)
		event.Errno = &errno
	}

	return event
}

func signalFromType(st uint32) (string, string) {
	switch st {
	case signalTypeDNSLatency:
		return "dns_latency_ms", "ms"
	case signalTypeTCPRetransmit:
		return "tcp_retransmits_total", "count"
	case signalTypeRunqueueDelay:
		return "runqueue_delay_ms", "ms"
	case signalTypeConnectLat:
		return "connect_latency_ms", "ms"
	case signalTypeTLSHandshake:
		return "tls_handshake_ms", "ms"
	case signalTypeCPUSteal:
		return "cpu_steal_pct", "ns"
	case signalTypeMemReclaim:
		return "mem_reclaim_latency_ms", "ms"
	case signalTypeDiskIOLatency:
		return "disk_io_latency_ms", "ms"
	case signalTypeSyscallLat:
		return "syscall_latency_ms", "ms"
	default:
		return "unknown", "unknown"
	}
}

// convertValue converts raw nanosecond/count values from the kernel to
// the unit expected by the signal schema.
func convertValue(signalType uint32, valueNS uint64) float64 {
	switch signalType {
	case signalTypeTCPRetransmit:
		return float64(valueNS) // already a count
	case signalTypeCPUSteal:
		return float64(valueNS) // raw ns; Go-side aggregates to pct
	default:
		return float64(valueNS) / 1e6 // ns -> ms
	}
}

func ipFromU32(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		ip&0xFF, (ip>>8)&0xFF, (ip>>16)&0xFF, (ip>>24)&0xFF)
}
