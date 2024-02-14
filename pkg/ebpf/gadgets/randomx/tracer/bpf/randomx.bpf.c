#include "../../../../include/amd64/vmlinux.h"

#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

#include "randomx.h"
#include "../../../../include/mntns_filter.h"
#include "../../../../include/macros.h"
#include "../../../../include/buffer.h"

// Events map.
struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(u32));
	__uint(value_size, sizeof(u32));
} events SEC(".maps");

// we need this to make sure the compiler doesn't remove our struct.
const struct event *unusedevent __attribute__((unused));

SEC("tracepoint/x86_fpu/x86_fpu_regs_deactivated")
int tracepoint__x86_fpu_regs_deactivated(struct trace_event_raw_x86_fpu *ctx) {
    struct event event = {};

    struct task_struct *current_task = (struct task_struct*)bpf_get_current_task();
    if (!current_task) {
        return 0;
    }

    u64 mntns_id = BPF_CORE_READ(current_task, nsproxy, mnt_ns, ns.inum);

    if (gadget_should_discard_mntns_id(mntns_id)) {
        return 0;
    }

    uint mxcsr = BPF_CORE_READ(ctx, fpu, fpstate, regs.xsave.i387.mxcsr);

    int fpcr = (mxcsr & 0x6000) >> 13;
    if (fpcr != 0) {
        __u32 ppid = BPF_CORE_READ(current_task, real_parent, pid);
        u64 uid_gid = bpf_get_current_uid_gid();
        /* event data */
        event.timestamp = bpf_ktime_get_boot_ns();
        event.mntns_id = mntns_id;
        event.pid = bpf_get_current_pid_tgid() >> 32;
        event.ppid = ppid;
        event.uid = (u32)uid_gid;
        event.gid = (u32)(uid_gid >> 32);
        bpf_get_current_comm(&event.comm, sizeof(event.comm));
        
        /* emit event */
        bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));
    }

    return 0;
}

char _license[] SEC("license") = "GPL";
