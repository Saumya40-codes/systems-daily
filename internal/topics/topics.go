package topics

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// Topic is a systems-ish deep-dive subject.
type Topic struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Angles   []string `json:"angles"` // optional focus hints for the LLM
}

// Catalog is a hand-curated set of low-level / systems topics.
// Enough for ~months of daily variety without repetition.
var Catalog = []Topic{
	// Memory & allocators
	{ID: "buddy-allocator", Title: "Buddy memory allocator internals", Category: "memory",
		Angles: []string{"block splitting/coalescing", "orders and freelists", "fragmentation tradeoffs"}},
	{ID: "slab-allocator", Title: "Slab / object caching allocators (SLAB, SLUB)", Category: "memory",
		Angles: []string{"caches vs pages", "per-CPU freelists", "why kernels love slabs"}},
	{ID: "jemalloc", Title: "How jemalloc organizes arenas and size classes", Category: "memory",
		Angles: []string{"arenas and threads", "tcache", "extent management"}},
	{ID: "tcmalloc", Title: "tcmalloc: thread caches and page heaps", Category: "memory",
		Angles: []string{"central freelist", "span", "vs glibc malloc"}},
	{ID: "mmap-internals", Title: "mmap, demand paging, and anonymous mappings", Category: "memory",
		Angles: []string{"page faults", "VMAs", "MAP_PRIVATE vs SHARED"}},
	{ID: "tlb-shootdown", Title: "TLB shootdowns and multi-core address spaces", Category: "memory",
		Angles: []string{"IPI cost", "lazy TLB", "when munmap hurts"}},
	{ID: "virtual-memory", Title: "Virtual memory: page tables, walks, and huge pages", Category: "memory",
		Angles: []string{"4-level page tables", "THP", "TLB coverage"}},
	{ID: "numa-memory", Title: "NUMA memory placement and first-touch policy", Category: "memory",
		Angles: []string{"local vs remote latency", "numactl", "auto-balancing pitfalls"}},
	{ID: "gc-vs-manual", Title: "Why systems code still prefers manual memory control", Category: "memory",
		Angles: []string{"latency tails", "ownership models", "arenas in games/kernels"}},

	// OS / concurrency
	{ID: "context-switch", Title: "What actually happens on a context switch", Category: "os",
		Angles: []string{"registers & kernel stack", "scheduler entry", "cost breakdown"}},
	{ID: "futex", Title: "Futexes: userspace locking with kernel help", Category: "os",
		Angles: []string{"FUTEX_WAIT/WAKE", "mutex fast path", "thundering herd"}},
	{ID: "epoll", Title: "epoll readiness model vs select/poll", Category: "os",
		Angles: []string{"edge vs level trigger", "epoll_ctl cost", "when it still scales poorly"}},
	{ID: "io-uring", Title: "io_uring submission and completion rings", Category: "os",
		Angles: []string{"SQ/CQ rings", "fixed buffers", "why it beats epoll+read"}},
	{ID: "cgroups-v2", Title: "cgroups v2: resource control for memory and CPU", Category: "os",
		Angles: []string{"controllers", "pressure stalls (PSI)", "container reality"}},
	{ID: "namespaces", Title: "Linux namespaces and the illusion of isolation", Category: "os",
		Angles: []string{"pid/mnt/net", "user namespaces", "what is NOT isolated"}},
	{ID: "scheduler-cfs", Title: "CFS: fair scheduling without timeslices as you know them", Category: "os",
		Angles: []string{"vruntime", "red-black tree", "nice weights"}},
	{ID: "rcu-basics", Title: "RCU: wait-free reads in the kernel", Category: "os",
		Angles: []string{"grace periods", "why it works", "userspace RCU existence"}},
	{ID: "interrupt-handling", Title: "Hard IRQs, softirqs, and bottom halves", Category: "os",
		Angles: []string{"top half constraints", "NAPI", "threaded IRQs"}},
	{ID: "page-cache", Title: "The page cache: how files become memory", Category: "os",
		Angles: []string{"writeback", "dirty pages", "O_DIRECT escape hatch"}},

	// CPU / architecture
	{ID: "cache-coherence", Title: "MESI cache coherence and false sharing", Category: "cpu",
		Angles: []string{"state machine", "cache lines", "padding atomics"}},
	{ID: "memory-ordering", Title: "Memory ordering: acquire, release, and the mess in between", Category: "cpu",
		Angles: []string{"x86 TSO vs ARM", "compiler barriers", "a real bug pattern"}},
	{ID: "branch-prediction", Title: "Branch predictors and why mispredicts hurt", Category: "cpu",
		Angles: []string{"pipeline flush", "indirect branches", "spectre-shaped intuition"}},
	{ID: "simd-basics", Title: "SIMD: packing work into vector registers", Category: "cpu",
		Angles: []string{"SSE/AVX intuition", "when autovectorization fails", "alignment"}},
	{ID: "syscalls", Title: "The syscall path from userspace to kernel", Category: "cpu",
		Angles: []string{"syscall instruction", "pt_regs", "vdso fast paths"}},
	{ID: "atomic-ops", Title: "Atomic operations and lock-free data structure caveats", Category: "cpu",
		Angles: []string{"CAS loops", "ABA", "when locks win"}},

	// Networking stack
	{ID: "tcp-congestion", Title: "TCP congestion control: CUBIC and friends", Category: "net",
		Angles: []string{"cwnd dynamics", "loss vs delay signals", "BBR intuition"}},
	{ID: "sk-buff", Title: "sk_buff: how the Linux network stack carries packets", Category: "net",
		Angles: []string{"linear vs paged data", "checksum offload", "zero-copy paths"}},
	{ID: "nics-rings", Title: "NIC ring buffers, DMA, and interrupt moderation", Category: "net",
		Angles: []string{"RX/TX rings", "coalescing", "busy poll"}},
	{ID: "ebpf-xdp", Title: "XDP and eBPF at the earliest RX point", Category: "net",
		Angles: []string{"driver mode", "maps", "drop vs redirect"},
	},
	{ID: "udp-reliability", Title: "Building reliability on UDP (QUIC-shaped thinking)", Category: "net",
		Angles: []string{"loss recovery", "connection IDs", "head-of-line blocking"}},

	// Embedded / hardware
	{ID: "bare-metal-boot", Title: "Bare-metal boot: from reset vector to main", Category: "embedded",
		Angles: []string{"vector table", "BSS/.data init", "linker scripts"}},
	{ID: "rtos-vs-bare", Title: "RTOS scheduling vs bare-metal superloops", Category: "embedded",
		Angles: []string{"priorities & inversion", "stack per task", "when superloop wins"}},
	{ID: "dma-embedded", Title: "DMA on microcontrollers: freer CPU, harder bugs", Category: "embedded",
		Angles: []string{"descriptors", "cache coherency on MCUs", "half-complete IRQs"}},
	{ID: "watchdogs", Title: "Hardware and software watchdogs done right", Category: "embedded",
		Angles: []string{"windowed watchdogs", "deadlock vs hang", "safe reset policy"}},
	{ID: "flash-wear", Title: "Flash wear leveling and embedded filesystems", Category: "embedded",
		Angles: []string{"erase blocks", "LittleFS/SPIFFS ideas", "power-loss safety"}},
	{ID: "i2c-spi", Title: "I2C vs SPI: buses, clocks, and failure modes", Category: "embedded",
		Angles: []string{"clock stretching", "CS lines", "debugging stuck buses"}},
	{ID: "cortex-m-faults", Title: "ARM Cortex-M fault handlers you can actually use", Category: "embedded",
		Angles: []string{"HardFault stack frame", "CFSR bits", "post-mortem dumps"}},
	{ID: "power-modes", Title: "MCU low-power modes and wake sources", Category: "embedded",
		Angles: []string{"stop/standby", "RTC wake", "peripheral clock gates"}},

	// GNSS / GPS / radio navigation
	{ID: "gps-signals", Title: "GPS L1 C/A: how a weak signal finds you", Category: "gnss",
		Angles: []string{"PRN codes", "correlation", "why indoors fails"}},
	{ID: "gnss-trilateration", Title: "From pseudoranges to a position fix", Category: "gnss",
		Angles: []string{"4 satellites & clock bias", "GDOP", "least squares intuition"}},
	{ID: "gnss-errors", Title: "GNSS error sources: ionosphere, multipath, clocks", Category: "gnss",
		Angles: []string{"error budgets", "dual-frequency", "urban canyons"}},
	{ID: "rtk-basics", Title: "RTK: centimeter GNSS with carrier phase", Category: "gnss",
		Angles: []string{"base vs rover", "integer ambiguity", "fix length limits"}},
	{ID: "gnss-assisted", Title: "A-GNSS / assisted GPS and TTFF", Category: "gnss",
		Angles: []string{"ephemeris download cost", "hot/warm/cold start", "SUPL-shaped idea"}},
	{ID: "sbas", Title: "SBAS (WAAS/EGNOS): corrections from the sky", Category: "gnss",
		Angles: []string{"integrity", "geo satellites", "aviation heritage"}},
	{ID: "nmea-ubx", Title: "NMEA vs binary GNSS protocols (u-blox UBX)", Category: "gnss",
		Angles: []string{"sentence parsing", "binary efficiency", "configuration gotchas"}},
	{ID: "ins-fusion", Title: "GNSS/INS fusion: when satellites disappear", Category: "gnss",
		Angles: []string{"IMU drift", "Kalman intuition", "urban navigation"}},

	// Filesystems / storage
	{ID: "ext4-journal", Title: "ext4 journaling modes and crash consistency", Category: "storage",
		Angles: []string{"data=ordered", "journal replay", "fsync semantics"}},
	{ID: "ssd-ftl", Title: "SSD FTLs: mapping, GC, and write amplification", Category: "storage",
		Angles: []string{"LBA to physical", "over-provisioning", "TRIM"}},
	{ID: "b-trees-fs", Title: "B-trees in filesystems (and why not always LSM)", Category: "storage",
		Angles: []string{"extent trees", "range queries", "update costs"}},
	{ID: "raid-basics", Title: "RAID levels as tradeoffs, not magic", Category: "storage",
		Angles: []string{"stripe vs mirror", "write hole", "rebuild risk"}},

	// Compilers / linking / ABI
	{ID: "elf-loading", Title: "ELF loading: program headers to process image", Category: "toolchain",
		Angles: []string{"PT_LOAD", "dynamic linker", "relocations"}},
	{ID: "calling-convention", Title: "System V AMD64 calling convention in practice", Category: "toolchain",
		Angles: []string{"arg registers", "red zone", "stack frames"}},
	{ID: "linkers", Title: "What linkers actually resolve (and LTO changes)", Category: "toolchain",
		Angles: []string{"symbols & sections", "COMDATs", "why link times hurt"}},
	{ID: "undefined-behavior", Title: "Undefined behavior that systems programmers still hit", Category: "toolchain",
		Angles: []string{"strict aliasing", "shift UB", "optimizers exploiting UB"}},

	// Security at systems layer
	{ID: "aslr-dep", Title: "ASLR, NX/DEP, and why exploits got harder (not impossible)", Category: "security",
		Angles: []string{"entropy sources", "info leaks", "ROP intuition"}},
	{ID: "seccomp", Title: "seccomp-bpf: shrinking the kernel attack surface", Category: "security",
		Angles: []string{"filter language", "strict vs filter", "container use"}},
	{ID: "capabilities", Title: "Linux capabilities vs root-all-or-nothing", Category: "security",
		Angles: []string{"bounding set", "file caps", "common footguns"}},

	// Misc systems gold
	{ID: "timekeeping", Title: "Kernel timekeeping: clocks, timers, and CLOCK_MONOTONIC", Category: "os",
		Angles: []string{"TSC", "clocksource", "sleep precision"}},
	{ID: "signals", Title: "UNIX signals: async, messy, still essential", Category: "os",
		Angles: []string{"async-signal-safety", "signalfd", "why handlers should be tiny"}},
	{ID: "shared-memory", Title: "POSIX shared memory and memory-mapped IPC", Category: "os",
		Angles: []string{"shm_open", "sync with atomics", "vs sockets"}},
	{ID: "backpressure", Title: "Backpressure in systems: queues, drops, and load shedding", Category: "systems",
		Angles: []string{"bounded queues", "latency vs throughput", "overload control"}},
	{ID: "observability-ebpf", Title: "Tracing production systems with eBPF", Category: "systems",
		Angles: []string{"kprobes/uprobes", "overhead", "what not to trace"}},
	{ID: "lock-contention", Title: "Diagnosing lock contention like a systems engineer", Category: "systems",
		Angles: []string{"perf", "critical section size", "sharding locks"}},
}

// Pick selects a topic not in recentIDs. recentIDs should be topic IDs used recently.
func Pick(recentIDs []string, rng *rand.Rand) (Topic, error) {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	blocked := make(map[string]struct{}, len(recentIDs))
	for _, id := range recentIDs {
		blocked[id] = struct{}{}
	}

	var candidates []Topic
	for _, t := range Catalog {
		if _, ok := blocked[t.ID]; !ok {
			candidates = append(candidates, t)
		}
	}
	if len(candidates) == 0 {
		// All topics exhausted in window - fall back to full catalog
		candidates = append([]Topic(nil), Catalog...)
	}
	return candidates[rng.Intn(len(candidates))], nil
}

// ByID looks up a topic; returns error if unknown.
func ByID(id string) (Topic, error) {
	id = strings.TrimSpace(id)
	for _, t := range Catalog {
		if t.ID == id {
			return t, nil
		}
	}
	return Topic{}, fmt.Errorf("unknown topic id %q", id)
}

// Categories returns unique category names.
func Categories() []string {
	seen := map[string]struct{}{}
	var out []string
	for _, t := range Catalog {
		if _, ok := seen[t.Category]; !ok {
			seen[t.Category] = struct{}{}
			out = append(out, t.Category)
		}
	}
	return out
}
